package library

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/geocode"
)

func TestLocationQueueActivationAndBackfillWithoutAI(t *testing.T) {
	l := openTestLibrary(t)
	settings, err := l.LocationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Automatic {
		t.Fatal("new libraries must enable location lookup")
	}
	settings.Automatic = false
	if settings, err = l.SaveLocationSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	old := ocrFile(t, l)
	settings.Automatic = true
	if _, err = l.SaveLocationSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	fresh := ocrFile(t, l)
	q := l.LocationQueue()
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "missing")
	requireOCRState(t, q, fresh, "ready")
	rows := metaOf(t, l, fresh)
	if _, ok := rows["media:mime"]; !ok {
		t.Fatal("basic extraction not completed")
	}
	if _, err = q.Backfill(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if err = q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, old, "ready")
}
func TestLocationPersistenceAndSnapshotConflict(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"properties":{"type":"street","city":"Example","name":"Road"}}]}`))
	}))
	defer server.Close()
	l := openTestLibrary(t)
	id := ocrFile(t, l)
	extractWith(t, l, &fakeExtractor{id: "exif", ns: []string{"gps", "exif"}, version: 1, tags: []extract.KV{{Key: "gps:Latitude", Value: "12", Type: extract.TypeReal}, {Key: "gps:Longitude", Value: "34", Type: extract.TypeReal}}})
	settings, _ := l.LocationSettings(t.Context())
	settings.Endpoint = server.URL
	if _, err := l.SaveLocationSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	q := l.LocationQueue()
	if _, err := q.Request(t.Context(), id, false); err != nil {
		t.Fatal(err)
	}
	if err := q.Step(t.Context()); err != nil {
		t.Fatal(err)
	}
	requireOCRState(t, q, id, "ready")
	if got := metaOf(t, l, id)["geo:address"].Value; got != "Example · Road" {
		t.Fatal(got)
	}
	q.Request(t.Context(), id, false)
	q.Step(t.Context())
	if calls != 1 {
		t.Fatal("saved address queried again", calls)
	}
	file, _, _ := l.File(t.Context(), id)
	p, err := l.prepareLocation(t.Context(), id, file.BlobKey, true)
	if err != nil {
		t.Fatal(err)
	}
	extractWith(t, l, &fakeExtractor{id: "exif", ns: []string{"gps", "exif"}, version: 2, tags: []extract.KV{{Key: "gps:Latitude", Value: "13", Type: extract.TypeReal}, {Key: "gps:Longitude", Value: "34", Type: extract.TypeReal}}})
	request := q.jobRequest(OCRJob{FileID: id, BlobKey: file.BlobKey, RequestID: uuid.NewString()})
	if _, err = l.ExecuteRequest(t.Context(), request, p.Commit); !errors.Is(err, ErrSyncConflict) {
		t.Fatal("stale GPS accepted", err)
	}
	if err := l.Rebuild(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := metaOf(t, l, id)["geo:address"].Value; got != "Example · Road" {
		t.Fatal("rebuild lost location", got)
	}
}

type recordingGeocoder struct {
	called []time.Time
	err    error
}

func (p *recordingGeocoder) Reverse(context.Context, geocode.Point) (geocode.Result, error) {
	p.called = append(p.called, time.Now())
	return geocode.Result{}, p.err
}
func TestLocationRatePersistsAcrossWorkersAndCancellation(t *testing.T) {
	l := openTestLibrary(t)
	p := &recordingGeocoder{}
	if _, err := l.lookupLocation(t.Context(), p, geocode.Point{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.lookupLocation(t.Context(), p, geocode.Point{}); err != nil {
		t.Fatal(err)
	}
	if len(p.called) != 2 || p.called[1].Sub(p.called[0]) < 2*time.Second {
		t.Fatal("requests not spaced")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := l.lookupLocation(ctx, p, geocode.Point{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(p.called) != 2 {
		t.Fatal("cancelled request reached provider")
	}
	db, err := l.runtimeStore()
	if err != nil {
		t.Fatal(err)
	}
	var value string
	if err = db.DB.QueryRow(`SELECT next_at FROM service_limits WHERE service='photon'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	next, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !next.After(time.Now()) {
		t.Fatal("rate not persisted", err)
	}

}

