package library

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

func updateInstanceConfig(ctx context.Context, root, revision string, changes map[string]any) error {
	release, err := lockfile.New(filepath.Join(root, "config.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return config.Update(filepath.Join(root, "kist.toml"), revision, changes)
}

func legacyConfig(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid legacy configuration")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("trailing legacy configuration data")
	}
	return nil
}

// migrateInstanceConfig publishes the unified configuration before removing
// legacy files. Once version 2 exists, leftover files can never restore grants.
func migrateInstanceConfig(ctx context.Context, root, mode string) error {
	release, err := lockfile.New(filepath.Join(root, "config.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	path := filepath.Join(root, "kist.toml")
	cfg, err := config.Load(path)
	if err != nil && !(mode == "client" && errors.Is(err, os.ErrNotExist)) {
		return err
	}
	if cfg != nil && cfg.Mode != mode {
		return ErrNotALibrary
	}
	if cfg == nil {
		var old ClientSettings
		if err = legacyConfig(filepath.Join(root, "client.json"), &old); err != nil {
			return err
		}
		if err = old.validate(); err != nil {
			return err
		}
		cfg = config.Defaults("client")
		cfg.Client = config.Client{ID: old.ID, LibraryID: old.LibraryID, Timezone: old.Timezone, CacheMaxAge: old.CacheMaxAge, CacheMaxBytes: old.CacheMaxBytes}
		var connection clientConnectionFile
		err = legacyConfig(filepath.Join(root, "connection.json"), &connection)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			if connection.Version != 1 {
				return ErrSyncVersion
			}
			if connection.LibraryID != old.LibraryID {
				return ErrSyncConflict
			}
			if err = ValidateClientConnection(connection.ClientConnection, connection.Token); err != nil {
				return err
			}
			cfg.Client.Server, cfg.Client.AllowHTTP, cfg.Client.Token = connection.Server, connection.AllowHTTP, connection.Token
		}
		if err = config.Save(path, cfg); err != nil {
			return err
		}
	} else if cfg.Version < 2 {
		var access syncAccess
		err = legacyConfig(filepath.Join(root, "sync-access.json"), &access)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		grants := []config.Grant{}
		if err == nil {
			if access.Version != 1 {
				return ErrSyncVersion
			}
			for _, g := range access.Grants {
				grants = append(grants, config.Grant{ID: g.ID, CreatedAt: g.CreatedAt, Digest: g.Digest, Note: g.Note})
			}
		}
		if cfg.Library.Timezone == "" {
			return errors.New("set library.timezone to an IANA zone before migrating this legacy library")
		}
		changes := map[string]any{"version": 2, "mode": mode, "library.timezone": cfg.Library.Timezone}
		if len(grants) > 0 {
			changes["sync.grants"] = grants
		}
		if err = config.Update(path, "", changes); err != nil {
			return err
		}
	}
	if cfg != nil && cfg.Version < 3 {
		// Preserve the effective legacy opt-in policy before omission means
		// following the new default. This never initiates a location request.
		if err = config.Update(path, "", map[string]any{
			"version":             config.CurrentVersion,
			"geocoding.automatic": cfg.Geocoding.Automatic,
		}); err != nil {
			return err
		}
	}
	if err = os.Chmod(path, 0600); err != nil {
		return err
	}
	legacy := []string{"sync-access.json"}
	if mode == "client" {
		legacy = []string{"client.json", "connection.json"}
	}
	for _, name := range legacy {
		if err = os.Remove(filepath.Join(root, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return syncClientDir(root)
}

// InstanceSettings is the public, credential-free view of persisted settings.
type InstanceSettings struct {
	Revision             string            `json:"revision"`
	Mode                 string            `json:"mode"`
	ID                   string            `json:"id"`
	LibraryID            string            `json:"library_id"`
	Fields               []config.Field    `json:"fields"`
	Connection           *ClientConnection `json:"connection,omitempty"`
	CredentialConfigured bool              `json:"credential_configured"`
}

func instanceSettings(root string) (InstanceSettings, error) {
	raw, err := os.ReadFile(filepath.Join(root, "kist.toml"))
	if err != nil {
		return InstanceSettings{}, err
	}
	cfg, err := config.Decode(raw)
	if err != nil {
		return InstanceSettings{}, err
	}
	fields, err := config.Fields(raw)
	if err != nil {
		return InstanceSettings{}, err
	}
	out := InstanceSettings{Revision: config.Revision(raw), Mode: cfg.Mode, ID: cfg.Library.ID.String(), LibraryID: cfg.Library.ID.String(), Fields: fields}
	if cfg.Mode == "client" {
		out.ID = cfg.Client.ID.String()
		out.LibraryID = cfg.Client.LibraryID.String()
		out.CredentialConfigured = cfg.Client.Token != ""
		out.Connection = &ClientConnection{Server: cfg.Client.Server, AllowHTTP: cfg.Client.AllowHTTP}
	}
	return out, nil
}
func (l *Library) InstanceSettings(ctx context.Context) (InstanceSettings, error) {
	if err := ctx.Err(); err != nil {
		return InstanceSettings{}, err
	}
	return instanceSettings(l.layout.Root)
}
func (c *Client) InstanceSettings(ctx context.Context) (InstanceSettings, error) {
	if err := ctx.Err(); err != nil {
		return InstanceSettings{}, err
	}
	return instanceSettings(c.root)
}
func (l *Library) ServiceSettings() (config.Server, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return config.Server{}, err
	}
	return cfg.Server, nil
}
func (c *Client) ServiceSettings() (config.Server, error) {
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return config.Server{}, err
	}
	return cfg.Server, nil
}
func (l *Library) PrimaryColor() (string, error) {
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return "", err
	}
	return cfg.Appearance.PrimaryColor, nil
}
func (c *Client) PrimaryColor() (string, error) {
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return "", err
	}
	return cfg.Appearance.PrimaryColor, nil
}

func (l *Library) UpdateSettings(ctx context.Context, revision string, changes map[string]any, validators ...func(config.Server) error) error {
	return l.UpdateCombinedSettings(ctx, revision, changes, nil, nil, validators...)
}

// UpdateCombinedSettings publishes general, GPS and AI drafts in one validated
// configuration write. Activation boundaries are decided while both the archive
// and configuration locks are held, just as in the dedicated settings APIs.
func (l *Library) UpdateCombinedSettings(ctx context.Context, revision string, changes map[string]any, ai *OCRSettings, location *LocationSettings, validators ...func(config.Server) error) error {
	return l.UpdateCombinedAIServiceSettings(ctx, revision, changes, ai, location, nil, validators...)
}

// UpdateCombinedAIServiceSettings includes the layered AI draft in the same
// atomic publication as general and location settings.
func (l *Library) UpdateCombinedAIServiceSettings(ctx context.Context, revision string, changes map[string]any, ai *OCRSettings, location *LocationSettings, services *AIServiceSettings, validators ...func(config.Server) error) error {
	_, err := l.UpdateCombinedDraftSettings(ctx, revision, changes, ai, location, services, nil, validators...)
	return err
}

// UpdateCombinedDraftSettings saves credentials and all settings in one atomic
// configuration publication. Newly created secrets are returned only on success.
func (l *Library) UpdateCombinedDraftSettings(ctx context.Context, revision string, changes map[string]any, ai *OCRSettings, location *LocationSettings, services *AIServiceSettings, credentials *CredentialSettings, validators ...func(config.Server) error) ([]NewSyncCredential, error) {
	normalized, err := config.NormalizeChanges("library", changes)
	if err != nil {
		return nil, err
	}
	if revision == "" {
		return nil, config.ErrChanged
	}
	if ai == nil && location == nil && services == nil && credentials == nil {
		return nil, l.configure(ctx, revision, normalized, validators...)
	}
	if (ai != nil && ai.Revision != revision) || (location != nil && location.Revision != revision) {
		return nil, config.ErrChanged
	}
	if services != nil && (services.Revision != revision || ai != nil) {
		return nil, config.ErrChanged
	}
	release, err := l.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if err = l.catchUpLocked(ctx); err != nil {
		return nil, err
	}
	var created []NewSyncCredential
	err = l.configurePrepared(ctx, revision, normalized, func(cfg *config.Config) error {
		merge := func(values map[string]any, err error) error {
			if err != nil {
				return err
			}
			for key, value := range values {
				normalized[key] = value
			}
			return nil
		}
		if ai != nil {
			if cfg.AIServices != nil {
				return config.InvalidField("ai_services", "choice", "Use per-feature AI settings after migration.")
			}
			if err := merge(l.ocrSettingsChanges(cfg, *ai)); err != nil {
				return err
			}
		}
		if services != nil {
			if err := merge(l.aiServicesChanges(cfg, *services)); err != nil {
				return err
			}
		}

		if credentials != nil {
			var grants []config.Grant
			var err error
			grants, created, err = credentialSettingsChanges(cfg, *credentials, l.now())
			if err != nil {
				return err
			}
			normalized["sync.grants"] = grants
		}
		if location != nil {
			return merge(l.locationSettingsChanges(cfg, *location))
		}

		return nil
	}, validators...)
	if err != nil {
		return nil, err
	}
	return created, nil
}
func (c *Client) UpdateSettings(ctx context.Context, revision string, changes map[string]any, validators ...func(config.Server) error) error {
	normalized, err := config.NormalizeChanges("client", changes)
	if err != nil {
		return err
	}
	if revision == "" {
		return config.ErrChanged
	}
	return c.configure(ctx, revision, normalized, validators...)
}

// UpdateSettingsWithCredentials atomically saves relay settings and grant drafts.
func (c *Client) UpdateSettingsWithCredentials(ctx context.Context, revision string, changes map[string]any, credentials *CredentialSettings, validators ...func(config.Server) error) ([]NewSyncCredential, error) {
	normalized, err := config.NormalizeChanges("client", changes)
	if err != nil {
		return nil, err
	}
	if revision == "" {
		return nil, config.ErrChanged
	}
	if credentials == nil {
		return nil, c.configure(ctx, revision, normalized, validators...)
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	var created []NewSyncCredential
	err = c.configurePrepared(ctx, revision, normalized, func(cfg *config.Config) error {
		grants, secrets, err := credentialSettingsChanges(cfg, *credentials, time.Now())
		if err != nil {
			return err
		}
		normalized["sync.grants"], created = grants, secrets
		return nil
	}, validators...)
	if err != nil {
		return nil, err
	}
	return created, nil
}
