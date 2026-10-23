package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
)

// ClientConnection is safe to display. Credentials are returned separately,
// never embedded in status output or archive operations.
type ClientConnection struct {
	Server    string `json:"server"`
	AllowHTTP bool   `json:"allow_http"`
}

type clientConnectionFile struct {
	Version   int       `json:"version"`
	LibraryID domain.ID `json:"library_id"`
	ClientConnection
	Token string `json:"token"`
}

func ValidateClientConnection(connection ClientConnection, token string) error {
	return config.ValidateConnection(connection.Server, connection.AllowHTTP, token)
}

// SetConnection follows a successful authenticated capability check. Repointing
// a client to another library is forbidden even when its local catalog is empty.
func (c *Client) SetConnection(ctx context.Context, libraryID domain.ID, connection ClientConnection, token string) error {
	return c.SetConnectionRevision(ctx, libraryID, connection, token, "")
}

func (c *Client) SetConnectionRevision(ctx context.Context, libraryID domain.ID, connection ClientConnection, token, revision string) error {
	if err := ValidateClientConnection(connection, token); err != nil {
		return err
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return err
	}
	if settings.LibraryID != libraryID {
		return ErrSyncConflict
	}
	return updateInstanceConfig(ctx, c.root, revision, map[string]any{"client.server": connection.Server, "client.allow_http": connection.AllowHTTP, "client.token": token})
}

func (c *Client) connection() (ClientConnection, string, error) {
	cfg, err := config.Load(filepath.Join(c.root, "kist.toml"))
	if err != nil {
		return ClientConnection{}, "", err
	}
	if cfg.Mode != "client" {
		return ClientConnection{}, "", ErrNotALibrary
	}
	if cfg.Client.Server == "" && cfg.Client.Token == "" {
		return ClientConnection{}, "", os.ErrNotExist
	}
	connection := ClientConnection{Server: cfg.Client.Server, AllowHTTP: cfg.Client.AllowHTTP}
	if err := ValidateClientConnection(connection, cfg.Client.Token); err != nil {
		return ClientConnection{}, "", err
	}
	return connection, cfg.Client.Token, nil
}

// Connection reads current configuration, including changes by another process.
func (c *Client) Connection(ctx context.Context) (ClientConnection, string, error) {
	if err := ctx.Err(); err != nil {
		return ClientConnection{}, "", err
	}
	connection, token, err := c.connection()
	if errors.Is(err, os.ErrNotExist) {
		return ClientConnection{}, "", errors.New("client is not connected to a remote; configure a connection first")
	}
	return connection, token, err
}
