package library

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type refreshTransport struct{ endpoint string }

func (r refreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := req.Clone(req.Context())
	u := *req.URL
	next.URL = &u
	target, _ := http.NewRequest(http.MethodPost, r.endpoint, nil)
	next.URL.Scheme, next.URL.Host = target.URL.Scheme, target.URL.Host
	return http.DefaultTransport.RoundTrip(next)
}

func refreshCredentials(t *testing.T, l *Library, layered bool) ai.Credentials {
	t.Helper()
	c := ai.Credentials{HostID: "synthetic-host", Provider: "openai", ClientID: "synthetic-client", Subject: "synthetic-subject", AccessToken: "expired-access", RefreshToken: "rotating-refresh", Scope: "resource.invoke chatgpt.tokens.use.direct"}
	if layered {
		draft, _ := l.AIServicesSettings(t.Context())
		draft.Accounts = []AIAccountSettings{{Number: -1, ID: "A", Type: ai.AccountChatGPT}, {Number: -2, ID: "B", Type: ai.AccountChatGPT}}
		saveServiceDraft(t, l, draft)
		if err := l.SaveAIAccountConnection(t.Context(), 1, ai.Credentials{}, c); err != nil {
			t.Fatal(err)
		}
	} else if err := l.SaveAIConnection(t.Context(), ai.Credentials{}, c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAIAccountRefreshSerializedWithoutConfigLock(t *testing.T) {
	for _, layered := range []bool{false, true} {
		name := "legacy"
		if layered {
			name = "layered"
		}
		t.Run(name, func(t *testing.T) {
			l := openTestLibrary(t)
			selected := refreshCredentials(t, l, layered)
			entered, proceed := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) != 1 {
					t.Error("redeemed a rotating token more than once")
					http.Error(w, "already used", http.StatusBadRequest)
					return
				}
				if err := r.ParseForm(); err != nil || r.Form.Get("refresh_token") != "rotating-refresh" {
					t.Error("unexpected refresh input")
				}
				close(entered)
				select {
				case <-proceed:
				case <-r.Context().Done():
					return
				}
				io.WriteString(w, `{"access_token":"fresh-access","refresh_token":"fresh-refresh","expires_in":3600,"token_type":"Bearer"}`)
			}))
			defer server.Close()
			client := &http.Client{Transport: refreshTransport{server.URL}, Timeout: 5 * time.Second}
			type result struct {
				credentials ai.Credentials
				err         error
			}
			results := make(chan result, 2)
			call := func() {
				c, err := l.refreshAIAccount(t.Context(), 1, selected, client)
				results <- result{c, err}
			}
			go call()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("refresh did not start")
			}
			go call()
			// The slow endpoint must not block unrelated configuration writes.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			if layered {
				_, err := l.PrepareAIAccountConnection(ctx, 2)
				if err != nil {
					t.Error("refresh held config lock", err)
				}
			} else {
				if err := l.updateAI(ctx, func(c *ai.Credentials) error { c.Email = "example@example.test"; return nil }); err != nil {
					t.Error("refresh held config lock", err)
				}
			}
			cancel()
			// Metadata-only credential changes are intentionally still CAS
			// protected. Restore the legacy identity before publishing refresh.
			if !layered {
				if err := l.updateAI(t.Context(), func(c *ai.Credentials) error { c.Email = ""; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			close(proceed)
			for range 2 {
				result := <-results
				if result.err != nil || result.credentials.AccessToken != "fresh-access" || result.credentials.RefreshToken != "fresh-refresh" {
					t.Fatal("refresh did not share persisted credentials", result.err)
				}
			}
			if calls.Load() != 1 {
				t.Fatal("unexpected refresh count", calls.Load())
			}
		})
	}
}

func TestAIAccountRefreshCancellationAndConcurrentDisconnect(t *testing.T) {
	l := openTestLibrary(t)
	selected := refreshCredentials(t, l, true)
	release, err := lockfile.New(filepath.Join(l.layout.Root, "ai-account-1-refresh.lock")).Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = l.refreshAIAccount(ctx, 1, selected, &http.Client{})
	release()
	if !errors.Is(err, context.Canceled) {
		t.Fatal("refresh admission ignored cancellation", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a local disconnect while the remote refresh is in flight.
		disconnected := selected
		disconnected.AccessToken, disconnected.RefreshToken = "", ""
		if err := l.SaveAIAccountConnection(t.Context(), 1, selected, disconnected); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"access_token":"must-not-be-saved","refresh_token":"must-not-be-saved","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer server.Close()
	_, err = l.refreshAIAccount(t.Context(), 1, selected, &http.Client{Transport: refreshTransport{server.URL}})
	if !errors.Is(err, config.ErrChanged) {
		t.Fatal("refresh resurrected disconnected account", err)
	}
	cfg, _ := config.Load(l.layout.ConfigPath())
	a, _ := cfg.AIServices.Account(1)
	if a.Credentials.AccessToken != "" {
		t.Fatal("disconnect was overwritten")
	}
	_, err = l.refreshAIAccount(t.Context(), 1, selected, &http.Client{})
	if !errors.Is(err, config.ErrChanged) {
		t.Fatal("accepted disconnected snapshot", err)
	}
	reconnected := selected
	reconnected.Subject = "another-synthetic-subject"
	reconnected.ExpiresAt = time.Now().Add(time.Hour).Unix()
	if err := l.SaveAIAccountConnection(t.Context(), 1, a.Credentials, reconnected); err != nil {
		t.Fatal(err)
	}
	_, err = l.refreshAIAccount(t.Context(), 1, selected, &http.Client{})
	if !errors.Is(err, config.ErrChanged) {
		t.Fatal("running task switched to a different signed-in identity", err)
	}
}

func TestAIAccountLegacyProxyPolicy(t *testing.T) {
	l := openTestLibrary(t)
	refreshCredentials(t, l, false)
	cfg, _ := config.Load(l.layout.ConfigPath())
	client, err := configuredAccountHTTP(cfg, 1, time.Second)
	if err != nil || client.Transport.(*http.Transport).Proxy == nil {
		t.Fatal("legacy empty proxy stopped inheriting environment", err)
	}
	draft, _ := l.AIServicesSettings(t.Context())
	saveServiceDraft(t, l, draft)
	cfg, _ = config.Load(l.layout.ConfigPath())
	client, err = configuredAccountHTTP(cfg, 1, time.Second)
	if err != nil || client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("explicit direct route inherited environment", err)
	}
	if _, err = configuredAccountHTTP(cfg, 999, time.Second); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatal("accepted removed account", err)
	}
}
