package api

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIOnlyAcceptsLoopbackRequests(t *testing.T) {
	s, _ := testServer(t)
	for _, tc := range []struct {
		addr, host string
		code       int
	}{{"192.0.2.1:1234", "localhost:8473", 403}, {"127.0.0.1:1234", "localhost:8473", 200}, {"127.0.0.1:1234", "archive.example:8473", 403}} {
		req := httptest.NewRequest("GET", "http://"+tc.host+Prefix+"/ai", nil)
		req.RemoteAddr = tc.addr
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "access_token") {
			t.Fatal("secret field exposed")
		}
	}
}

func TestAISettingsAndQueueRoutes(t *testing.T) {
	s, _ := testServer(t)
	callLocal := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	settings := callLocal("GET", "/ai/settings", "")
	if settings.Code != 200 {
		t.Fatal(settings.Code, settings.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(settings.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	value["model"] = "gpt-6-luna"
	value["auto_ocr"] = true
	value["auto_desc"] = true
	value["auto_orientation"] = true
	value["proxy_url"] = "http://127.0.0.1:7890"
	body, _ := json.Marshal(value)
	if got := callLocal("PUT", "/ai/settings", string(body)); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := callLocal("PUT", "/ai/settings", string(body)); got.Code != 409 {
		t.Fatal("stale update", got.Code)
	}
	for _, test := range []struct{ method, path, body string }{{"GET", "/ai/desc", ""}, {"POST", "/ai/desc/pause", `{"paused":true}`}, {"POST", "/ai/desc/backfill", `{"retry":false}`}, {"GET", "/ai/ocr", ""}, {"POST", "/ai/ocr/pause", `{"paused":true}`}, {"POST", "/ai/ocr/backfill", `{"retry":false}`}} {
		if got := callLocal(test.method, test.path, test.body); got.Code != 200 {
			t.Fatal(test.path, got.Code, got.Body.String())
		}
	}
}

func TestDescriptionFileRouteIsLocalOnly(t *testing.T) {
	s, _ := testServer(t)
	for _, method := range []string{"GET", "POST"} {
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+"/files/01a1737a-ceb0-7392-a1a9-a5ecb0b7429e/desc", strings.NewReader(`{"retry":false}`))
		r.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

// Exercise the real HTTP validator; mocked browser responses cannot catch
// missing required fields before a refresh reaches its queue.
func TestAIReextractHTTPContract(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "refresh", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	body, media := multipartUpload(t, []uploadFile{{Path: "sample.png", LastModified: 1600000000000}}, []uploadFixture{{"sample.png", imageData.String()}})
	if w := upload(t, s, box.ID.String(), "refresh-fixture", body, media); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	files := decodeBody[wire.FileList](t, s, "/files")
	cfg, err := l.OCRSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Model = "test"
	if _, err := l.SaveOCRSettings(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"ocr", "desc", "orientation"} {
		key := uuid.NewString()
		send := func(payload string) *httptest.ResponseRecorder {
			r := httptest.NewRequest("POST", "http://localhost:8473"+Prefix+"/files/"+files.Files[0].ID+"/"+kind, strings.NewReader(payload))
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			return w
		}
		missing, _ := json.Marshal(map[string]any{"force": true, "request_id": key})
		if w := send(string(missing)); w.Code != 422 {
			t.Fatal("missing retry should fail validation", w.Code, w.Body)
		}
		valid, _ := json.Marshal(map[string]any{"force": true, "retry": false, "request_id": key})
		for range 2 {
			w := send(string(valid))
			if w.Code != 200 {
				t.Fatal(kind, w.Code, w.Body)
			}
			var job library.OCRJob
			if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
				t.Fatal(err)
			}
			if job.State != "queued" || !job.Force || job.RequestID != key {
				t.Fatal(job)
			}
		}
	}
}

func TestManualImageRotationHTTPContract(t *testing.T) {
	s, l := testServer(t)
	box, err := l.CreateBox(t.Context(), "rotate", "", domain.LocalDate{}, domain.LocalDate{})
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 4, 2)))
	body, media := multipartUpload(t, []uploadFile{{Path: "direction.png", LastModified: 1600000000000}}, []uploadFixture{{"direction.png", raw.String()}})
	if w := upload(t, s, box.ID.String(), "rotation-fixture", body, media); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	f := decodeBody[wire.FileList](t, s, "/files").Files[0]
	path := Prefix + "/files/" + f.ID + "/image-rotation"
	for _, angle := range []any{90, nil, 0} {
		body, _ := json.Marshal(map[string]any{"blob_key": f.BlobKey, "clockwise_degrees": angle})
		key := uuid.NewString()
		for range 2 {
			w := call(s, "PUT", path, key, string(body))
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
		}
		got := decodeBody[wire.FileList](t, s, "/files").Files[0]
		if angle == nil {
			if got.ImageRotationManual {
				t.Fatal("manual direction not released")
			}
		} else if !got.ImageRotationManual || got.ImageRotation != angle.(int) {
			t.Fatal(got)
		}
	}
}

func TestLocationPolicyHTTPRoundTrip(t *testing.T) {
	s, _ := testServer(t)
	call := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://localhost:8473"+Prefix+"/geo/settings", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	var data map[string]any
	read := func() {
		t.Helper()
		w := call("GET", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if data["request_interval_seconds"] != float64(2) || data["pause_on_error"] != true {
		t.Fatal("incorrect defaults", data)
	}
	data["request_interval_seconds"] = 0
	data["pause_on_error"] = false
	raw, _ := json.Marshal(data)
	w := call("PUT", string(raw))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	read()
	if data["request_interval_seconds"] != float64(0) || data["pause_on_error"] != false {
		t.Fatal("explicit zero/false replaced by defaults", data)
	}
	data["request_interval_seconds"] = -1
	raw, _ = json.Marshal(data)
	if w = call("PUT", string(raw)); w.Code < 400 {
		t.Fatal("accepted negative interval")
	}
	read()
	if data["request_interval_seconds"] != float64(0) {
		t.Fatal("invalid settings were persisted")
	}
}
