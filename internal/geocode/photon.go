// Package geocode resolves capture coordinates without uploading file content.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultEndpoint = "https://photon.komoot.io/reverse"

type Point struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Accuracy  *float64 `json:"accuracy,omitempty"`
}

// Coordinates accepts EXIF decimal degrees and explicit hemisphere references.
// Unknown datums are not silently plotted as WGS84.
func Coordinates(values map[string]string) *Point {
	datum := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(values["gps:MapDatum"]), " ", ""))
	if datum != "" && datum != "WGS84" && datum != "WGS-84" {
		return nil
	}
	coordinate := func(key, positive, negative string, max float64) (float64, bool) {
		raw, exists := values[key]
		n, err := strconv.ParseFloat(raw, 64)
		if !exists || err != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > max {
			return 0, false
		}
		ref := strings.ToUpper(strings.TrimSpace(values[key+"Ref"]))
		switch ref {
		case negative:
			n = -math.Abs(n)
		case positive:
			if n < 0 {
				return 0, false
			}
		case "":
		default:
			return 0, false
		}
		return n, true
	}
	lat, a := coordinate("gps:Latitude", "N", "S", 90)
	lon, b := coordinate("gps:Longitude", "E", "W", 180)
	if !a || !b {
		return nil
	}
	p := &Point{Latitude: lat, Longitude: lon}
	raw := values["gps:HPositioningError"]
	if parts := strings.Split(raw, "/"); len(parts) == 2 {
		a, e1 := strconv.ParseFloat(parts[0], 64)
		b, e2 := strconv.ParseFloat(parts[1], 64)
		if e1 == nil && e2 == nil && b != 0 {
			raw = strconv.FormatFloat(a/b, 'g', -1, 64)
		}
	}
	if n, e := strconv.ParseFloat(raw, 64); e == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 {
		p.Accuracy = &n
	}
	return p
}

type Result struct {
	Address string `json:"address"`
}
type Provider interface {
	Reverse(context.Context, Point) (Result, error)
}
type Photon struct {
	Endpoint string
	Client   *http.Client
}
type RetryError struct{ After time.Duration }

func (e *RetryError) Error() string { return "location service is busy; retry later" }

func ValidateEndpoint(endpoint string) error {
	u, e := url.Parse(endpoint)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid Photon endpoint")
	}
	return nil
}

func (p Photon) Reverse(ctx context.Context, point Point) (Result, error) {
	if err := ValidateEndpoint(p.Endpoint); err != nil {
		return Result{}, err
	}
	u, _ := url.Parse(p.Endpoint)
	q := u.Query()
	q.Set("lat", strconv.FormatFloat(point.Latitude, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(point.Longitude, 'f', -1, 64))
	q.Set("lang", "default")
	q.Set("limit", "5")
	q.Set("radius", "1")
	q.Add("layer", "street")
	q.Add("layer", "locality")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "kist (https://github.com/jinzhao1994/kist)")
	req.Header.Set("Accept", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	// Redirects would bypass per-request throttling and disclose coordinates to another host.
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := bounded.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, errors.New("location service request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == 429 || response.StatusCode == 503 {
		return Result{}, &RetryError{After: retryAfter(response.Header.Get("Retry-After"), time.Now())}
	}
	if response.StatusCode != 200 {
		return Result{}, fmt.Errorf("location service returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return Result{}, errors.New("invalid location response")
	}
	var data struct {
		Type     string `json:"type"`
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if json.Unmarshal(raw, &data) != nil || data.Type != "FeatureCollection" {
		return Result{}, errors.New("invalid location response")
	}
	for _, f := range data.Features {
		prop := func(k string) string { s, _ := f.Properties[k].(string); return strings.TrimSpace(s) }
		kind := prop("type")
		if kind != "street" && kind != "locality" {
			continue
		}
		parts := []string{}
		seen := map[string]bool{}
		// No POI or house number: Photon extents cannot prove building containment.
		for _, key := range []string{"country", "state", "county", "city", "district", "locality", "name"} {
			s := prop(key)
			if s != "" && !seen[s] {
				parts = append(parts, s)
				seen[s] = true
			}
		}
		if len(parts) > 0 {
			address := strings.Join(parts, " · ")
			if len(address) > 1024 {
				return Result{}, errors.New("location response is too long")
			}
			return Result{Address: address}, nil
		}
	}
	return Result{}, nil
}
func retryAfter(value string, now time.Time) time.Duration {
	delay := time.Minute
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds > 0 {
		delay = time.Duration(seconds) * time.Second
	} else if until, err := http.ParseTime(value); err == nil && until.After(now) {
		delay = until.Sub(now)
	}
	if delay < time.Minute {
		delay = time.Minute
	}
	return delay
}
