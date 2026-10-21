package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
)

func TestSyncAccessCLI(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nas")
	if _, err := library.Create(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	out, err := runKist(t, "", "--library", root, "--json", "sync-access", "create")
	var credential library.NewSyncCredential
	if err != nil || json.Unmarshal([]byte(out), &credential) != nil || credential.Token == "" {
		t.Fatalf("create=%s err=%v", out, err)
	}
	out, err = runKist(t, "", "--library", root, "sync-access", "list")
	if err != nil || !strings.Contains(out, credential.ID) || strings.Contains(out, credential.Token) {
		t.Fatalf("list=%s err=%v", out, err)
	}
	if _, err := runKist(t, "", "--library", root, "sync-access", "revoke", credential.ID); err != nil {
		t.Fatal(err)
	}
	out, err = runKist(t, "", "--library", root, "sync-access", "list")
	if err != nil || strings.Contains(out, credential.ID) {
		t.Fatalf("revoked list=%s err=%v", out, err)
	}
}
