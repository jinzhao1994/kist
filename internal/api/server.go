// Package api serves the versioned HTTP contract. All state changes go through
// library, including request deduplication and crash recovery.
package api

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/library"
)

const Prefix = "/api/v1"
const maxJSONBody = 1 << 20

// Options declares trusted authorities. Hosts include ports; wildcard hosts
// are deliberately unsupported. AllowedOrigins are full scheme/host origins.
type Options struct {
	AllowedHosts   []string
	AllowedOrigins []string
	MaxUploadBytes int64
	MaxUploadFiles int
	UploadTempDir  string
}

type Server struct {
	library        *library.Library
	mux            *http.ServeMux
	contract       huma.API
	hosts          map[string]bool
	origins        map[string]bool
	maxUploadBytes int64
	maxUploadFiles int
	uploadTempDir  string
}

func New(l *library.Library, opt Options) (*Server, error) {
	if opt.MaxUploadBytes == 0 {
		opt.MaxUploadBytes = DefaultMaxUploadBytes
	}
	if opt.MaxUploadFiles == 0 {
		opt.MaxUploadFiles = DefaultMaxUploadFiles
	}
	if opt.MaxUploadBytes < 1 || opt.MaxUploadFiles < 1 {
		return nil, fmt.Errorf("upload limits must be positive")
	}
	s := &Server{maxUploadBytes: opt.MaxUploadBytes, maxUploadFiles: opt.MaxUploadFiles, uploadTempDir: opt.UploadTempDir, library: l, mux: http.NewServeMux(), hosts: map[string]bool{}, origins: map[string]bool{}}
	if l == nil || len(opt.AllowedHosts) == 0 {
		return nil, fmt.Errorf("a library and explicit allowed hosts are required")
	}
	for _, host := range opt.AllowedHosts {
		u, err := url.Parse("http://" + host)
		if err != nil || u.Host != host || u.Hostname() == "" || u.User != nil || u.Path != "" || strings.ContainsAny(host, "*?#\\") {
			return nil, fmt.Errorf("invalid allowed host %q", host)
		}
		s.hosts[strings.ToLower(host)] = true
	}
	for _, origin := range opt.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("invalid allowed origin %q", origin)
		}
		s.origins[origin] = true
	}
	cfg := huma.DefaultConfig("kist API", "1.0.0")
	cfg.OpenAPIPath = Prefix + "/openapi"
	cfg.DocsPath = "" // An embedded UI will link to the offline JSON contract.
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	cfg.Transformers = nil // Keep wire JSON identical to the CLI: no injected $schema.
	s.contract = humago.New(s.mux, cfg)
	s.readRoutes()
	s.fileRoutes()
	s.tagRoutes()
	s.contentRoutes()
	s.uploadRoutes()
	s.extractionRoutes()
	s.writeRoutes()
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeFailure(w, huma.Error404NotFound("No such endpoint"))
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
	if !s.hosts[strings.ToLower(r.Host)] {
		writeFailure(w, huma.Error403Forbidden("Host is not allowed"))
		return
	}
	origin := r.Header.Get("Origin")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	sameOrigin := scheme + "://" + r.Host
	if origin != "" && origin != sameOrigin && !s.origins[origin] {
		writeFailure(w, huma.Error403Forbidden("Origin is not allowed"))
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" && origin == "" {
		writeFailure(w, huma.Error403Forbidden("Cross-site access is not allowed"))
		return
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || (r.Method == "DELETE" && r.ContentLength != 0) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		upload := r.Method == "POST" && strings.HasPrefix(r.URL.Path, Prefix+"/boxes/") && strings.HasSuffix(r.URL.Path, "/uploads")
		if upload && err == nil && media == "multipart/form-data" {
			r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
			s.mux.ServeHTTP(w, r)
			return
		}
		if err != nil || media != "application/json" {
			writeFailure(w, huma.Error415UnsupportedMediaType("Use application/json"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	}
	s.mux.ServeHTTP(w, r)
}

func writeFailure(w http.ResponseWriter, err error) {
	failure := failure(err).(*HTTPError)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(failure.Status)
	_ = json.NewEncoder(w).Encode(failure)
}

type output[T any] struct{ Body T }

func response[T any](body T, err error) (*output[T], error) {
	if err != nil {
		return nil, failure(err)
	}
	return &output[T]{Body: body}, nil
}
