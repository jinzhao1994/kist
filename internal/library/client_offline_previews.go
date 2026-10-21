package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/derive"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

type ClientOfflinePreview struct {
	Key       string          `json:"key"`
	Recipe    string          `json:"recipe"`
	Files     []domain.FileID `json:"files"`
	Available bool            `json:"available"`
}

func (c *Client) offlinePreviewsLocked(ctx context.Context, originals []ClientOfflineOriginal) ([]ClientOfflinePreview, error) {
	previews := []ClientOfflinePreview{}
	positions := map[string]int{}
	for _, original := range originals {
		for _, id := range original.Files {
			var name string
			if err := c.db.QueryRowContext(ctx, `SELECT name FROM files WHERE id=?`, id).Scan(&name); err != nil {
				return nil, err
			}
			info, err := readMediaInfo(c.db, id)
			if err != nil {
				return nil, err
			}
			profile, ok := derive.PreviewFor(name, info.Kind, info.MIME)
			if !ok {
				continue
			}
			recipes := []string{profile.Thumbnail}
			if profile.Detail != "" && profile.Detail != profile.Thumbnail {
				recipes = append(recipes, profile.Detail)
			}
			for _, recipe := range recipes {
				key := original.Key + "/" + recipe
				if position, ok := positions[key]; ok {
					previews[position].Files = append(previews[position].Files, id)
				} else {
					positions[key] = len(previews)
					previews = append(previews, ClientOfflinePreview{Key: original.Key, Recipe: recipe, Files: []domain.FileID{id}})
				}
			}
		}
	}
	return previews, nil
}

func waitOfflinePreview(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) prepareOfflinePreview(ctx context.Context, preview ClientOfflinePreview, source DerivedSource, render ClientRenderer) error {
	for {
		_, reader, err := c.OpenDerived(ctx, preview.Key, preview.Recipe)
		if err == nil {
			return reader.Close()
		}
		if !errors.Is(err, ErrNotCached) {
			return err
		}
		_, err = c.GenerateDerived(ctx, preview.Key, preview.Recipe, render)
		if err == nil {
			return nil
		}
		if lockfile.IsBusy(err) {
			if err = waitOfflinePreview(ctx); err != nil {
				return err
			}
			continue
		}
		var conversion *derive.Failure
		if !errors.Is(err, ErrNotCached) && !errors.As(err, &conversion) {
			return err
		}
		if source == nil {
			if conversion != nil {
				return fmt.Errorf("%w: %s", ErrMediaNotReady, conversion.Code)
			}
			return ErrNotCached
		}
		// A retry is explicit once per preparation; polling never repeatedly
		// restarts a failed remote generation or hides a terminal dependency error.
		for retry := true; ; retry = false {
			d, e := c.CacheDerived(ctx, preview.Key, preview.Recipe, retry, source)
			if e != nil {
				if lockfile.IsBusy(e) {
					if e = waitOfflinePreview(ctx); e == nil {
						continue
					}
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if conversion != nil {
					return fmt.Errorf("%w: %s", ErrMediaNotReady, conversion.Code)
				}
				return e
			}
			switch d.Job.State {
			case "ready":
				return nil
			case "queued", "running":
				if e = waitOfflinePreview(ctx); e != nil {
					return e
				}
			default:
				return fmt.Errorf("%w: %s", ErrMediaNotReady, d.Job.Reason)
			}
		}
	}
}
