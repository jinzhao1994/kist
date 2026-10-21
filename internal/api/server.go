// Package api serves the versioned HTTP contract. All state changes go through
// library, including request deduplication and crash recovery.
package api

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
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
	UI             http.Handler
	MapProvider    *MapProvider
}

type Server struct {
	mapConnect     string
	library        *library.Library
	client         *library.Client
	media          *library.Media
	mux            *http.ServeMux
	contract       huma.API
	hosts          map[string]bool
	origins        map[string]bool
	maxUploadBytes int64
	maxUploadFiles int
	uploadTempDir  string
	historySlots   chan struct{}
	uploads        *staging.Store
	syncUploads    *staging.Store
	uploadSlots    chan struct{}
	// uploadFault is immutable while serving and used only by fault-injection tests.
	uploadFault func(string) error
}

func New(l *library.Library, opt Options) (*Server, error) {
	return newServer(l, nil, opt)
}

// NewClient serves the local editing catalog with the same HTTP protections.
func NewClient(c *library.Client, opt Options) (*Server, error) {
	return newServer(nil, c, opt)
}

func newServer(l *library.Library, c *library.Client, opt Options) (*Server, error) {
	if opt.MaxUploadBytes == 0 {
		opt.MaxUploadBytes = DefaultMaxUploadBytes
	}
	if opt.MaxUploadFiles == 0 {
		opt.MaxUploadFiles = DefaultMaxUploadFiles
	}
	if opt.MaxUploadBytes < 1 || opt.MaxUploadFiles < 1 {
		return nil, fmt.Errorf("upload limits must be positive")
	}
	s := &Server{maxUploadBytes: opt.MaxUploadBytes, maxUploadFiles: opt.MaxUploadFiles, uploadTempDir: opt.UploadTempDir, library: l, client: c, mux: http.NewServeMux(), hosts: map[string]bool{}, origins: map[string]bool{}}
	provider := DefaultMapProvider()
	if opt.MapProvider != nil {
		provider = *opt.MapProvider
	}
	connect, mapErr := provider.connectSources()
	if mapErr != nil {
		return nil, mapErr
	}
	s.mapConnect = connect
	s.historySlots = make(chan struct{}, 2)
	if (l == nil && c == nil) || len(opt.AllowedHosts) == 0 {
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
	cfg.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", func(t reflect.Type, hint string) string {
		name := huma.DefaultSchemaNamer(t, hint)
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.PkgPath() == reflect.TypeFor[staging.File]().PkgPath() {
			return "Upload" + name
		}
		return name
	})
	var err error
	if l != nil {
		s.uploads, err = staging.New(filepath.Join(l.Layout().Root, "uploads"), opt.MaxUploadBytes, opt.MaxUploadFiles)
		if err != nil {
			return nil, err
		}
		s.uploadSlots = make(chan struct{}, staging.MaxSessions)
		s.syncUploads, err = staging.New(filepath.Join(l.Layout().Root, "sync-uploads"), opt.MaxUploadBytes, 1)
		if err != nil {
			return nil, err
		}
	}
	if c != nil {
		s.uploads, err = staging.New(c.UploadStagingPath(), opt.MaxUploadBytes, opt.MaxUploadFiles)
		if err != nil {
			return nil, err
		}
		s.uploadSlots = make(chan struct{}, staging.MaxSessions)
	}
	cfg.OpenAPIPath = Prefix + "/openapi"
	cfg.DocsPath = "" // An embedded UI will link to the offline JSON contract.
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	cfg.Transformers = nil // Keep wire JSON identical to the CLI: no injected $schema.
	s.contract = humago.New(s.mux, cfg)
	s.mapRoutes(provider)
	if c != nil {
		s.clientReadRoutes()
		s.clientManagementRoutes()
		s.clientSyncRoutes()
		s.clientWriteRoutes()
		s.clientExtractionRoutes()
		s.textRoutes()
		s.sheetRoutes()
		s.uploadSessionRoutes()
		s.contentRoutes()
	} else {
		s.readRoutes()
		s.historyRoutes()
		s.historicalRoutes()
		s.fileRoutes()
		s.tagRoutes()
		s.savedSearchRoutes()
		s.filesetRoutes()
		s.media = l.Media()
		s.contentRoutes()
		s.textRoutes()
		s.sheetRoutes()
		s.mediaRoutes()
		s.uploadRoutes()
		s.uploadSessionRoutes()
		s.extractionRoutes()
		s.writeRoutes()
		s.syncRoutes()
	}
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if opt.UI != nil && r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
			opt.UI.ServeHTTP(w, r)
			return
		}
		writeFailure(w, huma.Error404NotFound("No such endpoint"))
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; worker-src 'self'; font-src 'self' blob:; img-src 'self' blob: data:; media-src 'self' blob:; connect-src "+s.mapConnect)
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
	// External links may open the UI shell, but never bypass API protection.
	uiPath := path.Clean(r.URL.Path)
	uiNavigation := r.Method == http.MethodGet &&
		r.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		r.Header.Get("Sec-Fetch-Dest") == "document" &&
		uiPath != "/api" && !strings.HasPrefix(uiPath, "/api/")
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" && origin == "" && !uiNavigation {
		writeFailure(w, huma.Error403Forbidden("Cross-site access is not allowed"))
		return
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Upload-SHA256, Upload-Length")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.library != nil && strings.HasPrefix(path.Clean(r.URL.Path), Prefix+"/sync/") && !s.authorizeSync(w, r) {
		return
	}
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || (r.Method == "DELETE" && r.ContentLength != 0) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		chunk := r.Method == "PUT" && (strings.HasPrefix(r.URL.Path, Prefix+"/upload-sessions/") || strings.HasPrefix(r.URL.Path, Prefix+"/sync/original-uploads/")) && strings.Contains(r.URL.Path, "/files/")
		if chunk && err == nil && media == "application/octet-stream" {
			r.Body = http.MaxBytesReader(w, r.Body, staging.ChunkBytes)
			s.mux.ServeHTTP(w, r)
			return
		}
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
		limit := int64(maxJSONBody)
		if path.Clean(r.URL.Path) == Prefix+"/sync/submit" {
			limit = maxSyncBody
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
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
