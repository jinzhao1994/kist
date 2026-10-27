package geocode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCoordinates(t *testing.T) {
	values := map[string]string{"gps:Latitude": "12.5", "gps:LatitudeRef": "S", "gps:Longitude": "45", "gps:LongitudeRef": "W", "gps:HPositioningError": "7/2"}
	p := Coordinates(values)
	if p == nil || p.Latitude != -12.5 || p.Longitude != -45 || p.Accuracy == nil || *p.Accuracy != 3.5 {
		t.Fatal(p)
	}
	for _, bad := range []string{"NaN", "Inf", "91", ""} {
		values["gps:Latitude"] = bad
		if Coordinates(values) != nil {
			t.Fatal(bad)
		}
	}
	values["gps:Latitude"] = "0"
	values["gps:Longitude"] = "0"
	if Coordinates(values) == nil {
		t.Fatal("valid zero coordinates rejected")
	}
	values["gps:MapDatum"] = "other"
	if Coordinates(values) != nil {
		t.Fatal("unknown datum accepted")
	}
}
func TestPhotonRequestAndPrecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("lang") != "default" || q.Get("lat") != "12" || q.Get("lon") != "34" || q.Get("radius") != "1" || len(q["layer"]) != 2 || r.Header.Get("User-Agent") == "" {
			t.Error(r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"properties":{"type":"house","name":"Private shop","housenumber":"99"}},{"properties":{"type":"street","country":"Example","state":"City","city":"City","district":"District","name":"Road","housenumber":"99"}}]}`))
	}))
	defer server.Close()
	accuracy := 1.0
	got, err := (Photon{Endpoint: server.URL, Client: server.Client()}).Reverse(t.Context(), Point{Latitude: 12, Longitude: 34, Accuracy: &accuracy})
	if err != nil || got.Address != "Example · City · District · Road" {
		t.Fatal(got, err)
	}
}
func TestPhotonErrors(t *testing.T) {
	for _, status := range []int{429, 503, 500, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "120")
				w.WriteHeader(status)
			}))
			defer server.Close()
			_, err := (Photon{Endpoint: server.URL}).Reverse(context.Background(), Point{})
			if err == nil {
				t.Fatal("accepted error")
			}
			var retry *RetryError
			if status == 429 || status == 503 {
				if !errors.As(err, &retry) || retry.After != 2*time.Minute {
					t.Fatal(err)
				}
			}
		})
	}
	now := time.Now().UTC().Truncate(time.Second)
	if retryAfter(now.Add(3*time.Minute).Format(http.TimeFormat), now) != 3*time.Minute {
		t.Fatal("HTTP date ignored")
	}
}
