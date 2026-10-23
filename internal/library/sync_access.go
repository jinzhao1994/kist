package library

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
)

type SyncCredential struct {
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

func (l *Library) syncAccess() (syncAccess, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return syncAccess{}, err
	}
	access := syncAccess{Version: 1, Grants: []syncGrant{}}
	for _, g := range cfg.Sync.Grants {
		access.Grants = append(access.Grants, syncGrant{SyncCredential: SyncCredential{ID: g.ID, CreatedAt: g.CreatedAt}, Digest: g.Digest})
	}
	return access, nil
}

// AuthorizeSync explicitly enables read/write sync for one independently
// revocable credential. Operational credentials are configuration, not archive
// events, and cannot be restored by undoing an archive operation.
func (l *Library) AuthorizeSync(ctx context.Context) (NewSyncCredential, error) {
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return NewSyncCredential{}, err
	}
	defer release()
	access, err := l.syncAccess()
	if err != nil {
		return NewSyncCredential{}, err
	}
	var id [16]byte
	var secret [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return NewSyncCredential{}, err
	}
	if _, err := rand.Read(secret[:]); err != nil {
		return NewSyncCredential{}, err
	}
	credential := NewSyncCredential{SyncCredential: SyncCredential{ID: hex.EncodeToString(id[:]), CreatedAt: l.now().UTC()}}
	credential.Token = "kist_sync." + credential.ID + "." + base64.RawURLEncoding.EncodeToString(secret[:])
	digest := sha256.Sum256([]byte(credential.Token))
	access.Grants = append(access.Grants, syncGrant{SyncCredential: credential.SyncCredential, Digest: hex.EncodeToString(digest[:])})
	if err := l.saveSyncAccess(ctx, access); err != nil {
		return NewSyncCredential{}, err
	}
	return credential, nil
}

func (l *Library) SyncCredentials(ctx context.Context) ([]SyncCredential, error) {
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

func (l *Library) RevokeSync(ctx context.Context, id string) (bool, error) {
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

// CheckSyncCredential reads current configuration on every request, so a grant
// revoked in another process cannot survive in a running server's memory.
func (l *Library) CheckSyncCredential(ctx context.Context, token string) (bool, error) {
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

func (l *Library) saveSyncAccess(ctx context.Context, access syncAccess) error {
	grants := []config.Grant{}
	for _, g := range access.Grants {
		grants = append(grants, config.Grant{ID: g.ID, CreatedAt: g.CreatedAt, Digest: g.Digest})
	}
	return updateInstanceConfig(ctx, l.layout.Root, "", map[string]any{"sync.grants": grants})
}
