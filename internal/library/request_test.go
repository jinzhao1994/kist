package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func requestFor(key, body string) Request {
	sum := sha256.Sum256([]byte(body))
	return Request{ID: key, Digest: hex.EncodeToString(sum[:])}
}

func TestRequestRetryReturnsOriginalResult(t *testing.T) {
	l := openTestLibrary(t)
	req := requestFor("request-001", "create box")
	calls := 0
	create := func(ctx context.Context) error {
		calls++
		_, err := l.CreateBox(ctx, "archive", "", domain.LocalDate{}, domain.LocalDate{})
		return err
	}
	first, err := l.ExecuteRequest(t.Context(), req, create)
	if err != nil {
		t.Fatal(err)
	}
	second, err := l.ExecuteRequest(t.Context(), req, create)
	if err != nil {
		t.Fatal(err)
	}
	if first.OpID == 0 || first != second || calls != 1 {
		t.Fatalf("first=%v second=%v calls=%d", first, second, calls)
	}
	if _, err := l.ExecuteRequest(t.Context(), requestFor(req.ID, "other body"), create); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("conflict = %v", err)
	}
	if n, _ := l.OpCount(t.Context()); n != 1 {
		t.Fatalf("ops = %d", n)
	}
	status, found, err := l.RequestStatus(t.Context(), req.ID)
	if err != nil || !found || status != first {
		t.Fatalf("status=%v found=%v err=%v", status, found, err)
	}
}

func TestConcurrentRequestsAcrossConnectionsCommitOnce(t *testing.T) {
	l := openTestLibrary(t)
	second, err := Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	req := requestFor("request-concurrent", "create tag")
	var calls atomic.Int32
	var wg sync.WaitGroup
	const count = 8
	results := make(chan RequestResult, count)
	failures := make(chan error, count)
	for i := range count {
		db := l
		if i%2 == 1 {
			db = second
		}
		wg.Go(func() {
			result, err := db.ExecuteRequest(t.Context(), req, func(ctx context.Context) error {
				calls.Add(1)
				_, err := db.CreateTag(ctx, "once", nil)
				return err
			})
			results <- result
			failures <- err
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	for result := range results {
		if result.OpID != 1 {
			t.Fatalf("result=%v", result)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("callback ran %d times", calls.Load())
	}
}

func TestNoChangeReceiptSurvivesLaterChanges(t *testing.T) {
	l := openTestLibrary(t)
	b, err := l.CreateBox(t.Context(), "before", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	req := requestFor("request-no-change", "name=before")
	result, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error {
		_, err := l.UpdateBox(ctx, b.ID, BoxFields{Name: "before"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, _, err := store.GetOp(l.db, result.OpID)
	if err != nil || receipt.Action != string(oplog.ActionRequestReceipt) {
		t.Fatalf("receipt=%v err=%v", receipt, err)
	}
	after, _, _ := l.Box(t.Context(), b.ID)
	if after.UpdatedAt != b.UpdatedAt {
		t.Fatal("receipt touched content")
	}
	if _, err := l.UpdateBox(t.Context(), b.ID, BoxFields{Name: "later"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ExecuteRequest(t.Context(), req, func(context.Context) error { t.Fatal("no-op replayed after later edit"); return nil }); err != nil {
		t.Fatal(err)
	}
	after, _, _ = l.Box(t.Context(), b.ID)
	if after.Name != "later" {
		t.Fatal("retry changed later state")
	}
	if _, err := l.Undo(t.Context(), result.OpID); !errors.Is(err, oplog.ErrNotUndoable) {
		t.Fatalf("undo receipt = %v", err)
	}
}

func TestRequestRecoversAfterAppendAndRebuild(t *testing.T) {
	l := openTestLibrary(t)
	req := requestFor("request-crash", "create tag")
	boom := errors.New("power cut")
	l.hooks.afterAppend = func() error { return boom }
	result, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error { _, err := l.CreateTag(ctx, "survives", nil); return err })
	var outcome *WriteOutcomeError
	if !errors.As(err, &outcome) || outcome.OpID != 1 || result.OpID != 1 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	l.hooks.afterAppend = nil
	l.Close()
	reopened, err := Open(l.Layout().Root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	replayed, err := reopened.ExecuteRequest(t.Context(), req, func(context.Context) error { t.Fatal("committed request ran again"); return nil })
	if err != nil || replayed != result {
		t.Fatalf("replayed=%v err=%v", replayed, err)
	}
	before := snapshotOf(t, reopened)
	if err := reopened.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, reopened) != before {
		t.Fatal("rebuild changed request result")
	}
	if _, err := reopened.db.Exec(`UPDATE op_requests SET digest = 'corrupt'`); err != nil {
		t.Fatal(err)
	}
	report, err := reopened.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Count(integrity.ViewDrift) == 0 {
		t.Fatal("deep did not detect corrupted request index")
	}
	if err := reopened.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if snapshotOf(t, reopened) != before {
		t.Fatal("rebuild did not repair request index")
	}
}

func TestRejectedRequestDoesNotReserveKey(t *testing.T) {
	l := openTestLibrary(t)
	req := requestFor("request-rejected", "invalid command")
	boom := errors.New("validation failed")
	if _, err := l.ExecuteRequest(t.Context(), req, func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, found, err := l.RequestStatus(t.Context(), req.ID); err != nil || found {
		t.Fatalf("rejected request exists: %v %v", found, err)
	}
	if n, _ := l.OpCount(t.Context()); n != 0 {
		t.Fatal("rejection appended history")
	}
	if _, err := l.ExecuteRequest(t.Context(), Request{}, func(context.Context) error { t.Fatal("invalid request executed"); return nil }); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal(err)
	}
}

func TestRequestScopeCannotBeReusedOrWriteTwice(t *testing.T) {
	l := openTestLibrary(t)
	var saved context.Context
	result, err := l.ExecuteRequest(t.Context(), requestFor("request-scope", "two writes"), func(ctx context.Context) error {
		saved = ctx
		if _, err := l.CreateTag(ctx, "first", nil); err != nil {
			return err
		}
		_, err := l.CreateTag(ctx, "second", nil)
		return err
	})
	var outcome *WriteOutcomeError
	if !errors.As(err, &outcome) || !errors.Is(err, ErrRequestScope) || result.OpID != 1 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	if _, err := l.CreateTag(saved, "outside", nil); !errors.Is(err, ErrRequestScope) {
		t.Fatalf("saved context = %v", err)
	}
	if n, _ := l.OpCount(t.Context()); n != 1 {
		t.Fatalf("ops=%d", n)
	}
}

func TestFailedRequestedApplyDoesNotReserveKey(t *testing.T) {
	l := openTestLibrary(t)
	parent, err := l.CreateTag(t.Context(), "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateTag(t.Context(), "child", &parent.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := l.OpCount(t.Context())
	req := requestFor("request-refused-apply", "delete-parent")
	if _, err := l.ExecuteRequest(t.Context(), req, func(ctx context.Context) error { return l.DeleteTag(ctx, parent.ID) }); !errors.Is(err, oplog.ErrTagInUse) {
		t.Fatalf("delete = %v", err)
	}
	if _, found, err := l.RequestStatus(t.Context(), req.ID); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if after, _ := l.OpCount(t.Context()); after != before {
		t.Fatal("rejected apply polluted history")
	}
}
