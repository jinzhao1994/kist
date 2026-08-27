package library

import (
	"fmt"
	"sort"
	"sync"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// TagTree is the whole tag tree, in memory.
//
// There is no materialized path column and no closure table (spec 1.6). A few
// thousand nodes fit comfortably, resolving descendants is a microsecond-scale
// walk, and — the part that actually pays — moving a subtree is a single
// parent_id write instead of a cascading rewrite.
//
// Consistency rests on library being the only writer and on there being one
// process (spec 1.6). A second process writing to the same library would leave
// this tree silently stale; that is open question B3, deliberately not solved
// in v0.1 because the CLI-only milestones cannot expose it.
type TagTree struct {
	mu     sync.RWMutex
	byID   map[domain.TagID]*tagNode
	byFold map[string]*tagNode
	roots  map[domain.TagID]*tagNode
}

type tagNode struct {
	tag         domain.Tag
	parent      *tagNode
	children    map[domain.TagID]*tagNode
	directCount int
}

// NewTagTree builds a tree from a flat list. counts maps a tag to how many
// things hang directly off it; it may be nil.
func NewTagTree(tags []domain.Tag, counts map[domain.TagID]int) (*TagTree, error) {
	t := &TagTree{
		byID:   make(map[domain.TagID]*tagNode, len(tags)),
		byFold: make(map[string]*tagNode, len(tags)),
		roots:  map[domain.TagID]*tagNode{},
	}
	for _, tag := range tags {
		if _, exists := t.byFold[tag.NameFold]; exists {
			return nil, fmt.Errorf("two tags are both named %q; the unique index on name_fold should have prevented this", tag.Name)
		}
		n := &tagNode{tag: tag, children: map[domain.TagID]*tagNode{}, directCount: counts[tag.ID]}
		t.byID[tag.ID] = n
		t.byFold[tag.NameFold] = n
	}
	// Second pass: every node exists, so parents resolve regardless of order.
	for _, n := range t.byID {
		if n.tag.ParentID == nil {
			t.roots[n.tag.ID] = n
			continue
		}
		parent, ok := t.byID[*n.tag.ParentID]
		if !ok {
			return nil, fmt.Errorf("tag %q points at parent %s, which does not exist", n.tag.Name, *n.tag.ParentID)
		}
		n.parent = parent
		parent.children[n.tag.ID] = n
	}
	return t, nil
}

// LoadTagTree reads the tree and its counts from the database.
func LoadTagTree(q store.Queryer) (*TagTree, error) {
	tags, err := store.ListTags(q)
	if err != nil {
		return nil, err
	}
	counts, err := store.TagDirectCounts(q)
	if err != nil {
		return nil, err
	}
	return NewTagTree(tags, counts)
}

func (t *TagTree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.byID)
}

func (t *TagTree) ByID(id domain.TagID) (domain.Tag, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return domain.Tag{}, false
	}
	return n.tag, true
}

// ByName resolves a name through the same fold function the unique index uses,
// so "Passport" and "passport" find the same tag (spec 1.6, 4.7).
func (t *TagTree) ByName(name string) (domain.Tag, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byFold[domain.Fold(name)]
	if !ok {
		return domain.Tag{}, false
	}
	return n.tag, true
}

func (t *TagTree) Roots() []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return sortedTags(t.roots)
}

func (t *TagTree) Children(id domain.TagID) []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	return sortedTags(n.children)
}

// Descendants returns the id set for a tag filter: the node itself plus
// everything under it (spec 4.3).
func (t *TagTree) Descendants(id domain.TagID) []domain.TagID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	var out []domain.TagID
	var walk func(*tagNode)
	walk = func(n *tagNode) {
		out = append(out, n.tag.ID)
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(n)
	return out
}

// Path returns the chain from the root down to this tag, inclusive.
func (t *TagTree) Path(id domain.TagID) []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	var reversed []domain.Tag
	for cur := n; cur != nil; cur = cur.parent {
		reversed = append(reversed, cur.tag)
	}
	out := make([]domain.Tag, len(reversed))
	for i, tag := range reversed {
		out[len(reversed)-1-i] = tag
	}
	return out
}

// WouldCycle reports whether making newParent the parent of node would close a
// loop. A loop makes Descendants recurse until the stack runs out, so this is
// checked when the command is built — Apply is not allowed to decide things.
func (t *TagTree) WouldCycle(node, newParent domain.TagID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for cur := t.byID[newParent]; cur != nil; cur = cur.parent {
		if cur.tag.ID == node {
			return true
		}
	}
	return false
}

// Count is the subtree total: what the tag selector shows next to a node.
func (t *TagTree) Count(id domain.TagID) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return 0
	}
	total := 0
	var walk func(*tagNode)
	walk = func(n *tagNode) {
		total += n.directCount
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(n)
	return total
}

func (t *TagTree) SetDirectCount(id domain.TagID, n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if node, ok := t.byID[id]; ok {
		node.directCount = n
	}
}

func (t *TagTree) AddDirectCount(id domain.TagID, delta int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if node, ok := t.byID[id]; ok {
		node.directCount += delta
	}
}

// Insert adds a node. It mirrors what the database just did, so a violated
// invariant here means the two have drifted.
func (t *TagTree) Insert(tag domain.Tag) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.byID[tag.ID]; exists {
		return fmt.Errorf("tag %s is already in the tree", tag.ID)
	}
	if _, exists := t.byFold[tag.NameFold]; exists {
		return fmt.Errorf("tag name %q is already taken", tag.Name)
	}
	n := &tagNode{tag: tag, children: map[domain.TagID]*tagNode{}}
	t.byID[tag.ID] = n
	t.byFold[tag.NameFold] = n
	return t.attach(n)
}

// Update handles both rename and move: the caller passes the new state of the
// node and the tree rearranges itself.
func (t *TagTree) Update(tag domain.Tag) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.byID[tag.ID]
	if !ok {
		return fmt.Errorf("tag %s is not in the tree", tag.ID)
	}
	if other, exists := t.byFold[tag.NameFold]; exists && other != n {
		return fmt.Errorf("tag name %q is already taken", tag.Name)
	}
	delete(t.byFold, n.tag.NameFold)
	t.detach(n)

	n.tag = tag
	t.byFold[tag.NameFold] = n
	return t.attach(n)
}

func (t *TagTree) Remove(id domain.TagID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.byID[id]
	if !ok {
		return fmt.Errorf("tag %s is not in the tree", id)
	}
	if len(n.children) > 0 {
		return fmt.Errorf("tag %q still has children", n.tag.Name)
	}
	t.detach(n)
	delete(t.byID, id)
	delete(t.byFold, n.tag.NameFold)
	return nil
}

// attach and detach assume the lock is held.
func (t *TagTree) attach(n *tagNode) error {
	if n.tag.ParentID == nil {
		n.parent = nil
		t.roots[n.tag.ID] = n
		return nil
	}
	parent, ok := t.byID[*n.tag.ParentID]
	if !ok {
		return fmt.Errorf("parent tag %s does not exist", *n.tag.ParentID)
	}
	n.parent = parent
	parent.children[n.tag.ID] = n
	return nil
}

func (t *TagTree) detach(n *tagNode) {
	if n.parent != nil {
		delete(n.parent.children, n.tag.ID)
	}
	delete(t.roots, n.tag.ID)
	n.parent = nil
}

// sortedTags returns nodes in byte order of their name (spec 4.8), so listings
// are stable and identical on every platform.
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
