package library

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
)

type SyncCredential struct {
	Note      string    `json:"note,omitempty"`
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSyncCredential reveals the bearer secret once. It is never stored in the
// journal, the database or the credential listing.
type NewSyncCredential struct {
	SyncCredential
	Token string `json:"token"`
}

type syncGrant struct {
	SyncCredential
	Digest string `json:"digest"`
}

type syncAccess struct {
	Version int         `json:"version"`
	Grants  []syncGrant `json:"grants"`
}

func (l syncAccessStore) syncAccess() (syncAccess, error) {
	cfg, err := config.Load(filepath.Join(l.root, "kist.toml"))
	if err != nil {
		return syncAccess{}, err
	}
	access := syncAccess{Version: 1, Grants: []syncGrant{}}
	for _, g := range cfg.Sync.Grants {
		access.Grants = append(access.Grants, syncGrant{SyncCredential: SyncCredential{ID: g.ID, CreatedAt: g.CreatedAt, Note: g.Note}, Digest: g.Digest})
	}
	return access, nil
}

// AuthorizeSync explicitly enables read/write sync for one independently
// revocable credential. Operational credentials are configuration, not archive
// events, and cannot be restored by undoing an archive operation.
func (l syncAccessStore) AuthorizeSync(ctx context.Context) (NewSyncCredential, error) {
	return l.AuthorizeSyncWithNote(ctx, "")
}

// AuthorizeSyncWithNote creates a credential with an optional human-readable label.
func (l syncAccessStore) AuthorizeSyncWithNote(ctx context.Context, note string) (NewSyncCredential, error) {
	note = strings.TrimSpace(note)
	if !config.ValidCredentialNote(note) {
		return NewSyncCredential{}, ErrInvalidRequest
	}

	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return NewSyncCredential{}, err
	}
	defer release()
	access, err := l.syncAccess()
	if err != nil {
		return NewSyncCredential{}, err
	}
	credential, digest, err := newSyncCredential(note, l.now().UTC())
	if err != nil {
		return NewSyncCredential{}, err
	}
	access.Grants = append(access.Grants, syncGrant{SyncCredential: credential.SyncCredential, Digest: digest})
	if err := l.saveSyncAccess(ctx, access); err != nil {
		return NewSyncCredential{}, err
	}
	return credential, nil
}

func (l syncAccessStore) SyncCredentials(ctx context.Context) ([]SyncCredential, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	access, err := l.syncAccess()
	if err != nil {
		return nil, err
	}
	out := make([]SyncCredential, 0, len(access.Grants))
	for _, grant := range access.Grants {
		out = append(out, grant.SyncCredential)
	}
	return out, nil
}

func (l syncAccessStore) RevokeSync(ctx context.Context, id string) (bool, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	access, err := l.syncAccess()
	if err != nil {
		return false, err
	}
	for i, grant := range access.Grants {
		if grant.ID == id {
			access.Grants = append(access.Grants[:i], access.Grants[i+1:]...)
			return true, l.saveSyncAccess(ctx, access)
		}
	}
	return false, nil
}

// SetSyncCredentialNote changes only the display label of an existing grant.
func (l syncAccessStore) SetSyncCredentialNote(ctx context.Context, id, note string) (bool, error) {
	note = strings.TrimSpace(note)
	if !config.ValidCredentialNote(note) {
		return false, ErrInvalidRequest
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	access, err := l.syncAccess()
	if err != nil {
		return false, err
	}
	for i := range access.Grants {
		if access.Grants[i].ID == id {
			access.Grants[i].Note = note
			return true, l.saveSyncAccess(ctx, access)
		}
	}
	return false, nil
}

// CheckSyncCredential reads current configuration on every request, so a grant
// revoked in another process cannot survive in a running server's memory.
func (l syncAccessStore) CheckSyncCredential(ctx context.Context, token string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	parts := strings.Split(token, ".")
	if len(token) > 128 || len(parts) != 3 || parts[0] != "kist_sync" || len(parts[1]) != 32 {
		return false, nil
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(secret) != 32 {
		return false, nil
	}
	access, err := l.syncAccess()
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256([]byte(token))
	for _, grant := range access.Grants {
		if grant.ID == parts[1] {
			want, _ := hex.DecodeString(grant.Digest)
			return subtle.ConstantTimeCompare(digest[:], want) == 1, nil
		}
	}
	return false, nil
}

func (l syncAccessStore) saveSyncAccess(ctx context.Context, access syncAccess) error {
	grants := []config.Grant{}
	for _, g := range access.Grants {
		grants = append(grants, config.Grant{ID: g.ID, CreatedAt: g.CreatedAt, Note: g.Note, Digest: g.Digest})
	}
	return updateInstanceConfig(ctx, l.root, "", map[string]any{"sync.grants": grants})
}

// syncAccessStore is operational configuration shared by root and relay nodes.
type syncAccessStore struct {
	root string
	lock *lockfile.Locker
	now  func() time.Time
}

func (l *Library) AuthorizeSync(ctx context.Context) (NewSyncCredential, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).AuthorizeSync(ctx)
}

func (l *Library) AuthorizeSyncWithNote(ctx context.Context, note string) (NewSyncCredential, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).AuthorizeSyncWithNote(ctx, note)
}

