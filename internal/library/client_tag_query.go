package library

import (
	"context"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/store"
)

// ResolveTag resolves a name or ID against the current local editing view.
func (c *Client) ResolveTag(ctx context.Context, ref string) (domain.Tag, error) {
	var tag domain.Tag
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var err error
		tag, err = resolveTag(tx.Queryer(), ref)
		return err
	})
	return tag, err
}

func (c *Client) TagCondition(ctx context.Context, refs []string) (query.TagExpr, error) {
	return c.tagCondition(ctx, refs, false)
}

func (c *Client) ExcludeTags(ctx context.Context, refs []string) (query.TagExpr, error) {
	return c.tagCondition(ctx, refs, true)
}

// Resolve every subtree from one snapshot so concurrent edits cannot mix trees.
func (c *Client) tagCondition(ctx context.Context, refs []string, exclude bool) (query.TagExpr, error) {
	var result query.TagExpr
	err := c.readView(ctx, func(tx *store.ViewTx) error {
		var terms query.TagAll
		for _, ref := range refs {
			tag, err := resolveTag(tx.Queryer(), ref)
			if err != nil {
				return err
			}
			ids, err := store.TagDescendants(tx.Queryer(), []domain.TagID{tag.ID})
			if err != nil {
				return err
			}
			terms = append(terms, query.TagAny(ids))
		}
		if len(terms) == 0 {
			return nil
		}
		if exclude {
			result = query.TagNot{Expr: query.TagAnyOf(terms)}
		} else {
			result = terms
		}
		return nil
	})
	return result, err
}
