package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type probeRoundTrip func(*http.Request) (*http.Response, error)

func (f probeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProxyExitResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"success", 200, `{"success":true,"ip":"2001:db8::1","country":"Example","region":"Region","city":"City","latitude":1,"error":"forged"}`, ""},
		{"rate limited", 429, `private service error`, "rate_limited"},
		{"redirect", 302, ``, "service"},
		{"upstream failure", 503, `private service error`, "service"},
		{"false success", 200, `{"success":false,"ip":"192.0.2.1"}`, "service"},
		{"invalid IP", 200, `{"success":true,"ip":"not-an-ip"}`, "service"},
		{"invalid JSON", 200, `malformed`, "service"},
		{"oversized", 200, strings.Repeat(" ", 16*1024+1), "service"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: probeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.URL.String() != proxyTestURL || r.Header.Get("Authorization") != "" || r.Body != nil {
					t.Fatal("unexpected test destination or payload")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})}
			result := queryProxyExit(t.Context(), client)
			if result.Error != test.want {
				t.Fatalf("got %+v", result)
			}
			if test.want == "" && (result.IP != "2001:db8::1" || result.City != "City") {
				t.Fatal(result)
			}
		})
	}
	client := &http.Client{Transport: probeRoundTrip(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("proxy http://user:private-password@example.test")
	})}
	if result := queryProxyExit(t.Context(), client); result.Error != "connection" || result.IP != "" {
		t.Fatal(result)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result := queryProxyExit(ctx, client); result.Error != "timeout" {
		t.Fatal(result)
	}
}

func TestProxyProbeUsesSelectedProxyWithoutDirectFallback(t *testing.T) {
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "CONNECT" || r.Host != "ipwho.is:443" || r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpzeW50aGV0aWM=" {
			t.Error("wrong proxy tunnel or authentication")
		}
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer proxy.Close()
	address := strings.Replace(proxy.URL, "http://", "http://user:synthetic@", 1)
	started := time.Now()
	result := TestProxy(t.Context(), Proxy{Type: ProxyHTTP, URL: address})
	if result.Error != "connection" || calls.Load() != 1 || time.Since(started) > time.Second*5 {
		t.Fatalf("result=%+v calls=%d", result, calls.Load())
	}
}

func TestProxyProbeHonorsEarlierDeadline(t *testing.T) {
	hold := make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-hold }))
	defer proxy.Close()
	defer close(hold)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if result := TestProxy(ctx, Proxy{Type: ProxyHTTP, URL: proxy.URL}); result.Error != "timeout" {
		t.Fatal(result)
	}
}
