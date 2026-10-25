package api

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"net/url"
	"testing"
)

func TestNavigationHTTPOnBothInstances(t *testing.T) {
	s, l := testServer(t)
	date, _ := domain.ParseLocalDate("2026-10")
	box, err := l.CreateBox(t.Context(), "旅行 #?%", "", date, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	client := clientServerFromNAS(t, l)
	for _, server := range []*Server{s, client} {
		location := decodeBody[library.NavigationLocation](t, server, "/navigation?box="+box.ID.String())
		resolved := decodeBody[library.NavigationLocation](t, server, "/navigation?path="+url.QueryEscape(location.Path))
		if resolved != location {
			t.Fatalf("path decoding: %+v %+v", location, resolved)
		}
		for _, test := range []struct {
			query  string
			status int
		}{
			{"path=" + url.QueryEscape("/boxes/%2e%2e"), 400},
			{"path=" + url.QueryEscape("/boxes/2026-10_missing"), 404},
			{"path=" + url.QueryEscape(location.Path) + "&box=" + box.ID.String(), 400},
			{"file=bad", 400},
			{"box=" + box.ID.String() + "&view=bad", 422},
		} {
			response := call(server, "GET", Prefix+"/navigation?"+test.query, "", "")
			if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%s: %d %s", test.query, response.Code, response.Body)
			}
		}
	}
}
