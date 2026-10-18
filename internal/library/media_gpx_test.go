package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/derive"
)

func TestGPXMediaPublicationAndReuse(t *testing.T) {
	l, box := openWithBox(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "route.gpx"), []byte(`<gpx><trk><trkseg><trkpt lat="45" lon="6"/><trkpt lat="46" lon="7"/></trkseg></trk></gpx>`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Import(t.Context(), dir, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	id := listAll(t, l, box)[0].ID
	m := l.Media()
	requested, err := m.Request(t.Context(), id, derive.GPXMap, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.work(t.Context()); err != nil {
		t.Fatal(err)
	}
	ready, file, err := l.Media().Open(t.Context(), id, derive.GPXMap)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var data struct {
		Type     string
		Features []json.RawMessage
	}
	if err = json.NewDecoder(file).Decode(&data); err != nil || data.Type != "FeatureCollection" || len(data.Features) != 1 {
		t.Fatalf("output: %+v %v", data, err)
	}
	if ready.State != "ready" || ready.Generation != requested.Generation {
		t.Fatal(ready)
	}
	reused, err := l.Media().Request(t.Context(), id, derive.GPXMap, false)
	if err != nil || reused.Generation != ready.Generation || reused.State != "ready" {
		t.Fatalf("reuse: %+v %v", reused, err)
	}
	report, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil || report.Total() != 0 {
		t.Fatalf("verify: %+v %v", report, err)
	}
}
