package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/journal"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

// pendingQueue owns the client's durable intents and decisions. Its journal
// sequence is local bookkeeping, never an authoritative NAS operation ID.
// No state is retained between calls: another process may have appended events.
type pendingQueue struct {
	log  *journal.Dir
	lock *lockfile.Locker
	now  func() time.Time
}

type pendingEntry struct {
	Sequence   int64
	Proposal   SyncProposal
	Digest     string
	Receipt    *SyncResult
	Forwarded  *RelayReceipt
	Upstream   *relayOrder
	Custody    *RelayReceipt
	Sending    bool
	Conflict   string
	Discarded  bool
	ReplacedBy string
	Reason     string
}

func (e pendingEntry) active() bool {
	return e.Receipt == nil && !e.Discarded && e.ReplacedBy == ""
}

type pendingEvent struct {
	Relay    *RelayPage    `json:"relay,omitempty"`
	Version  int           `json:"version"`
	Target   string        `json:"target,omitempty"`
	Proposal *SyncProposal `json:"proposal,omitempty"`
	Receipt  *SyncResult   `json:"receipt,omitempty"`
	Custody  *RelayReceipt `json:"custody,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Request  *Request      `json:"request,omitempty"`
}

func (e pendingEvent) validate(action string) error {
	if e.Version != 1 && e.Version != 2 && e.Version != 3 && e.Version != 4 {
		return ErrSyncVersion
	}
	if (e.Version == 2) != (e.Request != nil) || (e.Version == 3) != (e.Custody != nil) || (e.Version == 4) != (e.Relay != nil) {
		return ErrInvalidRequest
	}
	if e.Request != nil && (!e.Request.Valid() || action != "pending_enqueue" && action != "pending_request") {
		return ErrInvalidRequest
	}
	if len(e.Reason) > 4096 {
		return ErrInvalidRequest
	}
	switch action {
	case "pending_relay_page":
		if e.Relay == nil || e.Target != "" || e.Proposal != nil || e.Receipt != nil || e.Request != nil || e.Custody != nil || e.Reason != "" {
			return ErrInvalidRequest
		}
		return e.Relay.validate()
	case "pending_custody", "pending_relay_ack":
		if e.Target == "" || e.Custody == nil || e.Proposal != nil || e.Receipt != nil || e.Request != nil || e.Reason != "" || !e.Custody.valid() || e.Target != e.Custody.ProposalID {
			return ErrInvalidRequest
		}
	case "pending_request":
		if e.Request == nil || e.Proposal != nil || e.Target != "" || e.Receipt != nil || e.Reason != "" {
			return ErrInvalidRequest
		}
	case "pending_enqueue":
		if e.Proposal == nil || e.Target != "" || e.Receipt != nil || e.Reason != "" {
			return ErrInvalidRequest
		}
	case "pending_send":
		if e.Target == "" || e.Proposal != nil || e.Receipt != nil || e.Reason != "" {
			return ErrInvalidRequest
		}
	case "pending_ack":
		if e.Target == "" || e.Receipt == nil || e.Proposal != nil || e.Reason != "" || e.Receipt.OpID < 1 || e.Receipt.AcceptedAt.IsZero() || !time.Unix(0, e.Receipt.AcceptedAt.UnixNano()).Equal(e.Receipt.AcceptedAt) {
			return ErrInvalidRequest
		}
	case "pending_discard", "pending_resolve", "pending_reject":
		if e.Target == "" || e.Receipt != nil || e.Reason == "" || (action == "pending_resolve") != (e.Proposal != nil) {
			return ErrInvalidRequest
		}
	default:
		return ErrSyncVersion
	}
	if (e.Custody != nil && action != "pending_custody" && action != "pending_relay_ack") || (e.Relay != nil && action != "pending_relay_page") {
		return ErrInvalidRequest
	}
	if e.Proposal != nil {
		_, err := e.Proposal.digest()
		return err
	}
	return nil
}

type pendingState struct {
	sum           string
	relaySource   domain.ID
	relayHead     *RelayHead
	relayItems    []RelayItem
	relayTransfer *relayTransfer
	last          int64
	entries       []pendingEntry
	byID          map[string]int
	requests      map[string]pendingRequest
}

type pendingRequest struct {
	Digest  string
	LocalID string
}

// openPendingQueue opens a dedicated client directory, not a NAS ops directory.
// The caller creates and durably installs the surrounding client layout first.
func openPendingQueue(root string) (*pendingQueue, error) {
	d, err := journal.NewDir(root)
	if err != nil {
		return nil, err
	}
	return &pendingQueue{log: d, lock: lockfile.New(filepath.Join(root, "pending.lock")), now: time.Now}, nil
}

func (q *pendingQueue) read(ctx context.Context) (pendingState, error) {
	return q.readThrough(ctx, math.MaxInt64)
}

func (q *pendingQueue) readThrough(ctx context.Context, through int64) (pendingState, error) {
	s := pendingState{byID: map[string]int{}}
	if _, err := q.log.RecoverTail(); err != nil {
		return s, err
	}
	end := errors.New("pending prefix reached")
	err := q.log.Iter(func(line journal.Line) error {
		if line.ID > through {
			return end
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if line.ID != s.last+1 || line.Sync != nil || line.Request != nil || line.RevertsID != nil || line.RootOpID != nil || line.Origin != "user" {
			return fmt.Errorf("%w: invalid pending envelope", journal.ErrCorrupt)
		}
		if _, err := line.Time(); err != nil {
			return err
		}
		var event pendingEvent
		if err := json.Unmarshal(line.Payload, &event); err != nil {
			return fmt.Errorf("%w: pending payload: %v", journal.ErrCorrupt, err)
		}
		if err := s.apply(line.Action, event, line.ID); err != nil {
			if errors.Is(err, ErrSyncVersion) {
				return err
			}
			return fmt.Errorf("%w: pending event %d: %v", journal.ErrCorrupt, line.ID, err)
		}
		s.last, s.sum = line.ID, line.Sum
		return nil
	})
	if errors.Is(err, end) {
		err = nil
	}
	return s, err
}

func (s *pendingState) add(p SyncProposal, sequence int64) error {
	digest, err := p.digest()
	if err != nil {
		return err
	}
	if _, exists := s.byID[p.ID]; exists {
		return ErrRequestConflict
	}
	// Local references must already have a durable identity. Replaced or
	// discarded identities are not silently redirected to a different intent.
	refs := append([]string{}, p.Requires...)
	for _, ref := range []*SyncReference{p.Undo, p.Group} {
		if ref != nil {
			if (ref.OpID > 0) == (ref.LocalID != "") || ref.OpID < 0 {
				return ErrInvalidRequest
			}
			if ref.LocalID != "" {
				refs = append(refs, ref.LocalID)
			}
		}
	}
	for _, id := range refs {
		i, exists := s.byID[id]
		if !exists || s.entries[i].Discarded || s.entries[i].ReplacedBy != "" {
			return ErrSyncDependency
		}
	}
	s.byID[p.ID] = len(s.entries)
	s.entries = append(s.entries, pendingEntry{Sequence: sequence, Proposal: p, Digest: digest})
	return nil
}

func (s *pendingState) apply(action string, e pendingEvent, sequence int64) error {
	if err := e.validate(action); err != nil {
		return err
	}
	if action == "pending_relay_page" {
		return s.receiveRelayPage(*e.Relay, sequence)
	}
	if e.Request != nil {
		if _, exists := s.requests[e.Request.ID]; exists {
			return ErrRequestConflict
		}
	}
	if action == "pending_enqueue" {
		if err := s.add(*e.Proposal, sequence); err != nil {
			return err
		}
		if e.Request != nil {
			s.recordRequest(*e.Request, e.Proposal.ID)
		}
		return nil
	}
	if action == "pending_request" {
		s.recordRequest(*e.Request, "")
		return nil
	}
	i, exists := s.byID[e.Target]
	if !exists || (!s.entries[i].active() && action != "pending_custody") {
		return ErrSyncConflict
	}
	switch action {
	case "pending_relay_ack":
		if !s.entries[i].Sending || s.entries[i].Digest != e.Custody.Digest || s.relaySource != e.Custody.RelayID || s.relayHead == nil || s.relayHead.Root.LibraryID != e.Custody.LibraryID {
			return ErrSyncConflict
		}
		s.entries[i].Forwarded = e.Custody
		s.entries[i].Sending = false
	case "pending_custody":
		if s.entries[i].Custody != nil || e.Custody.Digest != s.entries[i].Digest {
			return ErrRequestConflict
		}
		s.entries[i].Custody = e.Custody
	case "pending_send":
		next, err := s.next()
		if s.relayHead != nil {
			next, err = s.nextRelay()
		}
		if err != nil {
			return err
		}
		if next == nil || next.Proposal.ID != e.Target {
			return ErrSyncDependency
		}
		s.entries[i].Sending = true
	case "pending_reject":
		if !s.entries[i].Sending {
			return ErrSyncConflict
		}
		s.entries[i].Sending = false
		s.entries[i].Conflict = e.Reason
	case "pending_ack":
		if !s.entries[i].Sending {
			return ErrSyncConflict
		}
		for _, entry := range s.entries {
			if entry.Receipt != nil && entry.Receipt.OpID == e.Receipt.OpID {
				return ErrSyncConflict
			}
		}
		s.entries[i].Receipt = e.Receipt
		s.entries[i].Sending = false
	case "pending_discard", "pending_resolve":
		if s.entries[i].Sending || s.entries[i].Upstream != nil || s.entries[i].Forwarded != nil {
			return ErrSyncConflict
		}
		if e.Proposal != nil {
			// Add before changing the original so a failed validation leaves it
			// intact. A replacement must not depend on the intent it supersedes.
			for _, id := range e.Proposal.Requires {
				if id == e.Target {
					return ErrSyncDependency
				}
			}
			for _, ref := range []*SyncReference{e.Proposal.Undo, e.Proposal.Group} {
				if ref != nil && ref.LocalID == e.Target {
					return ErrSyncDependency
				}
			}
			if err := s.add(*e.Proposal, s.entries[i].Sequence); err != nil {
				return err
			}
			s.entries[i].ReplacedBy = e.Proposal.ID
		} else {
			s.entries[i].Discarded = true
		}
		s.entries[i].Reason = e.Reason
	default:
		return ErrSyncVersion
	}
	return nil
}

func (s *pendingState) recordRequest(request Request, localID string) {
	if s.requests == nil {
		s.requests = map[string]pendingRequest{}
	}
	s.requests[request.ID] = pendingRequest{Digest: request.Digest, LocalID: localID}
}

// next never skips a conflict or a broken dependency to send a later intent.
// Resolutions retain their original logical position despite being appended.
func (s *pendingState) next() (*pendingEntry, error) {
	var next *pendingEntry
	for i := range s.entries {
		e := &s.entries[i]
		if e.active() && (next == nil || comparePending(*e, *next) < 0) {
			next = e
		}
	}
	if next == nil {
		return nil, nil
	}
	if next.Upstream != nil || next.Forwarded != nil {
		return next, ErrSyncDependency
	}
	if next.Conflict != "" {
		return next, ErrSyncConflict
	}
	refs := append([]string{}, next.Proposal.Requires...)
	for _, ref := range []*SyncReference{next.Proposal.Undo, next.Proposal.Group} {
		if ref != nil && ref.LocalID != "" {
			refs = append(refs, ref.LocalID)
		}
	}
	for _, id := range refs {
		i, found := s.byID[id]
		if !found || s.entries[i].Receipt == nil {
			return next, ErrSyncDependency
		}
	}
	return next, nil
}

func (q *pendingQueue) snapshot(ctx context.Context) ([]pendingEntry, error) {
	release, err := q.lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	s, err := q.read(ctx)
	return s.entries, err
}

// append performs validation and persistence under the same cross-process lock.
// An append error is an uncertain outcome; retries compare the durable record.
func (q *pendingQueue) append(ctx context.Context, action string, event pendingEvent) error {
	release, err := q.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return q.appendLocked(ctx, action, event)
}

// appendLocked is used when the owning client already holds the same lock
// across view validation, durable enqueue and materialization.
func (q *pendingQueue) appendLocked(ctx context.Context, action string, event pendingEvent) error {
	s, err := q.read(ctx)
	if err != nil {
		return err
	}
	event.Version = 1
	if event.Relay != nil {
		event.Version = 4
	}
	if event.Request != nil {
		event.Version = 2
	}
	if event.Custody != nil {
		event.Version = 3
	}
	if err := event.validate(action); err != nil {
		return err
	}
	if event.Request != nil {
		if old, found := s.requests[event.Request.ID]; found {
			localID := ""
			if event.Proposal != nil {
				localID = event.Proposal.ID
			}
			if old.Digest != event.Request.Digest || old.LocalID != localID {
				return ErrRequestConflict
			}
			if event.Proposal != nil {
				digest, err := event.Proposal.digest()
				if err != nil {
					return err
				}
				if digest != s.entries[s.byID[localID]].Digest {
					return ErrRequestConflict
				}
			}
			return nil
		}
	}
	if action == "pending_enqueue" && event.Proposal != nil {
		if i, found := s.byID[event.Proposal.ID]; found {
			if event.Request != nil {
				return ErrRequestConflict
			}
			digest, err := event.Proposal.digest()
			if err != nil {
				return err
			}
			if digest == s.entries[i].Digest {
				return nil
			}
			return ErrRequestConflict
		}
	}
	if i, found := s.byID[event.Target]; found {
		old := s.entries[i]
		if action == "pending_relay_ack" && old.Forwarded != nil && event.Custody != nil && *old.Forwarded == *event.Custody {
			return nil
		}
		if action == "pending_custody" && old.Custody != nil && event.Custody != nil && *old.Custody == *event.Custody {
			return nil
		}
		if action == "pending_send" && old.Sending {
			return nil
		}
		if action == "pending_reject" && old.active() && !old.Sending && old.Conflict == event.Reason {
			return nil
		}
		if action == "pending_ack" && old.Receipt != nil && event.Receipt != nil && old.Receipt.OpID == event.Receipt.OpID && old.Receipt.AcceptedAt.Equal(event.Receipt.AcceptedAt) {
			return nil
		}
		if action == "pending_discard" && old.Discarded && old.Reason == event.Reason {
			return nil
		}
		if action == "pending_resolve" && event.Proposal != nil && old.ReplacedBy == event.Proposal.ID && old.Reason == event.Reason {
			digest, err := event.Proposal.digest()
			if err == nil && digest == s.entries[s.byID[old.ReplacedBy]].Digest {
				return nil
			}
			return ErrRequestConflict
		}
	}
	relayRetry := false
	if p := event.Relay; p != nil {
		relayRetry = (s.relayHead != nil && *s.relayHead == p.Head && s.relayTransfer == nil) || (s.relayTransfer != nil && s.relayTransfer.Head == p.Head && p.Next <= len(s.relayTransfer.Items))
	}
	if err := s.apply(action, event, s.last+1); err != nil {
		return err
	}
	if relayRetry {
		return nil
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return q.log.Append(journal.Line{ID: s.last + 1, TS: journal.FormatTime(q.now()), Action: action, Origin: "user", Payload: raw})
}
