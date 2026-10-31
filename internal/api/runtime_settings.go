package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
	staging "github.com/jinzhao1994/kist/internal/upload"
)

type runtimeKey struct{}
type runtimeSettings struct {
	hosts, origins       map[string]bool
	mapConnect           string
	provider             MapProvider
	maxUploadBytes       int64
	maxUploadFiles       int
	uploads, syncUploads *staging.Store
	service              map[string]any
}

func (s *Server) runtime(ctx context.Context) *runtimeSettings {
	if runtime, ok := ctx.Value(runtimeKey{}).(*runtimeSettings); ok {
		return runtime
	}
	return s.current.Load()
}
func (s *Server) buildRuntime(opt Options) (*runtimeSettings, error) {
	if (s.library == nil && s.client == nil) || len(opt.AllowedHosts) == 0 {
		return nil, fmt.Errorf("a library and explicit allowed hosts are required")
	}
	if opt.MaxUploadBytes == 0 {
		opt.MaxUploadBytes = DefaultMaxUploadBytes
	}
	if opt.MaxUploadFiles == 0 {
		opt.MaxUploadFiles = DefaultMaxUploadFiles
	}
	r := &runtimeSettings{hosts: map[string]bool{}, origins: map[string]bool{}, maxUploadBytes: opt.MaxUploadBytes, maxUploadFiles: opt.MaxUploadFiles, provider: DefaultMapProvider(), service: map[string]any{}}
	if opt.MapProvider != nil {
		r.provider = *opt.MapProvider
	}
	var err error
	r.mapConnect, err = r.provider.connectSources()
	if err != nil {
		return nil, err
	}
	for _, host := range opt.AllowedHosts {
		u, err := url.Parse("http://" + host)
		if err != nil || u.Host != host || u.Hostname() == "" || u.User != nil || u.Path != "" || strings.ContainsAny(host, "*?#\\") {
			return nil, fmt.Errorf("invalid allowed host")
		}
		r.hosts[strings.ToLower(host)] = true
	}
	for _, origin := range opt.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("invalid allowed origin")
		}
		r.origins[u.Scheme+"://"+u.Host] = true
	}
	var stagingStore func(bool, int64, int) (*staging.Store, error)
	if s.library != nil {
		stagingStore = s.library.UploadStaging
	} else {
		stagingStore = s.client.UploadStaging
	}
	r.uploads, err = stagingStore(false, r.maxUploadBytes, r.maxUploadFiles)
	if err != nil {
		return nil, err
	}
	r.syncUploads, err = stagingStore(true, r.maxUploadBytes, 1)
	if err != nil {
		return nil, err
	}
	if opt.ServiceSettings != nil {
		raw, _ := json.Marshal(opt.ServiceSettings)
		_ = json.Unmarshal(raw, &r.service)
	}
	return r, nil
}

// prepareSettings keeps the bound listener and explicit command-line overrides.
func (s *Server) prepareSettings(stored config.Server) (*runtimeSettings, error) {
	if s.resolveSettings != nil {
		opt, err := s.resolveSettings(stored)
		if err != nil {
			return nil, err
		}
		return s.buildRuntime(opt)
	}
	// Embedded users without command-line flags retain their original hosts.
	hosts := append([]string(nil), s.baseHosts...)
	for _, origin := range stored.AllowedOrigins {
		u, err := url.Parse(origin)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, u.Host)
	}
	stored.Listen, _ = s.startupSettings["server.listen"].(string)
	return s.buildRuntime(Options{ServiceSettings: &stored, AllowedHosts: hosts, AllowedOrigins: stored.AllowedOrigins, MaxUploadBytes: stored.MaxUploadBytes, MaxUploadFiles: stored.MaxUploadFiles, MapProvider: &MapProvider{StyleURL: stored.MapStyle, TerrainTiles: stored.MapTerrainTiles, TerrainAttribution: stored.MapTerrainAttribution, ResourceOrigins: stored.MapResourceOrigins}})
}

// ReloadSettings is shared by the signal handler and the settings write path.
// All fallible preparation finishes before the runtime pointer is replaced.
func (s *Server) ReloadSettings(ctx context.Context) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	return s.reloadSettings(ctx, "", nil)
}
func (s *Server) reloadSettings(ctx context.Context, revision string, changes map[string]any) error {
	return s.reloadCombinedSettings(ctx, revision, changes, nil, nil)
}

func (s *Server) reloadCombinedSettings(ctx context.Context, revision string, changes map[string]any, ai *library.OCRSettings, location *library.LocationSettings) error {
	return s.reloadCombinedServiceSettings(ctx, revision, changes, ai, location, nil)
}

func (s *Server) reloadCombinedServiceSettings(ctx context.Context, revision string, changes map[string]any, ai *library.OCRSettings, location *library.LocationSettings, services *library.AIServiceSettings) error {
	_, err := s.reloadCombinedDraftSettings(ctx, revision, changes, ai, location, services, nil)
	return err
}

func (s *Server) reloadCombinedDraftSettings(ctx context.Context, revision string, changes map[string]any, ai *library.OCRSettings, location *library.LocationSettings, services *library.AIServiceSettings, credentials *library.CredentialSettings) ([]library.NewSyncCredential, error) {
	if s.client != nil && (ai != nil || location != nil || services != nil) {
		return nil, fmt.Errorf("metadata settings require a library")
	}
	afterCommit := s.prepareAISignInChange(services)
	finishAuthorization, err := s.authorizeSignInDrafts(services)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() { finishAuthorization(committed) }()

	var next *runtimeSettings
	validate := func(stored config.Server) (err error) { next, err = s.prepareSettings(stored); return }
	var created []library.NewSyncCredential
	if changes == nil {
		if s.client != nil {
			err = s.client.ReloadSettings(ctx, validate)
		} else {
			err = s.library.ReloadSettings(ctx, validate)
		}
	} else if s.client != nil {
		created, err = s.client.UpdateSettingsWithCredentials(ctx, revision, changes, credentials, validate)
	} else {
		created, err = s.library.UpdateCombinedDraftSettings(ctx, revision, changes, ai, location, services, credentials, validate)
	}
	if err != nil {
		return nil, err
	}
	s.current.Store(next)
	committed = true
	afterCommit()
	return created, nil
}
