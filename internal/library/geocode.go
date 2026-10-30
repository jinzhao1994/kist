package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/extract"
	"github.com/jinzhao1994/kist/internal/geocode"
	"github.com/jinzhao1994/kist/internal/lockfile"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

type geoExtractor struct{}

func (geoExtractor) ID() string           { return "geo" }
func (geoExtractor) Version() int         { return 1 }
func (geoExtractor) Namespaces() []string { return []string{"geo"} }
func (geoExtractor) Wants(m string) bool  { e, _ := extract.Lookup("exif"); return e.Wants(m) }
func (geoExtractor) Extract(context.Context, io.ReadSeeker, string) ([]extract.KV, error) {
	return nil, errors.New("location extraction requires a provider")
}

func gpsPoint(rows []store.MetadataRow) *geocode.Point {
	values := map[string]string{}
	for _, r := range rows {
		if !r.Tombstone {
			values[r.Key] = r.Value
		}
	}
	return geocode.Coordinates(values)
}

func (l *Library) prepareLocation(ctx context.Context, id domain.FileID, expectedBlob string, force bool) (*PreparedExtraction, error) {
	p := &PreparedExtraction{library: l, extractor: "geo"}
	err := l.readTx(ctx, func(tx *store.ViewTx) error {
		f, found, err := tx.GetFile(id)
		if err != nil {
			return err
		}
		if !found {
			return ErrNoSuchFile
		}
		if f.BlobKey != expectedBlob {
			return ErrSyncConflict
		}
		p.file = &f
		p.rows, err = store.FileMetadata(tx.Queryer(), id)
		if err != nil {
			return err
		}
		v, err := store.Extractions(tx.Queryer(), id)
		p.version = v["geo"]
		return err
	})
	if err != nil {
		return nil, err
	}
	if !force && p.version >= 1 {
		return p, nil
	}
	point := gpsPoint(p.rows)
	facts := []extract.KV{}
	if point != nil {
		cfg, err := config.Load(l.layout.ConfigPath())
		if err != nil {
			return nil, err
		}
		result, err := l.lookupLocation(ctx, geocode.Photon{Endpoint: cfg.Geocoding.Endpoint, Client: &http.Client{Timeout: 20 * time.Second}}, *point)
		if err != nil {
			return nil, err
		}
		facts = append(facts, extract.KV{Key: "geo:address", Value: result.Address, Type: extract.TypeText}, extract.KV{Key: "geo:provider", Value: "photon", Type: extract.TypeText})
	}
	e := geoExtractor{}
	if err = extract.Check(e, facts); err != nil {
		return nil, err
	}
	change := extractionChange(e, *p.file, facts, p.rows, time.UTC)
	change.CapturedAt = p.file.CapturedAt
	change.CapturedWas = p.file.CapturedAt
	p.payload, err = oplog.Marshal(oplog.ExtractMetadataPayload{Extractor: "geo", Version: 1, PreviousVersion: p.version, Files: []oplog.FileExtraction{*change}})
	return p, err
}

