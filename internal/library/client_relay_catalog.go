package library

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// RelayHead fixes both the published root prefix and a separate relay snapshot.
// Sequence is a relay journal position and is never a root operation number.
type RelayHead struct {
	Root     SyncBoundary `json:"root"`
	Node     domain.ID    `json:"node"`
	Sequence int64        `json:"sequence"`
	Sum      string       `json:"sum"`
	Count    int          `json:"count"`
}

// RelayItem is a normalized intent and its outcome at a fixed relay boundary.
// Entries are exported in overlay order, including terminal decisions so that
// a disconnected child can learn deletions, replacements and final receipts.
type RelayItem struct {
	Proposal   SyncProposal `json:"proposal"`
	Confirmed  *SyncResult  `json:"confirmed,omitempty"`
	Conflict   string       `json:"conflict,omitempty"`
	Discarded  bool         `json:"discarded"`
	ReplacedBy string       `json:"replaced_by,omitempty"`
	Reason     string       `json:"reason,omitempty"`
}

type RelayPage struct {
	Head  RelayHead   `json:"head"`
	After int         `json:"after"`
	Next  int         `json:"next"`
	Done  bool        `json:"done"`
	Items []RelayItem `json:"items"`
}

type relayOrder struct {
	Node     domain.ID
	Position int
}
type relayTransfer struct {
	Head  RelayHead
	Items []RelayItem
}

func (p RelayPage) validate() error {
	h := p.Head
	if h.Node.IsZero() || h.Root.LibraryID.IsZero() || h.Sequence < 0 || h.Count < 0 || (h.Sequence == 0) != (h.Sum == "") || h.Root.OpID < 0 || (h.Root.OpID == 0) != (h.Root.Sum == "") || p.After < 0 || p.Next < p.After || p.Next > h.Count || p.Next-p.After != len(p.Items) || p.Done != (p.Next == h.Count) || (!p.Done && len(p.Items) == 0) || len(p.Items) > 256 {
		return ErrInvalidRequest
	}
	for _, item := range p.Items {
		if _, err := item.Proposal.digest(); err != nil {
			return err
		}
		if item.Proposal.Undo == nil {
			h, err := oplog.Lookup(oplog.Action(item.Proposal.Action))
			if err != nil {
				return err
			}
			if _, err := h.Targets(item.Proposal.Payload); err != nil {
				return err
			}
		}
		if len(item.Reason) > 4096 || len(item.Conflict) > 4096 || (item.Discarded && item.ReplacedBy != "") || ((item.Discarded || item.ReplacedBy != "") && item.Reason == "") {
			return ErrInvalidRequest
		}
		if r := item.Confirmed; r != nil {
			if item.Discarded || item.ReplacedBy != "" || item.Conflict != "" || r.OpID < 1 || r.AcceptedAt.IsZero() {
				return ErrInvalidRequest
			}
			if err := (pendingEvent{Version: 1, Target: item.Proposal.ID, Receipt: r}).validate("pending_ack"); err != nil {
				return err
			}
		}
	}
	return nil
}

func comparePending(a, b pendingEntry) int {
	if a.Upstream != nil && b.Upstream == nil {
		return -1
	}
	if a.Upstream == nil && b.Upstream != nil {
		return 1
	}
	if a.Upstream != nil && b.Upstream != nil {
		if a.Upstream.Position < b.Upstream.Position {
			return -1
		}
		if a.Upstream.Position > b.Upstream.Position {
			return 1
		}
	}
	if a.Sequence < b.Sequence {
		return -1
	}
	if a.Sequence > b.Sequence {
		return 1
	}
	return 0
}

func (c *Client) RelayHead(ctx context.Context) (RelayHead, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return RelayHead{}, err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return RelayHead{}, err
	}
	root, complete, err := c.boundary()
	if err != nil {
		return RelayHead{}, err
	}
	if !complete {
		return RelayHead{}, ErrSyncDependency
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return RelayHead{}, err
	}
	return RelayHead{Root: root, Node: settings.ID, Sequence: state.last, Sum: state.sum, Count: len(state.entries)}, nil
}

