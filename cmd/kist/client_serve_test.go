package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func TestClientServeWithoutNASConnection(t *testing.T) {
	nas, _, _ := clientNAS(t)
	head, err := nas.SyncHead(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	c, err := library.CreateClient(t.Context(), root, library.ClientSettings{LibraryID: head.LibraryID})
	if err != nil {
		t.Fatal(err)
	}
	box, err := c.CreateBox(t.Context(), "Offline", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := newClientServeCmd()
	cmd.SetArgs([]string{root, "--listen", address})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	var response *http.Response
	for {
		response, err = client.Get("http://" + address + "/api/v1/boxes")
		if err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("serve exited: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var boxes wire.BoxList
	if err := json.NewDecoder(response.Body).Decode(&boxes); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || len(boxes.Boxes) != 1 || boxes.Boxes[0].ID != box.ID.String() {
		t.Fatalf("response=%d %+v", response.StatusCode, boxes)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client server did not stop")
	}
}
