package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func cachePinsBytes(c *Client) ([]byte, error) {
	pins, err := c.readCachePins()
	if err != nil {
		return nil, err
	}
	return json.Marshal(pins)
}
func damageCacheObject(c *Client, key string) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE cache_objects SET offset=-1 WHERE key=?`, key)
	return err
}

func damageDerivedRecord(c *Client, path string) error {
	db, err := c.cacheDatabase()
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	i := strings.LastIndex(base, ".")
	_, err = db.DB.Exec(`UPDATE cache_derivations SET descriptor='{' WHERE original=? AND recipe=?`, "sha256:"+base[:i], base[i+1:])
	return err
}
func testCacheRecordBytes(c *Client, path string) ([]byte, error) {
	if filepath.Base(filepath.Dir(path)) == "derived" {
		r, e := c.loadDerivedRecord(path)
		if e != nil {
			return nil, e
		}
		return json.Marshal(r)
	}
	r, e := c.readCacheObject("sha256:" + strings.TrimSuffix(filepath.Base(path), ".json"))
	if e != nil {
		return nil, e
	}
	return json.Marshal(r)
}

func writeClientJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".client-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncClientDir(filepath.Dir(path))
}
