package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/config"
)

func TestCredentialDraftAtomicSaveAndRevocation(t *testing.T) {
	for _, mode := range []string{"library", "client"} {
		t.Run(mode, func(t *testing.T) {
			l := openTestLibrary(t)
			root := l.layout.Root
			var access interface {
				AuthorizeSyncWithNote(context.Context, string) (NewSyncCredential, error)
				CheckSyncCredential(context.Context, string) (bool, error)
			} = l
			save := func(revision string, changes map[string]any, draft *CredentialSettings) ([]NewSyncCredential, error) {
				return l.UpdateCombinedDraftSettings(t.Context(), revision, changes, nil, nil, nil, draft)
			}
			if mode == "client" {
				c := newSyncClient(t, l)
				access, root = c, c.root
				save = func(revision string, changes map[string]any, draft *CredentialSettings) ([]NewSyncCredential, error) {
					return c.UpdateSettingsWithCredentials(t.Context(), revision, changes, draft)
				}
			}
			old, err := access.AuthorizeSyncWithNote(t.Context(), "original")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "kist.toml")
			before, _ := os.ReadFile(path)
			cfg, _ := config.Load(path)
			oldGrant := cfg.Sync.Grants[0]
			invalid := &CredentialSettings{Items: []CredentialDraft{{Note: strings.Repeat("字", 201)}}}
			if created, err := save(config.Revision(before), map[string]any{"appearance.primary_color": "green"}, invalid); err == nil || len(created) != 0 {
				t.Fatal("invalid draft was accepted or exposed a secret", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatal("invalid draft partially saved other settings")
			}
			draft := &CredentialSettings{Items: []CredentialDraft{{ID: old.ID, Note: " changed "}, {Note: "new device"}}}
			created, err := save(config.Revision(before), map[string]any{"appearance.primary_color": "green"}, draft)
			if err != nil || len(created) != 1 {
				t.Fatal(created, err)
			}
			cfg, _ = config.Load(path)
			if cfg.Appearance.PrimaryColor != "green" || len(cfg.Sync.Grants) != 2 || cfg.Sync.Grants[0].Note != "changed" || cfg.Sync.Grants[0].Digest != oldGrant.Digest || !cfg.Sync.Grants[0].CreatedAt.Equal(oldGrant.CreatedAt) {
				t.Fatal("combined save changed credential identity or lost settings")
			}
			for _, token := range []string{old.Token, created[0].Token} {
				if valid, err := access.CheckSyncCredential(t.Context(), token); err != nil || !valid {
					t.Fatal("saved credential is not usable", err)
				}
			}
			if secrets, err := save(config.Revision(before), map[string]any{}, draft); !errors.Is(err, config.ErrChanged) || len(secrets) != 0 {
				t.Fatal("stale draft minted or returned credentials", err)
			}
			raw, _ := os.ReadFile(path)
			if strings.Contains(string(raw), created[0].Token) {
				t.Fatal("plaintext credential was persisted")
			}
			for _, bad := range []*CredentialSettings{
				{Items: []CredentialDraft{{ID: "missing"}}},
				{Items: []CredentialDraft{{ID: old.ID}, {ID: old.ID}}},
			} {
				if secrets, err := save(config.Revision(raw), map[string]any{}, bad); err == nil || len(secrets) != 0 {
					t.Fatal("invalid grant reference accepted")
				}
			}
			if _, err := save(config.Revision(raw), map[string]any{}, &CredentialSettings{}); err != nil {
				t.Fatal(err)
			}
			for _, token := range []string{old.Token, created[0].Token} {
				if valid, err := access.CheckSyncCredential(t.Context(), token); err != nil || valid {
					t.Fatal("revoked credential still usable", err)
				}
			}
		})
	}
}

func TestCredentialDraftRollsBackWhenAnotherSectionFails(t *testing.T) {
	l := openTestLibrary(t)
	before, _ := l.InstanceSettings(t.Context())
	location, _ := l.LocationSettings(t.Context())
	location.Endpoint, location.Changed = "invalid", []string{"endpoint"}
	created, err := l.UpdateCombinedDraftSettings(t.Context(), before.Revision, map[string]any{}, nil, &location, nil, &CredentialSettings{Items: []CredentialDraft{{Note: "must not be issued"}}})
	after, _ := l.InstanceSettings(t.Context())
	if err == nil || len(created) != 0 || before.Revision != after.Revision {
		t.Fatal("another section failure issued a credential", err)
	}
}