func TestLocationCooldownSurvivesClientReplacement(t *testing.T) {
	l := openTestLibrary(t)
	p := &recordingGeocoder{err: &geocode.RetryError{After: 3 * time.Minute}}
	if _, err := l.lookupLocation(t.Context(), p, geocode.Point{}); err == nil {
		t.Fatal("rate error discarded")
	}
	replacement := &recordingGeocoder{}
	_, err := l.lookupLocation(t.Context(), replacement, geocode.Point{})
	var retry *geocode.RetryError
	if !errors.As(err, &retry) || retry.After < 2*time.Minute || len(replacement.called) != 0 {
		t.Fatal("retry ignored persisted cooldown", err, replacement.called)
	}
}

func TestLocationFailurePolicyAndCooldown(t *testing.T) {
	for _, pause := range []bool{true, false} {
		t.Run(fmt.Sprint(pause), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"properties":{"type":"street","name":"Test Road"}}]}`))
			}))
			defer server.Close()
			l := openTestLibrary(t)
			first, second := ocrFile(t, l), ocrFile(t, l)
			extractWith(t, l, &fakeExtractor{id: "exif", ns: []string{"gps", "exif"}, version: 1, tags: []extract.KV{{Key: "gps:Latitude", Value: "12", Type: extract.TypeReal}, {Key: "gps:Longitude", Value: "34", Type: extract.TypeReal}}})
			settings, err := l.LocationSettings(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			settings.Automatic = false
			settings.Endpoint = server.URL
			settings.RequestIntervalSeconds = 0
			settings.PauseOnError = pause
			saved, err := l.SaveLocationSettings(t.Context(), settings)
			if err != nil {
				t.Fatal(err)
			}
			if saved.RequestIntervalSeconds != 0 || saved.PauseOnError != pause {
				t.Fatal("policy not saved")
			}
			q := l.LocationQueue()
			if _, err = q.Request(t.Context(), first, false); err != nil {
				t.Fatal(err)
			}
			if _, err = q.Request(t.Context(), second, false); err != nil {
				t.Fatal(err)
			}
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			status, err := q.Status(t.Context())
			if err != nil || status.Paused != pause {
				t.Fatal("unexpected pause policy", err, status)
			}
			requireOCRState(t, q, first, "failed")
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			requireOCRState(t, q, second, "queued")
			if calls != 1 {
				t.Fatal("cooldown did not protect the next item")
			}
			db, err := l.runtimeStore()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.DB.Exec(`UPDATE service_limits SET next_at=? WHERE service='photon'`, time.Now().Add(-time.Second).Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			if pause {
				if _, err = q.Pause(t.Context(), false); err != nil {
					t.Fatal(err)
				}
			}
			if err = q.Step(t.Context()); err != nil {
				t.Fatal(err)
			}
			requireOCRState(t, q, second, "ready")
			requireOCRState(t, q, first, "failed")
			if calls != 2 {
				t.Fatal("failed item was retried or next item did not run", calls)
			}
		})
	}
}

func TestLocationConfiguredInterval(t *testing.T) {
	l := openTestLibrary(t)
	settings, err := l.LocationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if settings.RequestIntervalSeconds != 2 || !settings.PauseOnError {
		t.Fatal("unexpected defaults", settings)
	}
	settings.RequestIntervalSeconds = 7
	if _, err = l.SaveLocationSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	p := &recordingGeocoder{}
	if _, err = l.lookupLocation(t.Context(), p, geocode.Point{}); err != nil {
		t.Fatal(err)
	}
	db, err := l.runtimeStore()
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err = db.DB.QueryRow(`SELECT next_at FROM service_limits WHERE service='photon'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	next, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || time.Until(next) < 6*time.Second {
		t.Fatal("configured interval was ignored", err)
	}
	settings, err = l.LocationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	settings.RequestIntervalSeconds = -1
	if _, err = l.SaveLocationSettings(t.Context(), settings); err == nil {
		t.Fatal("accepted negative interval")
	}
}
