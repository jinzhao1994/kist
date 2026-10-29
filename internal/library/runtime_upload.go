package library

import (
	"github.com/jinzhao1994/kist/internal/upload"
	"path/filepath"
)

func (l *Library) UploadStaging(sync bool, maxBytes int64, maxFiles int) (*upload.Store, error) {
	db, err := l.runtimeStore()
	if err != nil {
		return nil, err
	}
	name := "uploads"
	if sync {
		name = "sync-uploads"
	}
	return upload.New(filepath.Join(l.layout.Root, name), maxBytes, maxFiles, db.DB)
}
func (c *Client) UploadStaging(sync bool, maxBytes int64, maxFiles int) (*upload.Store, error) {
	db, err := c.runtimeStore()
	if err != nil {
		return nil, err
	}
	name := "uploads"
	if sync {
		name = "sync-uploads"
	}
	return upload.New(filepath.Join(c.root, name), maxBytes, maxFiles, db.DB)
}
