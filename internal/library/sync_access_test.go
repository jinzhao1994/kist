package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSyncCredentialsPersistWithoutSecretsAndRevokeAcrossHandles(t *testing.T) {
	l := openTestLibrary(t)
	credential, err := l.AuthorizeSync(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := l.CheckSyncCredential(t.Context(), credential.Token); err != nil || !ok {
		t.Fatalf("new credential: %v %v", ok, err)
	}
	for _, invalid := range []string{"", "Bearer " + credential.Token, credential.Token + "x", "kist_sync.invalid.invalid"} {
		if ok, err := l.CheckSyncCredential(t.Context(), invalid); err != nil || ok {
			t.Fatalf("invalid credential accepted: %v %v", ok, err)
		}
	}
	path := filepath.Join(l.layout.Root, "sync-access.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.Split(credential.Token, ".")[2]
	if strings.Contains(string(raw), credential.Token) || strings.Contains(string(raw), secret) {
		t.Fatal("stored bearer secret instead of digest")
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("credential permissions=%v err=%v", info, err)
	}
	list, err := l.SyncCredentials(t.Context())
	encoded, _ := json.Marshal(list)
	if err != nil || len(list) != 1 || strings.Contains(string(encoded), "digest") || strings.Contains(string(encoded), secret) {
		t.Fatalf("credential list=%s err=%v", encoded, err)
	}
	other, err := Open(l.layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if ok, err := other.RevokeSync(t.Context(), credential.ID); err != nil || !ok {
		t.Fatalf("revoke=%v %v", ok, err)
	}
	if ok, err := l.CheckSyncCredential(t.Context(), credential.Token); err != nil || ok {
		t.Fatalf("running handle retained revoked credential: %v %v", ok, err)
	}
	if count, _ := l.OpCount(t.Context()); count != 0 {
		t.Fatal("credential management polluted archive history")
	}
	if err := writeClientJSON(path, syncAccess{Version: 2}); err != nil {
		t.Fatal(err)
	}
	if ok, err := l.CheckSyncCredential(t.Context(), credential.Token); !errors.Is(err, ErrSyncVersion) || ok {
		t.Fatalf("unknown credential configuration failed open: %v %v", ok, err)
	}
}
