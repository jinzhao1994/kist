package library

import (
	"fmt"
	"sort"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// TagTree is the whole tag tree, read at one moment in time.
//
// It exists for the one caller that genuinely wants every tag: printing or
// rendering the tree itself. Everything else asks the database about the tags
// it actually needs — store.TagByName to resolve one, store.TagDescendants to
// expand a filter to a subtree — because reading a few thousand rows to answer
// a question about five of them is the wrong shape.
//
// There is no materialized path column and no closure table (spec 1.6). The
// payoff is that moving a subtree is a single parent_id write instead of a
// cascading rewrite; the cost is that descendants are a recursive query.
//
// It is an immutable snapshot, and nothing holds one beyond the operation that
// loaded it. A copy kept on Library would be this process's opinion of state
// every other kist process can also write, and it would be wrong from the
// first write anybody else made (spec 2.7). That is why it has no lock: after
// NewTagTree returns, nothing changes it.
type TagTree struct {
	byID  map[domain.TagID]*tagNode
	roots map[domain.TagID]*tagNode
}

type tagNode struct {
	tag      domain.Tag
	children map[domain.TagID]*tagNode
}

// NewTagTree builds a tree from a flat list.
func NewTagTree(tags []domain.Tag) (*TagTree, error) {
	t := &TagTree{
		byID:  make(map[domain.TagID]*tagNode, len(tags)),
		roots: map[domain.TagID]*tagNode{},
	}
	byFold := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if byFold[tag.NameFold] {
			return nil, fmt.Errorf("two tags are both named %q; the unique index on name_fold should have prevented this: %w", tag.Name, ErrInvariantViolated)
		}
		byFold[tag.NameFold] = true
		t.byID[tag.ID] = &tagNode{tag: tag, children: map[domain.TagID]*tagNode{}}
	}
	// Second pass: every node exists, so parents resolve regardless of order.
	for _, n := range t.byID {
		if n.tag.ParentID == nil {
			t.roots[n.tag.ID] = n
			continue
		}
		parent, ok := t.byID[*n.tag.ParentID]
		if !ok {
			return nil, fmt.Errorf("tag %q points at parent %s, which does not exist: %w", n.tag.Name, *n.tag.ParentID, ErrInvariantViolated)
		}
		parent.children[n.tag.ID] = n
	}
	return t, nil
}

// LoadTagTree reads the tree from the database.
func LoadTagTree(q store.Queryer) (*TagTree, error) {
	tags, err := store.ListTags(q)
	if err != nil {
		return nil, err
	}
	return NewTagTree(tags)
}

func (t *TagTree) Roots() []domain.Tag { return sortedTags(t.roots) }

func (t *TagTree) Children(id domain.TagID) []domain.Tag {
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	return sortedTags(n.children)
}

// Totals rolls direct counts up the tree: every tag mapped to how many things
// carry it or anything under it, which is what the tag selector shows next to
// a node (spec 1.6).
//
// One post-order walk does the whole tree. Asking node by node would be
// quadratic, and the caller that wants one number invariably wants all of them.
func (t *TagTree) Totals(direct map[domain.TagID]int) map[domain.TagID]int {
	out := make(map[domain.TagID]int, len(t.byID))
	var walk func(*tagNode) int
	walk = func(n *tagNode) int {
		total := direct[n.tag.ID]
		for _, c := range n.children {
			total += walk(c)
		}
		out[n.tag.ID] = total
		return total
	}
	for _, n := range t.roots {
		walk(n)
	}
	return out
}

func sortedTags(m map[domain.TagID]*tagNode) []domain.Tag {
	out := make([]domain.Tag, 0, len(m))
	for _, n := range m {
		out = append(out, n.tag)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}
