package library

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/ai"
	"github.com/jinzhao1994/kist/internal/ai/openai"
)

func TestAIAccountProbeCatalogAndSavedRoute(t *testing.T) {
	for _, mode := range []string{"api direct", "api proxy", "chatgpt"} {
		t.Run(mode, func(t *testing.T) {
			var status atomic.Int32
			status.Store(200)
			var calls, tunnels atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "GET" || r.URL.Path != "/v1/models" || r.ContentLength > 0 || r.Header.Get("Authorization") != "Bearer synthetic-key" {
					t.Error("wrong account diagnostic request")
				}
				w.WriteHeader(int(status.Load()))
				if status.Load() != 200 {
					io.WriteString(w, `{"error":{"message":"synthetic-key must never be returned"}}`)
					return
				}
				if mode == "chatgpt" {
					io.WriteString(w, `{"models":[{"slug":"synthetic","visibility":"list"}]}`)
				} else {
					io.WriteString(w, `{"data":[{"id":"synthetic"}]}`)
				}
			}))
			defer server.Close()
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tunnels.Add(1)
				if r.Method != "CONNECT" || r.Host != "api.openai.com:443" {
					t.Error("wrong selected proxy destination")
				}
				remote, err := net.Dial("tcp", server.Listener.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}
				defer remote.Close()
				conn, rw, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
				rw.Flush()
				go func() { io.Copy(remote, rw); remote.Close() }()
				io.Copy(conn, remote)
			}))
			defer proxy.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			previous := http.DefaultTransport
			transport := previous.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{RootCAs: roots, ServerName: server.Certificate().DNSNames[0]}
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				target := server.Listener.Addr().String()
				if address == proxy.Listener.Addr().String() {
					target = address
				} else if address != "api.openai.com:443" {
					return nil, errors.New("unexpected outbound destination")
				}
				return (&net.Dialer{}).DialContext(ctx, network, target)
			}
			http.DefaultTransport = transport
			defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()
			l := openTestLibrary(t)
			draft, _ := l.AIServicesSettings(t.Context())
			draft.Accounts = []AIAccountSettings{{Number: -1, ID: "Synthetic", Type: ai.AccountOpenAI, APIKey: "synthetic-key"}}
			if mode == "api proxy" {
				draft.Proxies = []AIProxySettings{{Number: -1, ID: "Synthetic proxy", Type: ai.ProxyHTTP, URL: proxy.URL}}
				draft.Accounts[0].Proxy = -1
			}
			if mode == "chatgpt" {
				draft.Accounts[0].Type = ai.AccountChatGPT
				draft.Accounts[0].APIKey = ""
			}
			saveServiceDraft(t, l, draft)
			if mode == "chatgpt" {
				if err := l.SaveAIAccountConnection(t.Context(), 1, ai.Credentials{}, ai.Credentials{Provider: "openai", AccessToken: "synthetic-key", ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(l.Layout().ConfigPath())
			result, err := l.TestAIAccount(t.Context(), 1)
			if err != nil || result.Error != "" || len(result.Models) != 1 || result.Models[0] != "synthetic" {
				t.Fatalf("%+v %v", result, err)
			}
			for code, reason := range map[int]string{401: "authentication", 403: "permission", 429: "rate_limited", 503: "service"} {
				status.Store(int32(code))
				result, err = l.TestAIAccount(t.Context(), 1)
				raw, _ := json.Marshal(result)
				if err != nil || result.Error != reason || strings.Contains(string(raw), "synthetic-key") {
					t.Fatalf("%+v %v", result, err)
				}
			}
			after, _ := os.ReadFile(l.Layout().ConfigPath())
			if !bytes.Equal(before, after) || calls.Load() != 5 || (mode == "api proxy") != (tunnels.Load() > 0) {
				t.Fatal("route, request count or configuration changed")
			}
		})
	}
}

func TestAIAccountProbeFailureBoundaries(t *testing.T) {
	if result := accountTestFailure(t.Context(), &openai.TokenError{Reauthenticate: true}); result.Error != "authentication" {
		t.Fatal(result)
	}
	l := openTestLibrary(t)
	draft, _ := l.AIServicesSettings(t.Context())
	draft.Accounts = []AIAccountSettings{{Number: -1, ID: "Synthetic", Type: ai.AccountChatGPT}}
	saveServiceDraft(t, l, draft)
	if result, err := l.TestAIAccount(t.Context(), 1); err != nil || result.Error != "not_configured" {
		t.Fatal(result, err)
	}
	if _, err := l.TestAIAccount(t.Context(), 999); !errors.Is(err, ErrNoSuchAIAccount) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result := accountTestFailure(ctx, openai.ErrConnection); result.Error != "timeout" {
		t.Fatal(result)
	}
	if result := accountTestFailure(t.Context(), openai.ErrConnection); result.Error != "connection" {
		t.Fatal(result)
	}
}
