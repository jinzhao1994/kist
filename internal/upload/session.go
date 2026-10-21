// Package upload manages discardable, durable upload staging. It has no access
// to the archive: only library may turn verified staged files into an op.
package upload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/lockfile"
)

const (
	Version         = 1
	ProtocolVersion = 2
	ChunkBytes      = 8 << 20
	MaxSessions     = 8
	IdleTTL         = 7 * 24 * time.Hour
	maxRecordBytes  = 8 << 20
)

// Problem distinguishes recoverable protocol failures from filesystem errors.
// Offset is the reliable server position when Kind is "offset".
type Problem struct {
	Kind   string
	Offset int64
}

func (e *Problem) Error() string { return "upload session: " + e.Kind }
func problem(kind string) error  { return &Problem{Kind: kind} }

// File is the same identity tuple used by checked multipart uploads.
type File struct {
	Path         string `json:"path"`
	LastModified int64  `json:"last_modified"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
}

// Manifest is immutable for the lifetime of a session. Target identifies the
// target's structural snapshot, computed and rechecked by the archive layer.
type Manifest struct {
	DestinationToken string `json:"destination_token,omitempty"`
	ConflictMode     string `json:"conflict_mode,omitempty"`
	ConflictToken    string `json:"conflict_token,omitempty"`
	Protocol         int    `json:"protocol,omitempty"`
	Box              string `json:"box"`
	Folder           string `json:"folder"`
	Target           string `json:"target"`
	Files            []File `json:"files"`
}

// State is a snapshot, never a mutable in-process progress cache.
type State struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Manifest Manifest  `json:"manifest"`
	Offsets  []int64   `json:"offsets"`
	Digests  []string  `json:"digests,omitempty"`
	Expires  time.Time `json:"expires"`
	Canceled bool      `json:"canceled,omitempty"`
	OpID     int64     `json:"op_id,omitempty"`
	LocalID  string    `json:"local_id,omitempty"`
	Noop     bool      `json:"noop,omitempty"`
}

// Completed distinguishes durable receipts from an unfinished upload.
func (s State) Completed() bool { return s.OpID != 0 || s.LocalID != "" || s.Noop }

func (s State) validReceipt() bool {
	if s.OpID < 0 {
		return false
	}
	count := 0
	if s.OpID != 0 {
		count++
	}
	if s.Noop {
		count++
	}
	if s.LocalID != "" {
		id, err := domain.ParseID(s.LocalID)
		if err != nil || id.String() != s.LocalID {
			return false
		}
		count++
	}
	return count <= 1
}

// Store is safe to reopen in another process. Limits apply to new sessions;
// lowering a limit does not make an existing record unreadable.
type Store struct {
	path     string
	maxBytes int64
	maxFiles int
	now      func() time.Time
	// fault is used by tests at durability boundaries; it is immutable in use.
	fault   func(string) error
	writeAt func(*os.File, []byte, int64) (int, error)
}

func New(path string, maxBytes int64, maxFiles int) (*Store, error) {
	if maxBytes <= 0 || maxBytes > (1<<63-1)/2 || maxFiles <= 0 {
		return nil, problem("invalid_limits")
	}
	if err := os.MkdirAll(filepath.Join(path, "locks"), 0700); err != nil {
		return nil, err
	}
	if err := syncDirectory(path); err != nil {
		return nil, err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return &Store{path: path, maxBytes: maxBytes, maxFiles: maxFiles, now: time.Now}, nil
}

// ID binds a client-generated creation key to a filesystem-safe identifier.
func ID(key string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(key))) }

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && hex.EncodeToString(b) == s
}

func manifestBytes(m Manifest) (int64, error) {
	if (m.Protocol != 0 && m.Protocol != 2) || m.Box == "" || m.Folder == "" || !validDigest(m.Target) || len(m.Files) == 0 {
		return 0, problem("invalid_manifest")
	}
	var total int64
	seen := map[string]bool{}
	for _, f := range m.Files {
		if !fs.ValidPath(f.Path) || f.Path == "." || len(f.Path) > 4096 || strings.ContainsAny(f.Path, "\\\x00") || f.Size < 0 || ((m.Protocol != 2 && !validDigest(f.SHA256)) || (m.Protocol == 2 && f.SHA256 != "")) || f.LastModified < -9223372036854 || f.LastModified > 9223372036854 {
			return 0, problem("invalid_manifest")
		}
		for _, part := range strings.Split(f.Path, "/") {
			if err := domain.ValidateName(part); err != nil {
				return 0, err
			}
		}
		fold := domain.Fold(f.Path)
		if seen[fold] || f.Size > 1<<63-1-total {
			return 0, problem("invalid_manifest")
		}
		seen[fold] = true
		total += f.Size
	}
	return total, nil
}

func (s *Store) checkpoint(point string) error {
	if s.fault != nil {
		return s.fault(point)
	}
	return nil
}

func (s *Store) lock(ctx context.Context, id string) (func(), error) {
	if !validDigest(id) {
		return nil, problem("invalid_id")
	}
	// Fixed stripes bound permanent lock files. They are never unlinked, even
	// when all sessions have expired; a reused inode would break exclusion.
	return lockfile.New(filepath.Join(s.path, "locks", id[:2]+".lock")).Acquire(ctx)
}

// Create returns the existing session for identical input. The caller must
// validate the creation key and check durable archive receipts before calling.
func (s *Store) Create(ctx context.Context, key string, manifest Manifest) (State, error) {
	var zero State
	total, err := manifestBytes(manifest)
	if err != nil {
		return zero, err
	}
	if len(key) == 0 || len(key) > 128 || total > s.maxBytes || len(manifest.Files) > s.maxFiles {
		return zero, problem("invalid_manifest")
	}
	id := ID(key)
	release, err := lockfile.New(filepath.Join(s.path, "locks", "registry.lock")).Acquire(ctx)
	if err != nil {
		return zero, err
	}
	defer release()
	if err := s.cleanCreates(); err != nil {
		return zero, err
	}
	if err := s.sweep(ctx); err != nil {
		return zero, err
	}
	unlock, err := s.lock(ctx, id)
	if err != nil {
		return zero, err
	}
	defer unlock()
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return zero, err
	}
	defer root.Close()
	if _, err := root.Lstat(id); err == nil {
		session, err := s.load(root, id)
		if err != nil {
			return zero, err
		}
		defer session.root.Close()
		want, _ := json.Marshal(manifest)
		got, _ := json.Marshal(session.state.Manifest)
		if !bytes.Equal(want, got) {
			return zero, problem("conflict")
		}
		if err := session.recover(); err != nil {
			return zero, err
		}
		return session.State(), nil
	} else if !os.IsNotExist(err) {
		return zero, err
	}
	entries, err := root.Open(".")
	if err != nil {
		return zero, err
	}
	names, err := entries.Readdirnames(-1)
	entries.Close()
	if err != nil {
		return zero, err
	}
	var reserved int64
	count := 0
	for _, name := range names {
		if !validDigest(name) {
			continue
		}
		record, err := readState(root, name+"/state.json")
		// Unknown or damaged records cannot safely be counted as zero bytes.
		if err != nil {
			return zero, err
		}
		bytes, err := manifestBytes(record.Manifest)
		if err != nil {
			return zero, err
		}
		if bytes > 2*s.maxBytes-reserved {
			return zero, problem("quota")
		}
		reserved += bytes
		count++
	}
	if count >= MaxSessions || total > 2*s.maxBytes-reserved {
		return zero, problem("quota")
	}
	// Build in a disposable name, then publish the whole session atomically.
	temp := "create-" + id
	if err := root.RemoveAll(temp); err != nil {
		return zero, err
	}
	if err := root.Mkdir(temp, 0700); err != nil {
		return zero, err
	}
	defer root.RemoveAll(temp)
	dir, err := root.OpenRoot(temp)
	if err != nil {
		return zero, err
	}
	defer dir.Close()
	state := State{Version: Version, ID: id, Manifest: manifest, Offsets: make([]int64, len(manifest.Files)), Expires: s.now().UTC().Add(IdleTTL)}
	session := &Session{store: s, root: dir, state: state}
	for i := range manifest.Files {
		f, err := dir.OpenFile(strconv.Itoa(i), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return zero, err
		}
		err = f.Sync()
		closeErr := f.Close()
		if err != nil {
			return zero, err
		}
		if closeErr != nil {
			return zero, closeErr
		}
	}
	if err := session.save(); err != nil {
		return zero, err
	}
	// Windows cannot rename a directory while its handle is open.
	if err := dir.Close(); err != nil {
		return zero, err
	}
	if err := s.checkpoint("before_publish"); err != nil {
		return zero, err
	}
	if err := replacePath(root, temp, id); err != nil {
		return zero, err
	}
	if err := syncDirectory(s.path); err != nil {
		return zero, err
	}
	return session.State(), nil
}

// Session is only valid inside With. Its OS lock protects staging while the
// caller verifies or imports files, including during cleanup from other servers.
type Session struct {
	store *Store
	root  *os.Root
	state State
}

// With serializes all access to one session and repairs unacknowledged tails.
func (s *Store) With(ctx context.Context, id string, fn func(*Session) error) error {
	release, err := s.lock(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return err
	}
	defer root.Close()
	session, err := s.load(root, id)
	if err != nil {
		return err
	}
	defer session.root.Close()
	if !s.now().Before(session.state.Expires) {
		return problem("expired")
	}
	if session.state.Canceled {
		return problem("canceled")
	}
	if err := session.recover(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(session)
}

func (s *Store) load(root *os.Root, id string) (*Session, error) {
	info, err := root.Lstat(id)
	if os.IsNotExist(err) {
		return nil, problem("not_found")
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, problem("corrupt")
	}
	dir, err := root.OpenRoot(id)
	if err != nil {
		return nil, err
	}
	state, err := readState(dir, "state.json")
	if err == nil && state.ID != id {
		err = problem("corrupt")
	}
	if err != nil {
		dir.Close()
		return nil, err
	}
	return &Session{store: s, root: dir, state: state}, nil
}

func readState(root *os.Root, name string) (State, error) {
	var state State
	info, err := root.Lstat(name)
	if err != nil {
		return state, err
	}
	if !info.Mode().IsRegular() {
		return state, problem("corrupt")
	}
	if info.Size() > maxRecordBytes {
		return state, problem("corrupt")
	}
	f, err := root.Open(name)
	if err != nil {
		return state, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, maxRecordBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, problem("corrupt")
	}
	if state.Version != Version {
		return state, problem("unsupported_version")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return state, problem("corrupt")
	}
	if _, err := manifestBytes(state.Manifest); err != nil {
		return state, problem("corrupt")
	}
	if !validDigest(state.ID) || state.Expires.IsZero() || !state.validReceipt() || len(state.Offsets) != len(state.Manifest.Files) {
		return state, problem("corrupt")
	}
	if len(state.Digests) > 0 {
		if state.Manifest.Protocol != 2 || len(state.Digests) != len(state.Manifest.Files) {
			return state, problem("corrupt")
		}
		for _, digest := range state.Digests {
			if !validDigest(digest) {
				return state, problem("corrupt")
			}
		}
	}
	for i, offset := range state.Offsets {
		if offset < 0 || offset > state.Manifest.Files[i].Size {
			return state, problem("corrupt")
		}
	}
	return state, nil
}

func (s *Session) State() State {
	state := s.state
	state.Manifest.Files = append([]File(nil), state.Manifest.Files...)
	state.Offsets = append([]int64(nil), state.Offsets...)
	state.Digests = append([]string(nil), state.Digests...)
	return state
}

func (s *Session) save() error {
	raw, err := json.Marshal(s.state)
	if err != nil {
		return err
	}
	if len(raw) > maxRecordBytes {
		return problem("invalid_manifest")
	}
	// Leftovers from a killed writer are never progress records.
	if err := s.root.Remove("state.next"); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := s.root.OpenFile("state.next", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove("state.next")
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := s.store.checkpoint("before_progress"); err != nil {
		return err
	}
	if err := replacePath(s.root, "state.next", "state.json"); err != nil {
		return err
	}
	if err := syncDirectory(s.root.Name()); err != nil {
		return err
	}
	return s.store.checkpoint("after_progress")
}

func (s *Session) openFile(index int) (*os.File, error) {
	name := strconv.Itoa(index)
	info, err := s.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, problem("corrupt")
	}
	return s.root.OpenFile(name, os.O_RDWR, 0)
}

func (s *Session) recover() error {
	for i, offset := range s.state.Offsets {
		f, err := s.openFile(i)
		if os.IsNotExist(err) {
			return problem("corrupt")
		}
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err == nil && info.Size() < offset {
			err = problem("corrupt")
		}
		if err == nil && info.Size() > offset {
			err = f.Truncate(offset)
			if err == nil {
				err = f.Sync()
			}
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// Renew explicitly resumes an idle session. Merely querying does not renew it.
func (s *Session) Renew() error {
	s.state.Expires = s.store.now().UTC().Add(IdleTTL)
	return s.save()
}

// Append confirms a block only after data and progress are durable. Retrying
// an already confirmed range verifies actual disk bytes, not an old digest.
func (s *Session) Append(ctx context.Context, index int, offset int64, data []byte, digest string) error {
	if s.state.Completed() {
		return problem("completed")
	}
	if index < 0 || index >= len(s.state.Offsets) || offset < 0 || len(data) == 0 || len(data) > ChunkBytes || !validDigest(digest) {
		return problem("invalid_chunk")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return problem("chunk_integrity")
	}
	current := s.state.Offsets[index]
	if int64(len(data)) > s.state.Manifest.Files[index].Size-offset || offset > current {
		return &Problem{Kind: "offset", Offset: current}
	}
	if offset < current && int64(len(data)) > current-offset {
		return &Problem{Kind: "offset", Offset: current}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := s.openFile(index)
	if err != nil {
		return err
	}
	defer f.Close()
	if offset < current {
		hash := sha256.New()
		if _, err := io.CopyN(hash, io.NewSectionReader(f, offset, int64(len(data))), int64(len(data))); err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != digest {
			return problem("chunk_integrity")
		}
		return s.Renew()
	}
	write := s.store.writeAt
	if write == nil {
		write = (*os.File).WriteAt
	}
	if _, err := write(f, data, offset); err != nil {
		return err
	}
	if err := s.store.checkpoint("after_write"); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := s.store.checkpoint("after_data_sync"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.state.Offsets[index] = offset + int64(len(data))
	s.state.Expires = s.store.now().UTC().Add(IdleTTL)
	return s.save()
}

// Verify checks every original before archive preparation. Paths are only
// usable while With holds the session lock. The archive must still verify the
// bytes it actually imports, including its content-addressed deduplication path.
func (s *Session) Verify(ctx context.Context) ([]string, error) { return s.paths(ctx, true) }

// Paths checks completeness and metadata. The caller must use PutVerified and
// verify final blobs while holding the session lock before committing.
func (s *Session) Paths(ctx context.Context) ([]string, error) { return s.paths(ctx, false) }

func (s *Session) paths(ctx context.Context, verify bool) ([]string, error) {
	paths := make([]string, len(s.state.Manifest.Files))
	for i, expected := range s.state.Manifest.Files {
		if s.state.Offsets[i] != expected.Size {
			return nil, problem("incomplete")
		}
		f, err := s.openFile(i)
		if err != nil {
			return nil, err
		}
		if s.state.Manifest.Protocol == 2 {
			if len(s.state.Digests) != len(s.state.Manifest.Files) {
				f.Close()
				return nil, problem("incomplete")
			}
			expected.SHA256 = s.state.Digests[i]
		}
		if verify {
			hash := sha256.New()
			n, err := io.Copy(hash, contextReader{ctx: ctx, r: f})
			closeErr := f.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if n != expected.Size || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
				return nil, problem("file_integrity")
			}
		} else {
			if err := f.Close(); err != nil {
				return nil, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strconv.Itoa(i)
		modified := time.UnixMilli(expected.LastModified)
		if err := s.root.Chtimes(name, modified, modified); err != nil {
			return nil, err
		}
		paths[i] = filepath.Join(s.root.Name(), name)
	}
	return paths, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// Complete records a receipt only after library has durably committed it.
// The archive's receipt remains authoritative if this save fails.
func (s *Session) Complete(opID int64) error {
	if opID <= 0 || s.state.Completed() && s.state.OpID != opID {
		return problem("conflict")
	}
	s.state.OpID = opID
	return s.save()
}

// CompleteLocal records a durable client request receipt. It is not a NAS op ID.
// The client's pending journal remains authoritative if this save fails.
func (s *Session) CompleteLocal(localID string, noop bool) error {
	receipt := State{LocalID: localID, Noop: noop}
	if !receipt.Completed() || !receipt.validReceipt() || s.state.Completed() && (s.state.LocalID != localID || s.state.Noop != noop) {
		return problem("conflict")
	}
	s.state.LocalID, s.state.Noop = localID, noop
	return s.save()
}

// Cancel marks staging for cleanup. The caller first checks the archive receipt
// under this same session lock; cancel must never undo an already committed op.
func (s *Session) Cancel() error {
	if s.state.Completed() {
		return problem("completed")
	}
	s.state.Canceled = true
	return s.save()
}

// Sweep skips busy sessions and removes expired or terminal staging. Call it
// outside With; cleanup never removes a live writer's creation directory.
func (s *Store) Sweep(ctx context.Context) error {
	release, err := lockfile.New(filepath.Join(s.path, "locks", "registry.lock")).Acquire(ctx)
	if err != nil {
		return err
	}
	err = s.cleanCreates()
	release()
	if err != nil {
		return err
	}
	return s.sweep(ctx)
}

// cleanCreates runs only under the registry lock, which also guards creation.
func (s *Store) cleanCreates() error {
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		if strings.HasPrefix(name, "create-") && validDigest(strings.TrimPrefix(name, "create-")) {
			if err := root.RemoveAll(name); err != nil {
				return err
			}
		}
	}
	return syncDirectory(s.path)
}

func (s *Store) sweep(ctx context.Context) error {
	root, err := os.OpenRoot(s.path)
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		id := strings.TrimPrefix(name, "discard-")
		if !validDigest(id) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		tryCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
		release, err := s.lock(tryCtx, id)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) || lockfile.IsBusy(err) {
				continue
			}
			return err
		}
		err = func() error {
			defer release()
			if name != id {
				if err := root.RemoveAll(name); err != nil {
					return err
				}
				return syncDirectory(s.path)
			}
			session, err := s.load(root, id)
			if err != nil {
				var p *Problem
				if errors.As(err, &p) && p.Kind == "not_found" {
					return nil
				}
				return err
			}
			state := session.state
			session.root.Close()
			if !state.Canceled && !state.Completed() && s.now().Before(state.Expires) {
				return nil
			}
			// Publish removal before recursive cleanup. If deletion is interrupted,
			// the next sweep sees discard-ID rather than a damaged live session.
			garbage := "discard-" + id
			if err := root.RemoveAll(garbage); err != nil {
				return err
			}
			if err := replacePath(root, id, garbage); err != nil {
				return err
			}
			if err := syncDirectory(s.path); err != nil {
				return err
			}
			if err := s.checkpoint("after_discard"); err != nil {
				return err
			}
			if err := root.RemoveAll(garbage); err != nil {
				return err
			}
			return syncDirectory(s.path)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

// Seal binds final client digests once. Retrying cannot change an upload's content.
func (s *Session) Seal(digests []string) error {
	if s.state.Manifest.Protocol != 2 {
		if len(digests) != 0 {
			return problem("invalid_manifest")
		}
		return nil
	}
	if len(digests) != len(s.state.Manifest.Files) {
		return problem("invalid_manifest")
	}
	for i, d := range digests {
		if !validDigest(d) {
			return problem("invalid_manifest")
		}
		if s.state.Offsets[i] != s.state.Manifest.Files[i].Size {
			return problem("incomplete")
		}
	}
	if len(s.state.Digests) > 0 {
		for i, d := range digests {
			if d != s.state.Digests[i] {
				return problem("file_integrity")
			}
		}
		return nil
	}
	s.state.Digests = append([]string(nil), digests...)
	return s.save()
}

// MatchPrefix checks actual durable bytes when originals are reselected.
func (s *Session) MatchPrefix(ctx context.Context, index int, offset int64, digest string) error {
	if index < 0 || index >= len(s.state.Offsets) || offset != s.state.Offsets[index] {
		return problem("invalid_chunk")
	}
	if !validDigest(digest) {
		return problem("invalid_chunk")
	}
	f, err := s.openFile(index)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, contextReader{ctx: ctx, r: f}, offset); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return problem("file_integrity")
	}
	return nil
}
