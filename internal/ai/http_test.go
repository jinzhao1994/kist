package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProxyRoutesAIRequests(t *testing.T) {
	called := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Host != "provider.invalid" {
			t.Error(r.URL.Host)
		}
		w.Write([]byte("proxied"))
	}))
	defer proxy.Close()
	client, err := HTTPClient(proxy.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Get("http://provider.invalid/models")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !called || string(body) != "proxied" {
		t.Fatal(string(body))
	}
}
func TestProxyValidation(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1:7890", "https://proxy.example", "http://localhost", "http://[::1]:7890", "socks5h://name:secret@proxy.example:1080", "socks5://127.0.0.1:7891", ""} {
		if _, err := HTTPClient(address, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	for _, address := range []string{"file:///tmp/socket", "localhost:7890", "https://proxy.invalid/path", "http://proxy.example:0", "http://proxy.example:65536", "http://proxy.example:", "http://proxy.example:abc", "http://-invalid.example", "http://proxy..example", "http://[invalid]", "http://proxy.example?query=1", "http://proxy.example#fragment"} {
		if _, err := HTTPClient(address, time.Second); err == nil {
			t.Fatal("accepted invalid proxy")
		}
	}
}

func TestHTTPProxyTunnelsHTTPS(t *testing.T) {
	var method, host string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, host = r.Method, r.Host
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	client, err := HTTPClient(proxy.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Get("https://provider.invalid/v1/models")
	if err == nil || method != "CONNECT" || host != "provider.invalid:443" {
		t.Fatalf("method=%s host=%s err=%v", method, host, err)
	}
}
