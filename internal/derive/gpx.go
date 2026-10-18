package derive

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const GPXMap = "gpx_map_v1"
const maxGPXBytes = 32 << 20
const maxGPXPoints = 100_000

type gpxGeometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}
type gpxFeature struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
	Geometry   gpxGeometry       `json:"geometry"`
}
type gpxCollection struct {
	Type     string       `json:"type"`
	Features []gpxFeature `json:"features"`
}

// parseGPX reads only coordinates from GPX 1.0/1.1. Extensions and URLs never
// become resources to fetch; segment boundaries remain explicit in the output.
func parseGPX(ctx context.Context, input io.Reader) (gpxCollection, error) {
	out := gpxCollection{Type: "FeatureCollection", Features: []gpxFeature{}}
	d := xml.NewDecoder(io.LimitReader(contextReader{ctx, input}, maxGPXBytes+1))
	var stack []xml.Name
	var segment [][2]float64
	count := 0
	rootSeen := false
	flush := func() {
		if len(segment) == 0 {
			return
		}
		geometry := gpxGeometry{Type: "LineString", Coordinates: segment}
		if len(segment) == 1 {
			geometry = gpxGeometry{Type: "Point", Coordinates: segment[0]}
		}
		out.Features = append(out.Features, gpxFeature{Type: "Feature", Properties: map[string]string{}, Geometry: geometry})
		segment = nil
	}
	for {
		token, err := d.Token()
		if d.InputOffset() > maxGPXBytes {
			return out, fail("limit_exceeded", "GPX exceeds input limit")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, &Failure{"decode_failed", err}
		}
		switch t := token.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				if rootSeen || t.Name.Local != "gpx" || (t.Name.Space != "" && t.Name.Space != "http://www.topografix.com/GPX/1/1" && t.Name.Space != "http://www.topografix.com/GPX/1/0") {
					return out, fail("unsupported", "expected GPX document")
				}
				rootSeen = true
			}
			stack = append(stack, t.Name)
			if len(stack) > 64 {
				return out, fail("limit_exceeded", "GPX nesting exceeds limit")
			}
			path := func(names ...string) bool {
				if len(names) != len(stack) {
					return false
				}
				for i, n := range names {
					if stack[i].Local != n || stack[i].Space != stack[0].Space {
						return false
					}
				}
				return true
			}
			point := path("gpx", "trk", "trkseg", "trkpt") || path("gpx", "rte", "rtept") || path("gpx", "wpt")
			if !point {
				continue
			}
			var lat, lon float64
			var haveLat, haveLon bool
			for _, a := range t.Attr {
				if a.Name.Space != "" || (a.Name.Local != "lat" && a.Name.Local != "lon") {
					continue
				}
				v, e := strconv.ParseFloat(a.Value, 64)
				if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
					return out, fail("decode_failed", "invalid GPX coordinate")
				}
				if a.Name.Local == "lat" {
					lat = v
					haveLat = true
				} else {
					lon = v
					haveLon = true
				}
			}
			if !haveLat || !haveLon || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
				return out, fail("decode_failed", "GPX coordinate outside geographic bounds")
			}
			count++
			if count > maxGPXPoints {
				return out, fail("limit_exceeded", "GPX exceeds point limit")
			}
			segment = append(segment, [2]float64{lon, lat})
			if t.Name.Local == "wpt" {
				flush()
			}
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(t)) != "" {
				return out, fail("decode_failed", "text outside GPX root")
			}
		case xml.EndElement:
			if len(stack) == 3 && stack[1].Local == "trk" && t.Name.Local == "trkseg" || len(stack) == 2 && t.Name.Local == "rte" {
				flush()
			}
			stack = stack[:len(stack)-1]
		}
	}
	if !rootSeen || len(stack) != 0 || count == 0 {
		return out, fail("decode_failed", "GPX contains no complete coordinates")
	}
	return out, nil
}

func renderGPX(ctx context.Context, input, output string) (Result, error) {
	f, err := os.Open(input)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return Result{}, err
	}
	if stat.Size() > maxGPXBytes {
		return Result{}, fail("limit_exceeded", "GPX exceeds input limit")
	}
	data, err := parseGPX(ctx, f)
	if err != nil {
		return Result{}, err
	}
	dst, err := os.Create(output)
	if err != nil {
		return Result{}, err
	}
	err = json.NewEncoder(dst).Encode(data)
	if err == nil {
		err = dst.Sync()
	}
	closeErr := dst.Close()
	if err == nil {
		err = closeErr
	}
	return Result{Engine: "go-gpx-1"}, err
}