// lookupLocation holds a dedicated OS lease across the request, not the archive
// write lock. Persisting the next slot makes throttling survive process restarts.
func (l *Library) lookupLocation(ctx context.Context, provider geocode.Provider, point geocode.Point) (geocode.Result, error) {
	if err := ctx.Err(); err != nil {
		return geocode.Result{}, err
	}
	release, err := lockfile.New(filepath.Join(l.layout.Root, "geocode-http.lock")).Acquire(ctx)
	if err != nil {
		return geocode.Result{}, err
	}
	defer release()
	cfg, err := config.Load(l.layout.ConfigPath())
	if err != nil {
		return geocode.Result{}, err
	}
	interval := time.Duration(cfg.Geocoding.RequestIntervalSeconds) * time.Second
	db, err := l.runtimeStore()
	if err != nil {
		return geocode.Result{}, err
	}
	path := filepath.Join(l.layout.Derived(), "geo", "rate.json")
	err = db.Import(ctx, "geocode-rate", func(tx *sql.Tx) error {
		raw, e := os.ReadFile(path)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil {
			return e
		}
		var next time.Time
		if e = json.Unmarshal(raw, &next); e != nil {
			return e
		}
		_, e = tx.Exec(`INSERT INTO service_limits VALUES('photon',?)`, next.Format(time.RFC3339Nano))
		return e
	})
	if err != nil {
		return geocode.Result{}, err
	}
	var next time.Time
	var value string
	err = db.DB.QueryRowContext(ctx, `SELECT next_at FROM service_limits WHERE service='photon'`).Scan(&value)
	if err == nil {
		next, err = time.Parse(time.RFC3339Nano, value)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return geocode.Result{}, err
	}
	if delay := time.Until(next); delay > 0 {
		if delay > interval {
			return geocode.Result{}, &geocode.RetryError{After: delay}
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return geocode.Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	save := func(next time.Time) error {
		// Persist cooldown even if the network request was cancelled.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, e := db.DB.ExecContext(saveCtx, `INSERT INTO service_limits VALUES('photon',?) ON CONFLICT(service) DO UPDATE SET next_at=excluded.next_at`, next.Format(time.RFC3339Nano))
		return e
	}
	// Reserve before I/O, so a killed process cannot release a burst of requests.
	if err = save(time.Now().Add(interval)); err != nil {
		return geocode.Result{}, err
	}
	result, callErr := provider.Reverse(ctx, point)
	delay := interval
	if callErr != nil {
		if delay < time.Minute {
			delay = time.Minute
		}
		var retry *geocode.RetryError
		if errors.As(callErr, &retry) && retry.After > delay {
			delay = retry.After
		}
	}
	if err = save(time.Now().Add(delay)); err != nil {
		return geocode.Result{}, err
	}
	return result, callErr
}

// locationCoolingDown lets the worker leave queued jobs untouched while the
// service cooldown is active, including when failures do not pause the queue.
func (l *Library) locationCoolingDown(ctx context.Context) (bool, error) {
	db, err := l.runtimeStore()
	if err != nil {
		return false, err
	}
	var raw string
	err = db.DB.QueryRowContext(ctx, `SELECT next_at FROM service_limits WHERE service='photon'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	next, err := time.Parse(time.RFC3339Nano, raw)
	return next.After(time.Now()), err
}

// LocationSettingDefaults exposes only editable defaults, never scan boundaries.
type LocationSettingDefaults struct {
	RequestIntervalSeconds int    `json:"request_interval_seconds"`
	PauseOnError           bool   `json:"pause_on_error"`
	Automatic              bool   `json:"automatic"`
	Endpoint               string `json:"endpoint"`
}

func locationSettingDefaults() LocationSettingDefaults {
	d := config.DefaultGeocoding()
	return LocationSettingDefaults{RequestIntervalSeconds: d.RequestIntervalSeconds, PauseOnError: d.PauseOnError, Automatic: d.Automatic, Endpoint: d.Endpoint}
}

type LocationSettings struct {
	Explicit               []string                 `json:"explicit" readOnly:"true"`
	Changed                []string                 `json:"changed,omitempty" maxItems:"4"`
	Defaults               *LocationSettingDefaults `json:"defaults,omitempty"`
	Reset                  []string                 `json:"reset,omitempty" maxItems:"4"`
	RequestIntervalSeconds int                      `json:"request_interval_seconds" minimum:"0" maximum:"86400"`
	PauseOnError           bool                     `json:"pause_on_error"`
	Automatic              bool                     `json:"automatic"`
	Endpoint               string                   `json:"endpoint" maxLength:"2048"`
	Revision               string                   `json:"revision"`
}

func locationSettings(c config.Geocoding) LocationSettings {
	defaults := locationSettingDefaults()
	return LocationSettings{Defaults: &defaults, RequestIntervalSeconds: c.RequestIntervalSeconds, PauseOnError: c.PauseOnError, Automatic: c.Automatic, Endpoint: c.Endpoint}
}
func (l *Library) LocationSettings(ctx context.Context) (LocationSettings, error) {
	c, explicit, revision, err := settingsSnapshot(l.layout.ConfigPath(), "geocoding", locationSettingDefaults())
	if err != nil {
		return LocationSettings{}, err
	}
	out := locationSettings(c.Geocoding)
	out.Explicit, out.Revision = explicit, revision
	return out, ctx.Err()
}
func (l *Library) SaveLocationSettings(ctx context.Context, in LocationSettings) (LocationSettings, error) {
	if err := l.UpdateCombinedSettings(ctx, in.Revision, map[string]any{}, nil, &in); err != nil {
		return LocationSettings{}, err
	}
	return l.LocationSettings(ctx)
}

func (l *Library) locationSettingsChanges(cfg *config.Config, in LocationSettings) (map[string]any, error) {
	in, selected, err := prepareSettings(in, locationSettings(cfg.Geocoding), locationSettingDefaults(), in.Changed, in.Reset)
	if err != nil {
		return nil, err
	}
	next := config.Geocoding{RequestIntervalSeconds: in.RequestIntervalSeconds, PauseOnError: in.PauseOnError, Automatic: in.Automatic, Endpoint: in.Endpoint, AfterOp: cfg.Geocoding.AfterOp}
	if next.Automatic && !cfg.Geocoding.Automatic {
		next.AfterOp, err = store.MaxOpID(l.db)
		if err != nil {
			return nil, err
		}
	}
	changes := map[string]any{"geocoding.request_interval_seconds": next.RequestIntervalSeconds, "geocoding.pause_on_error": next.PauseOnError, "geocoding.automatic": next.Automatic, "geocoding.endpoint": next.Endpoint}
	for key := range changes {
		field := key[len("geocoding."):]
		if !selected[field] {
			delete(changes, key)
		}
	}
	changes["geocoding.after_op"] = next.AfterOp
	for _, key := range in.Reset {
		changes["geocoding."+key] = nil
	}

	return changes, nil
}
