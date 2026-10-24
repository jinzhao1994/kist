package syncclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func TestClientPeerRejectsReplacedConnection(t *testing.T) {
	var requests atomic.Int64
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer remote.Close()
	id := domain.NewID()
	c, err := library.CreateClient(t.Context(), t.TempDir(), library.ClientSettings{LibraryID: id})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	connection := library.ClientConnection{Server: remote.URL}
	if err := c.SetConnection(t.Context(), id, connection, "first-token"); err != nil {
		t.Fatal(err)
	}
	peer, err := NewForClient(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Capabilities(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetConnection(t.Context(), id, connection, "replacement-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Capabilities(t.Context()); err == nil {
		t.Fatal("stale peer used old connection")
	}
	if requests.Load() != 1 {
		t.Fatal("sent request after connection changed")
	}
}
