package library

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestAIAccountMaintenanceRefreshesIdleAndBacksOff(t *testing.T) {
	for _, mode := range []string{"success", "temporary", "terminal"} {
		t.Run(mode, func(t *testing.T) {
			l := openTestLibrary(t)
			original := refreshCredentials(t, l, true)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch mode {
				case "temporary":
					w.WriteHeader(503)
					io.WriteString(w, `{"error":"private-upstream-body"}`)
				case "terminal":
					w.WriteHeader(400)
					io.WriteString(w, `{"error":"invalid_grant"}`)
				default:
					io.WriteString(w, `{"access_token":"fresh-access","refresh_token":"fresh-refresh","expires_in":3600,"token_type":"Bearer"}`)
				}
			}))
			defer server.Close()
			clientFor := func(*config.Config, int64, time.Duration) (*http.Client, error) {
				return &http.Client{Transport: refreshTransport{server.URL}}, nil
			}
			state := l.renewAIAccounts(t.Context(), nil, clientFor)
			if calls != 1 {
				t.Fatal("missing idle renewal", calls)
			}
			if mode == "success" {
				if len(state) != 0 {
					t.Fatal("successful renewal left failure")
				}
				cfg, _ := config.Load(l.layout.ConfigPath())
				a, _ := cfg.AIServices.Account(1)
				if a.Credentials.RefreshToken != "fresh-refresh" {
					t.Fatal("rotation not persisted")
				}
			} else {
				want := "temporary"
				if mode == "terminal" {
					want = "reauthenticate"
				}
				if state[1].Error != want || state[1].Version != credentialVersion(original) {
					t.Fatal("wrong recovery classification")
				}
			}
			state = l.renewAIAccounts(t.Context(), state, clientFor)
			if calls != 1 {
				t.Fatal("retried before backoff or refreshed a valid token")
			}
			if mode == "success" {
				return
			}
			retry := state[1]
			retry.RetryAt = time.Now().Add(-time.Second)
			state[1] = retry
			state = l.renewAIAccounts(t.Context(), state, clientFor)
			if mode == "terminal" && calls != 1 {
				t.Fatal("terminal refresh looped")
			}
			if mode == "temporary" && (calls != 2 || state[1].Delay != 2*time.Minute) {
				t.Fatal("bounded retry did not advance")
			}
			// New saved credentials invalidate the old stop/backoff decision.
			updated := original
			updated.RefreshToken = "replacement-after-login"
			if err := l.SaveAIAccountConnection(t.Context(), 1, original, updated); err != nil {
				t.Fatal(err)
			}
			before := calls
			l.renewAIAccounts(context.Background(), state, clientFor)
			if calls != before+1 {
				t.Fatal("new login retained obsolete failure")
			}
		})
	}
}
