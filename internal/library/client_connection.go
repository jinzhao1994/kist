package library

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	u, err := url.Parse(connection.Server)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("sync requires an HTTP(S) server URL without credentials, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || ip != nil && ip.IsLoopback()
	if u.Scheme == "http" && !loopback && !connection.AllowHTTP {
		return errors.New("sync requires HTTPS outside loopback unless cleartext transport is explicitly enabled")
	}
	if token == "" || len(token) > 2048 || strings.IndexFunc(token, func(r rune) bool { return r <= 0x20 || r >= 0x7f }) >= 0 {
		return errors.New("a valid sync credential is required")
	}
	return nil
}

// SetConnection follows a successful authenticated capability check. Repointing
// a client to another library is forbidden even when its local catalog is empty.
func (c *Client) SetConnection(ctx context.Context, libraryID domain.ID, connection ClientConnection, token string) error {
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
	return writeClientJSON(filepath.Join(c.root, "connection.json"), clientConnectionFile{Version: 1, LibraryID: libraryID, ClientConnection: connection, Token: token})
}

func (c *Client) connection() (ClientConnection, string, error) {
	var file clientConnectionFile
	if err := readClientJSON(filepath.Join(c.root, "connection.json"), &file); err != nil {
		return ClientConnection{}, "", err
	}
	if file.Version != 1 {
		return ClientConnection{}, "", ErrSyncVersion
	}
	settings, err := c.settings()
	if err != nil {
		return ClientConnection{}, "", err
	}
	if file.LibraryID != settings.LibraryID {
		return ClientConnection{}, "", ErrSyncConflict
	}
	if err := ValidateClientConnection(file.ClientConnection, file.Token); err != nil {
		return ClientConnection{}, "", err
	}
	return file.ClientConnection, file.Token, nil
}

// Connection reads current configuration, including changes by another process.
func (c *Client) Connection(ctx context.Context) (ClientConnection, string, error) {
	if err := ctx.Err(); err != nil {
		return ClientConnection{}, "", err
	}
	connection, token, err := c.connection()
	if errors.Is(err, os.ErrNotExist) {
		return ClientConnection{}, "", errors.New("client is not connected to a NAS; configure a connection first")
	}
	return connection, token, err
}
