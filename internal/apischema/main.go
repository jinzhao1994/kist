// Command apischema prints the live HTTP contract without starting a listener.
package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"

	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/library"
)

func run() error {
	root, err := os.MkdirTemp("", "kist-schema-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if _, err := library.Create(context.Background(), root); err != nil {
		return err
	}
	l, err := library.Open(root)
	if err != nil {
		return err
	}
	defer l.Close()
	handler, err := api.New(l, api.Options{AllowedHosts: []string{"schema.local"}})
	if err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "--client" {
		clientRoot, err := os.MkdirTemp("", "kist-client-schema-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(clientRoot)
		head, err := l.SyncHead(context.Background())
		if err != nil {
			return err
		}
		client, err := library.CreateClient(context.Background(), clientRoot, library.ClientSettings{LibraryID: head.LibraryID})
		if err != nil {
			return err
		}
		defer client.Close()
		handler, err = api.NewClient(client, api.Options{AllowedHosts: []string{"schema.local"}})
		if err != nil {
			return err
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "http://schema.local"+api.Prefix+"/openapi.json", nil))
	if recorder.Code != 200 {
		return fmt.Errorf("schema response: %d", recorder.Code)
	}
	_, err = os.Stdout.Write(recorder.Body.Bytes())
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
