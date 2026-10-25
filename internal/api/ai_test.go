package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIOnlyAcceptsLoopbackRequests(t *testing.T) {
	s, _ := testServer(t)
	for _, tc := range []struct {
		addr, host string
		code       int
	}{{"192.0.2.1:1234", "localhost:8473", 403}, {"127.0.0.1:1234", "localhost:8473", 200}, {"127.0.0.1:1234", "archive.example:8473", 403}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+Prefix+"/ai", nil)
		req.RemoteAddr = tc.addr
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "access_token") {
			t.Fatal("secret field exposed")
		}
	}
}
