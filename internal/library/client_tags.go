package library

import (
	"context"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/store"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
)

// CreateTag validates and stages a new tag in the local editing view.
func (c *Client) CreateTag(ctx context.Context, name string, parent *domain.TagID) (domain.Tag, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return domain.Tag{}, err
	}
	defer release()
	p, tag, err := createTagPayload(c.db, name, parent)
	if err != nil {
		return domain.Tag{}, err
	}
	if err := c.commandWriteLocked(ctx, scope, oplog.ActionCreateTag, p); err != nil {
		return domain.Tag{}, err
	}
	return tag, nil
}

func (c *Client) RenameTag(ctx context.Context, id domain.TagID, name string) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	p, err := renameTagPayload(c.db, id, name)
	if err != nil || p == nil {
		return err
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionRenameTag, p)
}

func (c *Client) MoveTag(ctx context.Context, id domain.TagID, parent *domain.TagID) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	p, err := moveTagPayload(c.db, id, parent)
	if err != nil || p == nil {
		return err
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionMoveTag, p)
}

func (c *Client) DeleteTag(ctx context.Context, id domain.TagID) error {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	p, err := deleteTagPayload(c.db, id)
	if err != nil {
		return err
	}
	return c.commandWriteLocked(ctx, scope, oplog.ActionDeleteTag, p)
}

func (c *Client) TagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	return c.changeBoxTags(ctx, tag, boxes, false)
}
func (c *Client) UntagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	return c.changeBoxTags(ctx, tag, boxes, true)
}
func (c *Client) TagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error) {
	return c.changeFileTags(ctx, tag, files, false)
}
func (c *Client) UntagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error) {
	return c.changeFileTags(ctx, tag, files, true)
}

func (c *Client) changeBoxTags(ctx context.Context, tag domain.TagID, boxes []domain.BoxID, remove bool) (int, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	changed, err := filterBoxTagsInView(store.NewViewTx(tx, time.Time{}), tag, boxes, remove)
	rollbackErr := tx.Rollback()
	if err != nil {
		return 0, err
	}
	if rollbackErr != nil {
		return 0, rollbackErr
	}
	if len(changed) == 0 {
		return 0, nil
	}
	if remove {
		err = c.commandWriteLocked(ctx, scope, oplog.ActionUntagBoxes, oplog.UntagBoxesPayload{TagID: tag, BoxIDs: changed})
	} else {
		err = c.commandWriteLocked(ctx, scope, oplog.ActionTagBoxes, oplog.TagBoxesPayload{TagID: tag, BoxIDs: changed})
	}
	if err != nil {
		return 0, err
	}
	return len(changed), nil
}

func (c *Client) changeFileTags(ctx context.Context, tag domain.TagID, files []domain.FileID, remove bool) (int, error) {
	scope, release, err := c.commandLock(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	changed, boxes, err := filterFileTagsInView(store.NewViewTx(tx, time.Time{}), tag, files, remove)
	rollbackErr := tx.Rollback()
	if err != nil {
		return 0, err
	}
	if rollbackErr != nil {
		return 0, rollbackErr
	}
	if len(changed) == 0 {
		return 0, nil
	}
	if remove {
		err = c.commandWriteLocked(ctx, scope, oplog.ActionUntagFiles, oplog.UntagFilesPayload{TagID: tag, FileIDs: changed, BoxIDs: boxes, Source: extract.UserSource})
	} else {
		err = c.commandWriteLocked(ctx, scope, oplog.ActionTagFiles, oplog.TagFilesPayload{TagID: tag, FileIDs: changed, BoxIDs: boxes, Source: extract.UserSource})
	}
	if err != nil {
		return 0, err
	}
	return len(changed), nil
}
