package library

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/projection"
	"github.com/jinzhao1994/kist/internal/store"
)

// NavigationTarget selects a logical object by current path or stable identity.
// Path is an escaped browser pathname, decoded exactly once per segment.
type NavigationTarget struct {
	Path       string
	Box        domain.BoxID
	Folder     domain.FolderID
	File       domain.ID
	FolderView bool
}

// NavigationLocation describes both forms of an object's current address.
type NavigationLocation struct {
	Box    string `json:"box"`
	Folder string `json:"folder,omitempty"`
	File   string `json:"file,omitempty"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	View   string `json:"view,omitempty"`
}

func (l *Library) Navigation(ctx context.Context, target NavigationTarget) (NavigationLocation, error) {
	var result NavigationLocation
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = resolveNavigation(tx, target)
		return err
	})
	return result, err
}
func (c *Client) Navigation(ctx context.Context, target NavigationTarget) (NavigationLocation, error) {
	var result NavigationLocation
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		result, err = resolveNavigation(tx, target)
		return err
	})
	return result, err
}

func readableSegments(path string) ([]string, error) {
	if !strings.HasPrefix(path, "/boxes/") {
		return nil, ErrInvalidRequest
	}
	raw := strings.Split(strings.TrimSuffix(strings.TrimPrefix(path, "/boxes/"), "/"), "/")
	if len(raw) > 34 {
		return nil, ErrInvalidRequest
	}
	for i, part := range raw {
		name, err := url.PathUnescape(part)
		if err != nil || !utf8.ValidString(name) || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
			return nil, ErrInvalidRequest
		}
		raw[i] = name
	}
	return raw, nil
}

func resolveNavigation(tx *store.ViewTx, target NavigationTarget) (NavigationLocation, error) {
	var result NavigationLocation
	boxes, err := store.ListBoxes(tx.Queryer(), store.ListBoxesOptions{})
	if err != nil {
		return result, err
	}
	named := make([]projection.Named, len(boxes))
	for i, b := range boxes {
		named[i] = projection.Named{ID: b.ID.ID, Name: projection.BoxDirName(b)}
	}
	slugs := projection.Resolve(named)
	if target.Path != "" {
		if !target.Box.IsZero() || !target.Folder.IsZero() || !target.File.IsZero() {
			return result, ErrInvalidRequest
		}
		parts, err := readableSegments(target.Path)
		if err != nil {
			return result, err
		}
		var root domain.FolderID
		for _, b := range boxes {
			if domain.Fold(slugs[b.ID.ID]) == domain.Fold(parts[0]) {
				target.Box = b.ID
				root = b.RootFolderID
				break
			}
		}
		if root.IsZero() {
			return result, ErrNoSuchBox
		}
		current := root
		for i, name := range parts[1:] {
			id, folder, found, err := store.EntryNamed(tx.Queryer(), current, name)
			if err != nil {
				return result, err
			}
			if !found {
				return result, ErrNoSuchFile
			}
			if !folder {
				if i != len(parts)-2 || target.FolderView {
					return result, ErrNoSuchFile
				}
				target.File = id
			} else {
				current = domain.FolderID{ID: id}
				target.Folder = current
			}
		}
		if !target.Folder.IsZero() && target.File.IsZero() && !target.FolderView {
			set, err := tx.GetFileset(target.Folder)
			if err != nil {
				return result, err
			}
			if set != nil {
				target.File = target.Folder.ID
				target.Folder = domain.FolderID{}
			}
		}
	}
	var leaf string
	var folder domain.FolderID
	var objectFolder domain.FolderID
	if !target.File.IsZero() {
		f, found, err := tx.GetFile(domain.FileID{ID: target.File})
		if err != nil {
			return result, err
		}
		if found {
			result.Kind = "file"
			folder = f.FolderID
			leaf = f.Name
		} else {
			objectFolder = domain.FolderID{ID: target.File}
			set, err := tx.GetFileset(objectFolder)
			if err != nil {
				return result, err
			}
			if set == nil {
				return result, ErrNoSuchFile
			}
			f, found, err := tx.GetFolder(objectFolder)
			if err != nil {
				return result, err
			}
			if !found || f.ParentID == nil {
				return result, ErrNoSuchFile
			}
			result.Kind = "fileset"
			folder = *f.ParentID
			leaf = f.Name
		}
		result.File = target.File.String()
	} else if !target.Folder.IsZero() {
		folder = target.Folder
		result.Kind = "folder"
	} else {
		b, found, err := tx.GetBox(target.Box)
		if err != nil {
			return result, err
		}
		if !found {
			return result, ErrNoSuchBox
		}
		folder = b.RootFolderID
		result.Kind = "box"
	}
	result.Folder = folder.String()
	var names []string
	seen := map[domain.FolderID]bool{}
	for {
		if seen[folder] || len(seen) > 32 {
			return result, ErrInvariantViolated
		}
		seen[folder] = true
		f, found, err := tx.GetFolder(folder)
		if err != nil {
			return result, err
		}
		if !found {
			return result, ErrNoSuchFile
		}
		if f.ParentID == nil {
			break
		}
		names = append(names, f.Name)
		folder = *f.ParentID
	}
	var box domain.Box
	for _, b := range boxes {
		if b.RootFolderID == folder {
			box = b
			break
		}
	}
	if box.ID.IsZero() {
		return result, ErrNoSuchBox
	}
	result.Box = box.ID.String()
	if result.Kind == "folder" {
		if target.Folder == box.RootFolderID {
			result.Kind = "box"
		} else {
			set, err := tx.GetFileset(target.Folder)
			if err != nil {
				return result, err
			}
			if set != nil {
				result.View = "folder"
			}
		}
	}
	if result.Kind == "box" || result.Folder == box.RootFolderID.String() {
		result.Folder = ""
	}
	slices.Reverse(names)
	parts := append([]string{slugs[box.ID.ID]}, names...)
	if leaf != "" {
		parts = append(parts, leaf)
	}
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	result.Path = "/boxes/" + strings.Join(parts, "/")
	return result, nil
}
