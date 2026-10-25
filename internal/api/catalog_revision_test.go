package api

import (
	"github.com/jinzhao1994/kist/internal/domain"
	"testing"
)

func TestCatalogRevisionBothInstances(t *testing.T) {
	server, nas := testServer(t)
	client := clientServerFromNAS(t, nas)
	for _, s := range []*Server{server, client} {
		before := decodeBody[catalogRevision](t, s, "/catalog-revision")
		if before.Revision == "" {
			t.Fatal("missing revision")
		}
		if next := decodeBody[catalogRevision](t, s, "/catalog-revision"); next != before {
			t.Fatal("read changed revision")
		}
		var err error
		if s.client != nil {
			_, err = s.client.CreateBox(t.Context(), "external edit", "", domain.LocalDate{}, domain.LocalDate{})
		} else {
			_, err = nas.CreateBox(t.Context(), "external edit", "", domain.LocalDate{}, domain.LocalDate{})
		}
		if err != nil {
			t.Fatal(err)
		}
		after := decodeBody[catalogRevision](t, s, "/catalog-revision")
		if after == before {
			t.Fatal("external edit did not change revision")
		}
		w := call(s, "GET", Prefix+"/catalog-revision", "", "")
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("revision must not be cached by HTTP")
		}
	}
}
