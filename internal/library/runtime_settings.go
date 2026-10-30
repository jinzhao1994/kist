package library

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type loadedSettings struct {
	config *config.Config
	zone   *time.Location
}

// configureInstance serializes validation, durable publication and activation.
// Validation must not change running state. Activation cannot fail.
func configureInstance(ctx context.Context, root, revision string, changes map[string]any, validate func(*config.Config) error, activate func(), prepare ...func(*config.Config) error) error {
	release, err := lockfile.New(filepath.Join(root, "config.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	path := filepath.Join(root, "kist.toml")
	if changes != nil {
		if len(prepare) > 0 && prepare[0] != nil {
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := prepare[0](cfg); err != nil {
				return err
			}
		}
		err = config.UpdateValidated(path, revision, changes, validate)
	} else {
		var cfg *config.Config
		cfg, err = config.Load(path)
		if err == nil {
			err = validate(cfg)
		}
	}
	if err != nil {
		return err
	}
	activate()
	return nil
}

func (l *Library) configure(ctx context.Context, revision string, changes map[string]any, validators ...func(config.Server) error) error {
	return l.configurePrepared(ctx, revision, changes, nil, validators...)
}

func (l *Library) configurePrepared(ctx context.Context, revision string, changes map[string]any, prepare func(*config.Config) error, validators ...func(config.Server) error) error {
	var next *loadedSettings
	return configureInstance(ctx, l.layout.Root, revision, changes, func(cfg *config.Config) error {
		old := l.settings.Load()
		if cfg.Mode != "library" || cfg.Library.ID != old.config.Library.ID {
			return fmt.Errorf("instance identity cannot change")
		}
		zone, err := cfg.Library.Zone()
		if err != nil {
			return err
		}
		mode := cfg.Storage.LinkMode
		if mode == "" {
			mode, err = blob.DetectLinkMode(l.blobs.Root(), l.layout.Boxes())
		} else if mode != old.config.Storage.LinkMode {
			err = blob.CheckLinkMode(l.blobs.Root(), l.layout.Boxes(), mode)
			if err != nil {
				return config.InvalidField("storage.link_mode", "link_mode", "The selected projection method is unavailable on this filesystem.")
			}
		}
		if err != nil {
			return err
		}
		for _, validate := range validators {
			if validate == nil {
				continue
			}
			if err := validate(cfg.Server); err != nil {
				return err
			}
		}
		// The resolved default is runtime state, not an explicit TOML setting.
		copy := *cfg
		copy.Storage.LinkMode = mode
		next = &loadedSettings{config: &copy, zone: zone}
		return nil
	}, func() { l.settings.Store(next) }, prepare)
}

// ReloadSettings validates and activates a file edited outside the web UI.
func (l *Library) ReloadSettings(ctx context.Context, validate func(config.Server) error) error {
	return l.configure(ctx, "", nil, validate)
}

func (c *Client) configure(ctx context.Context, revision string, changes map[string]any, validators ...func(config.Server) error) error {
	return configureInstance(ctx, c.root, revision, changes, func(cfg *config.Config) error {
		if cfg.Mode != "client" {
			return ErrNotALibrary
		}
		for _, validate := range validators {
			if validate == nil {
				continue
			}
			if err := validate(cfg.Server); err != nil {
				return err
			}
		}
		return nil
	}, func() {})
}

// ReloadSettings validates client configuration without reopening its catalog.
func (c *Client) ReloadSettings(ctx context.Context, validate func(config.Server) error) error {
	return c.configure(ctx, "", nil, validate)
}

// RuntimeStorageSettings returns the active interpretation and projection policy.
func (l *Library) RuntimeStorageSettings() (timezone, projection string) {
	current := l.settings.Load()
	return current.zone.String(), string(current.config.Storage.LinkMode)
}
