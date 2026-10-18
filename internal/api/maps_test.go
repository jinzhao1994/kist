package api

import (
	"strings"
	"testing"
)

func TestMapProviderSources(t *testing.T) {
	p := DefaultMapProvider()
	got, err := p.connectSources()
	if err != nil || got != "'self' https://tiles.openfreemap.org https://elevation-tiles-prod.s3.amazonaws.com" {
		t.Fatalf("%q %v", got, err)
	}
	p = MapProvider{StyleURL: "/maps/style.json", ResourceOrigins: []string{"https://maps.example", "https://maps.example"}}
	got, err = p.connectSources()
	if err != nil || got != "'self' https://maps.example" {
		t.Fatalf("%q %v", got, err)
	}
	for _, raw := range []string{"javascript:alert(1)", "https://maps.example; script-src *", "https://*.example/a", "//maps.example/a", "https://user:password@maps.example/a"} {
		if _, err = (MapProvider{StyleURL: raw}).connectSources(); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestMapProviderEndpointAndCSP(t *testing.T) {
	s, _ := testServer(t)
	response := call(s, "GET", Prefix+"/map-provider", "", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "tiles.openfreemap.org/styles/liberty") {
		t.Fatal(response.Code, response.Body)
	}
	csp := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'self' https://tiles.openfreemap.org") || !strings.Contains(csp, "worker-src 'self'") || strings.Contains(csp, "'unsafe-eval'") {
		t.Fatal(csp)
	}
}
