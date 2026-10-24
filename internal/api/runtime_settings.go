package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jinzhao1994/kist/internal/config"
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
	var uploadPath string
	if s.library != nil {
		uploadPath = filepath.Join(s.library.Layout().Root, "uploads")
	} else {
		uploadPath = s.client.UploadStagingPath()
	}
	r.uploads, err = staging.New(uploadPath, r.maxUploadBytes, r.maxUploadFiles)
	if err != nil {
		return nil, err
	}
	{
		syncPath := filepath.Join(filepath.Dir(uploadPath), "sync-uploads")
		r.syncUploads, err = staging.New(syncPath, r.maxUploadBytes, 1)
		if err != nil {
			return nil, err
		}
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
	var next *runtimeSettings
	validate := func(stored config.Server) (err error) { next, err = s.prepareSettings(stored); return }
	var err error
	if changes == nil {
		if s.client != nil {
			err = s.client.ReloadSettings(ctx, validate)
		} else {
			err = s.library.ReloadSettings(ctx, validate)
		}
	} else if s.client != nil {
		err = s.client.UpdateSettings(ctx, revision, changes, validate)
	} else {
		err = s.library.UpdateSettings(ctx, revision, changes, validate)
	}
	if err != nil {
		return err
	}
	s.current.Store(next)
	return nil
}
