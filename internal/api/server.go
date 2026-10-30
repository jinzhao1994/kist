// Package api serves the versioned HTTP contract. All state changes go through
// library, including request deduplication and crash recovery.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"path"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jinzhao1994/kist/internal/ai/openai"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
)

const Prefix = "/api/v1"
const maxJSONBody = 1 << 20

// Options declares trusted authorities. Hosts include ports; wildcard hosts
// are deliberately unsupported. AllowedOrigins are full scheme/host origins.
type Options struct {
	ResolveSettings  func(config.Server) (Options, error)
	ServiceSettings  *config.Server
	SettingOverrides []string
	ClientRenderer   library.ClientRenderer
	AllowedHosts     []string
	AllowedOrigins   []string
	MaxUploadBytes   int64
	MaxUploadFiles   int
	UploadTempDir    string
	UI               http.Handler
	MapProvider      *MapProvider
}

type Server struct {
	aiMu             sync.Mutex
	aiAttempt        *openai.Attempt
	aiError          string
	autoSyncRunning  atomic.Int64
	syncProgress     atomic.Pointer[library.ClientSyncProgress]
	autoSyncWake     chan struct{}
	current          atomic.Pointer[runtimeSettings]
	reloadMu         sync.Mutex
	resolveSettings  func(config.Server) (Options, error)
	baseHosts        []string
	startupSettings  map[string]any
	settingOverrides []string
	library          *library.Library
	client           *library.Client
	media            *library.Media
	video            *library.VideoPlayback
	previews         mediaBackend
	clientRenderer   library.ClientRenderer
	mux              *http.ServeMux
	contract         huma.API
	uploadTempDir    string
	historySlots     chan struct{}
	uploadSlots      chan struct{}
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
	s := &Server{autoSyncWake: make(chan struct{}, 1), uploadTempDir: opt.UploadTempDir, library: l, client: c, mux: http.NewServeMux(), resolveSettings: opt.ResolveSettings, baseHosts: append([]string(nil), opt.AllowedHosts...)}
	runtime, err := s.buildRuntime(opt)
	if err != nil {
		return nil, err
	}
	s.current.Store(runtime)
	s.historySlots = make(chan struct{}, 2)
	s.uploadSlots = make(chan struct{}, staging.MaxSessions)
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
	cfg.OpenAPIPath = Prefix + "/openapi"
	cfg.DocsPath = "" // An embedded UI will link to the offline JSON contract.
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	cfg.Transformers = nil // Keep wire JSON identical to the CLI: no injected $schema.
	s.contract = humago.New(s.mux, cfg)
	s.aiRoutes()
	s.mapRoutes()
	if err := s.settingsRoutes(opt); err != nil {
		return nil, err
	}
	if c != nil {
		s.clientReadRoutes()
		s.clientLocalHistoryRoutes()
		s.historyRoutes()
		s.historicalRoutes()
		s.clientRenderer = opt.ClientRenderer
		s.previews = c.Media(func(ctx context.Context) (library.DerivedSource, error) { return s.clientPeer(ctx) }, opt.ClientRenderer)
		s.mediaRoutes()
		s.clientManagementRoutes()
		s.clientSyncRoutes()
		s.relayRoutes()
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
		s.video = l.VideoPlayback()
		s.previews = nasMedia{s.media}
		s.contentRoutes()
		s.textRoutes()
		s.sheetRoutes()
		s.mediaRoutes()
		s.videoRoutes()
		s.uploadRoutes()
		s.uploadSessionRoutes()
		s.extractionRoutes()
		s.writeRoutes()
		s.syncRoutes()
	}
	s.metadataEditRoutes()
	s.navigationRoutes()
	s.catalogRevisionRoute()
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
	if requiresLocalAI(r.URL.Path) && !localAIRequest(r.RemoteAddr, r.Host) {
		writeFailure(w, huma.Error403Forbidden("AI connections and OCR require local access"))
		return
	}

	runtime := s.current.Load()
	r = r.WithContext(context.WithValue(r.Context(), runtimeKey{}, runtime))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; worker-src 'self'; font-src 'self' blob:; img-src 'self' blob: data:; media-src 'self' blob:; connect-src "+runtime.mapConnect)
	if !runtime.hosts[strings.ToLower(r.Host)] {
		writeFailure(w, huma.Error403Forbidden("Host is not allowed"))
		return
	}
	origin := r.Header.Get("Origin")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	sameOrigin := scheme + "://" + r.Host
	if origin != "" && origin != sameOrigin && !runtime.origins[origin] {
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
	if strings.HasPrefix(path.Clean(r.URL.Path), Prefix+"/sync/") {
		if !s.authorizeSync(w, r) {
			return
		}
		routed, err := s.syncRouteContext(r)
		if err != nil {
			writeFailure(w, huma.Error409Conflict("Cyclic or invalid relay route"))
			return
		}
		r = r.WithContext(routed)
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
			r.Body = http.MaxBytesReader(w, r.Body, runtime.maxUploadBytes)
			s.mux.ServeHTTP(w, r)
			return
		}
		if err != nil || media != "application/json" {
			writeFailure(w, huma.Error415UnsupportedMediaType("Use application/json"))
			return
		}
		limit := int64(maxJSONBody)
		if path.Clean(r.URL.Path) == Prefix+"/sync/submit" || path.Clean(r.URL.Path) == Prefix+"/sync/submit-conditional" || path.Clean(r.URL.Path) == Prefix+"/sync/relay/submit" {
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

// Match model invocation routes exactly; nested metadata writes do not call AI.
func requiresLocalAI(path string) bool {
	if path == Prefix+"/ai" || strings.HasPrefix(path, Prefix+"/ai/") {
		return true
	}
	rest, ok := strings.CutPrefix(path, Prefix+"/files/")
	if !ok {
		return false
	}
	parts := strings.Split(rest, "/")
	return len(parts) == 2 && (parts[1] == "ocr" || parts[1] == "desc" || parts[1] == "orientation")
}

// localAIRequest is shared by dedicated AI routes and combined settings writes.
func localAIRequest(remote, requestHost string) bool {
	host, _, err := net.SplitHostPort(remote)
	ip := net.ParseIP(host)
	hostName, _, hostErr := net.SplitHostPort(requestHost)
	if hostErr != nil {
		hostName = requestHost
	}
	hostIP := net.ParseIP(hostName)
	return err == nil && ip != nil && ip.IsLoopback() && (hostName == "localhost" || (hostIP != nil && hostIP.IsLoopback()))
}
