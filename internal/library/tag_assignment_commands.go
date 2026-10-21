package library

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

func filterBoxTagsInView(vtx *store.ViewTx, tag domain.TagID, boxes []domain.BoxID, wantPresent bool) ([]domain.BoxID, error) {
	if _, ok, err := store.TagByID(vtx.Queryer(), tag); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("%s: %w", tag, ErrNoSuchTag)
	}
	var changed []domain.BoxID
	seen := map[domain.BoxID]bool{}
	err := func() error {
		for _, id := range boxes {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, ok, err := vtx.GetBox(id); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%s: %w", id, ErrNoSuchBox)
			}
			has, err := vtx.HasBoxTag(id, tag)
			if err != nil {
				return err
			}
			if has == wantPresent {
				changed = append(changed, id)
			}
		}
		return nil
	}()
	return changed, err
}

func filterFileTagsInView(vtx *store.ViewTx, tag domain.TagID, files []domain.FileID, wantPresent bool) ([]domain.FileID, []domain.BoxID, error) {
	if _, ok, err := store.TagByID(vtx.Queryer(), tag); err != nil {
		return nil, nil, err
	} else if !ok {
		return nil, nil, fmt.Errorf("%s: %w", tag, ErrNoSuchTag)
	}
	var (
		changed     []domain.FileID
		boxes       []domain.BoxID
		seen        = map[domain.FileID]bool{}
		seenBox     = map[domain.BoxID]bool{}
		boxOfFolder = map[domain.FolderID]domain.BoxID{}
	)
	err := func() error {
		for _, id := range files {
			if seen[id] {
				continue
			}
			seen[id] = true
			f, ok, err := vtx.GetFile(id)
			if err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%s: %w", id, ErrNoSuchFile)
			}
			has, err := vtx.HasFileTag(id, tag)
			if err != nil {
				return err
			}
			if has == wantPresent {
				changed = append(changed, id)
				// The box comes from the folder chain now, and is cached per
				// folder: a batch tag usually walks one box, and the walk is
				// the only cost that grew when files stopped carrying a box_id.
				box, cached := boxOfFolder[f.FolderID]
				if !cached {
					if box, _, err = store.FolderChain(vtx.Queryer(), f.FolderID); err != nil {
						return err
					}
					boxOfFolder[f.FolderID] = box
				}
				if !seenBox[box] {
					seenBox[box] = true
					boxes = append(boxes, box)
				}
			}
		}
		return nil
	}()
	return changed, boxes, err
}