// RelayCatalog pages a fixed snapshot. A later edit never changes that snapshot.
// Byte limits are soft for a single large proposal, as in the root catalog.
func (c *Client) RelayCatalog(ctx context.Context, head RelayHead, after, limit, maxBytes int) (RelayPage, error) {
	page := RelayPage{Head: head, After: after, Next: after, Items: []RelayItem{}}
	if after < 0 || after > head.Count || limit < 1 || limit > 256 || maxBytes < 1 || maxBytes > 4<<20 {
		return page, ErrInvalidRequest
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return page, err
	}
	defer release()
	settings, err := c.settings()
	if err != nil {
		return page, err
	}
	root, complete, err := c.boundary()
	if err != nil {
		return page, err
	}
	if !complete || settings.ID != head.Node || root.LibraryID != head.Root.LibraryID || root.OpID < head.Root.OpID {
		return page, ErrSyncConflict
	}
	state, err := c.queue.readThrough(ctx, head.Sequence)
	if err != nil {
		return page, err
	}
	if state.last != head.Sequence || state.sum != head.Sum || len(state.entries) != head.Count {
		return page, ErrSyncConflict
	}
	// The root boundary is checked independently of the relay journal boundary.
	matched := head.Root.OpID == 0 && head.Root.Sum == ""
	receipts := map[string]SyncResult{}
	digests := map[string]string{}
	if err := c.catalog.Iter(func(line journal.Line) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID > head.Root.OpID {
			return nil
		}
		if line.ID == head.Root.OpID {
			matched = line.Sum == head.Root.Sum
		}
		if line.Sync != nil {
			at, err := journal.ParseTime(line.Sync.AcceptedAt)
			if err != nil {
				return err
			}
			receipts[line.Sync.ID] = SyncResult{OpID: line.ID, AcceptedAt: at}
			digests[line.Sync.ID] = line.Sync.Digest
		}
		return nil
	}); err != nil {
		return page, err
	}
	if !matched {
		return page, ErrSyncConflict
	}
	entries := slices.Clone(state.entries)
	slices.SortStableFunc(entries, comparePending)
	bytes := 0
	for i := after; i < len(entries); i++ {
		e := entries[i]
		item := RelayItem{Proposal: e.Proposal, Confirmed: e.Receipt, Conflict: e.Conflict, Discarded: e.Discarded, ReplacedBy: e.ReplacedBy, Reason: e.Reason}
		if receipt, ok := receipts[e.Proposal.ID]; ok {
			if digests[e.Proposal.ID] != e.Digest || e.Discarded || e.ReplacedBy != "" || (e.Receipt != nil && (e.Receipt.OpID != receipt.OpID || !e.Receipt.AcceptedAt.Equal(receipt.AcceptedAt))) {
				return page, ErrInvariantViolated
			}
			item.Confirmed = &receipt
			item.Conflict = ""
		}
		raw, err := json.Marshal(item)
		if err != nil {
			return page, err
		}
		if len(page.Items) >= limit || (len(page.Items) > 0 && bytes+len(raw) > maxBytes) {
			break
		}
		page.Items = append(page.Items, item)
		page.Next = i + 1
		bytes += len(raw)
	}
	page.Done = page.Next == head.Count
	return page, page.validate()
}

// RelayTransfer reports the durable page position, including an interrupted
// transfer. A complete head with Next==Count can be compared before polling.
func (c *Client) RelayTransfer(ctx context.Context) (*RelayHead, int, error) {
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer release()
	state, err := c.queue.read(ctx)
	if err != nil {
		return nil, 0, err
	}
	if t := state.relayTransfer; t != nil {
		return &t.Head, len(t.Items), nil
	}
	if h := state.relayHead; h != nil {
		return h, h.Count, nil
	}
	return nil, 0, nil
}

