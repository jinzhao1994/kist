package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeAccessConfiguration(t *testing.T) {
	for _, tc := range []struct {
		address string
		origins []string
		ok      bool
	}{
		{"127.0.0.1:8473", nil, true}, {"[::1]:8473", nil, true},
		{"0.0.0.0:8473", nil, false}, {":8473", nil, false},
		{"0.0.0.0:8473", []string{"https://archive.example"}, true},
		{"127.0.0.1:0", nil, false},
	} {
		opt, err := serveOptions(tc.address, tc.origins)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.address, err)
		}
		if tc.ok && len(opt.AllowedHosts) == 0 {
			t.Fatal("empty Host allowlist")
		}
	}
}
func TestServeStopsOnCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serveHTTP(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	}()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}