func (l *Library) SyncCredentials(ctx context.Context) ([]SyncCredential, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).SyncCredentials(ctx)
}

func (l *Library) RevokeSync(ctx context.Context, id string) (bool, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).RevokeSync(ctx, id)
}

func (l *Library) SetSyncCredentialNote(ctx context.Context, id, note string) (bool, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).SetSyncCredentialNote(ctx, id, note)
}

func (l *Library) CheckSyncCredential(ctx context.Context, token string) (bool, error) {
	return (syncAccessStore{root: l.layout.Root, lock: l.lock, now: l.now}).CheckSyncCredential(ctx, token)
}

func (l *Client) AuthorizeSync(ctx context.Context) (NewSyncCredential, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).AuthorizeSync(ctx)
}

func (l *Client) AuthorizeSyncWithNote(ctx context.Context, note string) (NewSyncCredential, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).AuthorizeSyncWithNote(ctx, note)
}

func (l *Client) SyncCredentials(ctx context.Context) ([]SyncCredential, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).SyncCredentials(ctx)
}

func (l *Client) RevokeSync(ctx context.Context, id string) (bool, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).RevokeSync(ctx, id)
}

func (l *Client) SetSyncCredentialNote(ctx context.Context, id, note string) (bool, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).SetSyncCredentialNote(ctx, id, note)
}

func (l *Client) CheckSyncCredential(ctx context.Context, token string) (bool, error) {
	return (syncAccessStore{root: l.root, lock: l.lock, now: time.Now}).CheckSyncCredential(ctx, token)
}

// CredentialDraft names an existing grant or requests a new one with an empty ID.
// Omitted existing grants are explicitly revoked when the whole draft is saved.
type CredentialDraft struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

type CredentialSettings struct {
	Items []CredentialDraft `json:"items"`
}

func newSyncCredential(note string, now time.Time) (NewSyncCredential, string, error) {
	var id [16]byte
	var secret [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return NewSyncCredential{}, "", err
	}
	if _, err := rand.Read(secret[:]); err != nil {
		return NewSyncCredential{}, "", err
	}
	credential := NewSyncCredential{SyncCredential: SyncCredential{ID: hex.EncodeToString(id[:]), CreatedAt: now.UTC(), Note: note}}
	credential.Token = "kist_sync." + credential.ID + "." + base64.RawURLEncoding.EncodeToString(secret[:])
	digest := sha256.Sum256([]byte(credential.Token))
	return credential, hex.EncodeToString(digest[:]), nil
}

func credentialSettingsChanges(cfg *config.Config, draft CredentialSettings, now time.Time) ([]config.Grant, []NewSyncCredential, error) {
	existing := map[string]config.Grant{}
	for _, grant := range cfg.Sync.Grants {
		existing[grant.ID] = grant
	}
	seen := map[string]bool{}
	for i, item := range draft.Items {
		field := fmt.Sprintf("credentials.items.%d", i)
		if !config.ValidCredentialNote(strings.TrimSpace(item.Note)) {
			return nil, nil, config.InvalidField(field+".note", "credential_note", "Use at most 200 characters.")
		}
		if item.ID == "" {
			continue
		}
		if _, ok := existing[item.ID]; !ok || seen[item.ID] {
			return nil, nil, config.InvalidField(field+".id", "reference", "Use a distinct existing credential.")
		}
		seen[item.ID] = true
	}
	grants := make([]config.Grant, 0, len(draft.Items))
	created := []NewSyncCredential{}
	for _, item := range draft.Items {
		note := strings.TrimSpace(item.Note)
		if item.ID != "" {
			grant := existing[item.ID]
			grant.Note = note
			grants = append(grants, grant)
			continue
		}
		credential, digest, err := newSyncCredential(note, now)
		if err != nil {
			return nil, nil, err
		}
		grants = append(grants, config.Grant{ID: credential.ID, CreatedAt: credential.CreatedAt, Note: note, Digest: digest})
		created = append(created, credential)
	}
	return grants, created, nil
}