// ReceiveRelayPage durably stores each page but publishes the overlay only on
// completion. The complete state transition is validated before its final fsync.
func (c *Client) ReceiveRelayPage(ctx context.Context, page RelayPage) error {
	if ctx.Value(clientRequestContextKey{}) != nil || ctx.Value(clientViewContextKey{}) != nil {
		return ErrRequestScope
	}
	if err := page.validate(); err != nil {
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
	root, complete, err := c.boundary()
	if err != nil {
		return err
	}
	if !complete || page.Head.Node == settings.ID || page.Head.Root.LibraryID != settings.LibraryID || root.OpID < page.Head.Root.OpID {
		return ErrSyncConflict
	}
	matched := page.Head.Root.OpID == 0 && page.Head.Root.Sum == ""
	if err := c.catalog.Iter(func(line journal.Line) error {
		if line.ID == page.Head.Root.OpID {
			matched = line.Sum == page.Head.Root.Sum
		}
		return ctx.Err()
	}); err != nil {
		return err
	}
	if !matched {
		return ErrSyncConflict
	}
	state, err := c.queue.read(ctx)
	if err != nil {
		return err
	}
	unchanged := state.relayHead != nil && *state.relayHead == page.Head && state.relayTransfer == nil
	if err := state.receiveRelayPage(page, state.last+1); err != nil {
		return err
	}
	if page.Done && !unchanged {
		if err := c.validateRelayState(ctx, state, root); err != nil {
			return err
		}
	}
	if err := c.queue.appendLocked(ctx, "pending_relay_page", pendingEvent{Relay: &page}); err != nil {
		return err
	}
	if !page.Done {
		return nil
	}
	_, err = c.rebuildLocked(ctx)
	return err
}

func (s *pendingState) receiveRelayPage(page RelayPage, sequence int64) error {
	if !s.relaySource.IsZero() && s.relaySource != page.Head.Node {
		return ErrSyncConflict
	}
	if s.relayHead != nil {
		if page.Head.Sequence < s.relayHead.Sequence || page.Head.Root.OpID < s.relayHead.Root.OpID {
			return ErrSyncConflict
		}
		if page.Head.Sequence == s.relayHead.Sequence && (page.Head.Sum != s.relayHead.Sum || page.Head.Count != s.relayHead.Count) {
			return ErrSyncConflict
		}
		if page.Head == *s.relayHead && s.relayTransfer == nil {
			for i, item := range page.Items {
				a, _ := json.Marshal(item)
				b, _ := json.Marshal(s.relayItems[page.After+i])
				if string(a) != string(b) {
					return ErrSyncConflict
				}
			}
			return nil
		}
	}
	if s.relayTransfer == nil {
		if page.After != 0 {
			return ErrSyncDependency
		}
		s.relayTransfer = &relayTransfer{Head: page.Head}
	}
	transfer := s.relayTransfer
	if transfer.Head != page.Head || page.After > len(transfer.Items) {
		return ErrSyncConflict
	}
	for i, item := range page.Items {
		pos := page.After + i
		if pos < len(transfer.Items) {
			a, _ := json.Marshal(transfer.Items[pos])
			b, _ := json.Marshal(item)
			if string(a) != string(b) {
				return ErrSyncConflict
			}
		} else {
			transfer.Items = append(transfer.Items, item)
		}
	}
	s.relaySource = page.Head.Node
	if !page.Done {
		return nil
	}
	if len(transfer.Items) != page.Head.Count {
		return ErrSyncDependency
	}
	if err := s.mergeRelayItems(page.Head.Node, transfer.Items, sequence); err != nil {
		return err
	}
	head := page.Head
	s.relayHead = &head
	s.relayItems = slices.Clone(transfer.Items)
	s.relayTransfer = nil
	return nil
}

func (s *pendingState) mergeRelayItems(node domain.ID, items []RelayItem, sequence int64) error {
	seen := map[string]bool{}
	receipts := map[int64]string{}
	for pos, item := range items {
		p := item.Proposal
		digest, err := p.digest()
		if err != nil {
			return err
		}
		if seen[p.ID] {
			return ErrRequestConflict
		}
		seen[p.ID] = true
		i, exists := s.byID[p.ID]
		if !exists {
			i = len(s.entries)
			s.byID[p.ID] = i
			s.entries = append(s.entries, pendingEntry{Sequence: sequence, Proposal: p, Digest: digest})
		}
		old := s.entries[i]
		if old.Digest != digest {
			return ErrRequestConflict
		}
		if (old.Discarded && (!item.Discarded || old.Reason != item.Reason)) || (old.ReplacedBy != "" && (old.ReplacedBy != item.ReplacedBy || old.Reason != item.Reason)) {
			return ErrSyncConflict
		}
		if old.Receipt != nil && (item.Discarded || item.ReplacedBy != "" || (item.Confirmed != nil && (old.Receipt.OpID != item.Confirmed.OpID || !old.Receipt.AcceptedAt.Equal(item.Confirmed.AcceptedAt)))) {
			return ErrSyncConflict
		}
		old.Upstream = &relayOrder{Node: node, Position: pos}
		old.Sending = false
		old.Conflict = item.Conflict
		old.Discarded = item.Discarded
		old.ReplacedBy = item.ReplacedBy
		old.Reason = item.Reason
		if item.Confirmed != nil {
			old.Receipt = item.Confirmed
		}
		if old.Receipt != nil {
			old.Conflict = ""
		}
		s.entries[i] = old
	}
	for _, entry := range s.entries {
		if entry.Upstream != nil && !seen[entry.Proposal.ID] {
			return ErrSyncConflict
		}
		if entry.Receipt != nil {
			id := entry.Receipt.OpID
			if previous, ok := receipts[id]; ok && previous != entry.Proposal.ID {
				return ErrSyncConflict
			}
			receipts[id] = entry.Proposal.ID
		}
		if !seen[entry.Proposal.ID] {
			continue
		}
		refs := append([]string{}, entry.Proposal.Requires...)
		for _, ref := range []*SyncReference{entry.Proposal.Undo, entry.Proposal.Group} {
			if ref != nil {
				if (ref.OpID > 0) == (ref.LocalID != "") || ref.OpID < 0 {
					return ErrInvalidRequest
				}
				if ref.LocalID != "" {
					refs = append(refs, ref.LocalID)
				}
			}
		}
		for _, ref := range refs {
			if !seen[ref] || s.entries[s.byID[ref]].Upstream.Position >= entry.Upstream.Position {
				return ErrSyncDependency
			}
		}
		if entry.ReplacedBy != "" {
			i, ok := s.byID[entry.ReplacedBy]
			if !ok || !seen[entry.ReplacedBy] || s.entries[i].Upstream.Position <= entry.Upstream.Position {
				return ErrSyncDependency
			}
		}
	}
	return nil
}

// RelayRootCatalog exports only the published authoritative prefix. It never
// serializes overlay rows or leaks a partially downloaded root transfer.
func (c *Client) RelayRootCatalog(ctx context.Context, boundary SyncBoundary, after int64, limit, maxBytes int) (SyncCatalogPage, error) {
	page := SyncCatalogPage{Boundary: boundary, After: after, Next: after, Lines: []json.RawMessage{}}
	if after < 0 || boundary.OpID < after || limit < 1 || limit > 256 || maxBytes < 1 || maxBytes > 4<<20 || (boundary.OpID == 0) != (boundary.Sum == "") {
		return page, ErrInvalidRequest
	}
	release, err := c.lock.Acquire(ctx)
	if err != nil {
		return page, err
	}
	defer release()
	root, complete, err := c.boundary()
	if err != nil {
		return page, err
	}
	if !complete || root.LibraryID != boundary.LibraryID || root.OpID < boundary.OpID {
		return page, ErrSyncConflict
	}
	return exportSyncCatalog(ctx, c.catalog, page, limit, maxBytes)
}

// Validate the completed candidate in a disposable database, before persisting
// its publication. The real derived database must never run ahead of the log.
func (c *Client) validateRelayState(ctx context.Context, state pendingState, root SyncBoundary) error {
	receipts := map[int64]pendingEntry{}
	for _, entry := range state.entries {
		if entry.Receipt != nil && entry.Receipt.OpID <= root.OpID {
			receipts[entry.Receipt.OpID] = entry
		}
	}
	if err := c.catalog.Iter(func(line journal.Line) error {
		if entry, ok := receipts[line.ID]; ok {
			if line.Sync == nil || line.Sync.ID != entry.Proposal.ID || line.Sync.Digest != entry.Digest || line.Sync.AcceptedAt != journal.FormatTime(entry.Receipt.AcceptedAt) {
				return ErrSyncConflict
			}
			delete(receipts, line.ID)
		}
		return ctx.Err()
	}); err != nil {
		return err
	}
	if len(receipts) != 0 {
		return ErrSyncConflict
	}
	db, err := store.Open(":memory:")
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	_, err = rebuildClientPrefixInto(ctx, db, c.catalog, state, root.OpID, root.Sum)
	return err
}
