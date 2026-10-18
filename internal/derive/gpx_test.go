package derive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGPXSegmentsAndWaypoints(t *testing.T) {
	input := `<gpx xmlns="http://www.topografix.com/GPX/1/1"><wpt lat="10" lon="20"/><trk><trkseg><trkpt lat="1" lon="179"/><trkpt lat="2" lon="-179"/></trkseg><trkseg><trkpt lat="3" lon="4"/></trkseg></trk><rte><rtept lat="5" lon="6"/><rtept lat="7" lon="8"/></rte><extensions><wpt lat="invalid" lon="invalid"/></extensions></gpx>`
	data, err := parseGPX(t.Context(), strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Features) != 4 {
		t.Fatalf("segments merged: %+v", data)
	}
	line, ok := data.Features[1].Geometry.Coordinates.([][2]float64)
	if !ok || len(line) != 2 || line[1] != [2]float64{-179, 2} {
		t.Fatalf("coordinates lost: %+v", data)
	}
	root := t.TempDir()
	src := filepath.Join(root, "original")
	dst := filepath.Join(root, "map.geojson")
	if err = os.WriteFile(src, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := (Renderer{}).Render(t.Context(), src, dst, GPXMap)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || !json.Valid(b) || result.Bytes != int64(len(b)) || result.Engine != "go-gpx-1" {
		t.Fatalf("%+v %s %v", result, b, err)
	}
	recipe, _ := Lookup(GPXMap)
	if recipe.Filename() == GPXMap+".json" {
		t.Fatal("output collides with job record")
	}
}

func TestGPXRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{`<html/>`, `<gpx/>`, `<gpx><wpt lat="NaN" lon="1"/></gpx>`, `<gpx><wpt lat="91" lon="1"/></gpx>`, `<gpx><wpt lat="1"/></gpx>`, `<gpx><wpt lat="1" lon="2"/>`, `<gpx><wpt lat="1" lon="2"/></gpx><gpx/>`, `<!DOCTYPE gpx [<!ENTITY x SYSTEM "file:///etc/passwd">]><gpx>&x;</gpx>`, `<gpx xmlns="https://example.invalid"><wpt lat="1" lon="2"/></gpx>`, `<gpx><extensions><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></extensions></gpx>`} {
		if _, err := parseGPX(t.Context(), strings.NewReader(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	for _, input := range []string{`<gpx>` + strings.Repeat(`<x>`, 65), `<gpx>` + strings.Repeat(`<wpt lat="1" lon="2"/>`, maxGPXPoints+1) + `</gpx>`} {
		if _, err := parseGPX(t.Context(), strings.NewReader(input)); Code(err) != "limit_exceeded" {
			t.Fatalf("limit: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := parseGPX(ctx, strings.NewReader(`<gpx/>`)); Code(err) != "cancelled" {
		t.Fatal(err)
	}
}
