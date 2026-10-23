package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// MapProvider describes browser-only map resources. No GPX data is sent to
// these services. URLs are deployment options, never read from uploaded files.
type MapProvider struct {
	StyleURL           string   `json:"style_url"`
	TerrainTiles       string   `json:"terrain_tiles"`
	TerrainAttribution string   `json:"terrain_attribution"`
	ResourceOrigins    []string `json:"resource_origins"`
}

func DefaultMapProvider() MapProvider {
	return MapProvider{
		StyleURL:           "https://tiles.openfreemap.org/styles/liberty",
		TerrainTiles:       "https://elevation-tiles-prod.s3.amazonaws.com/terrarium/{z}/{x}/{y}.png",
		TerrainAttribution: `<a href="https://www.mapzen.com/rights/">Mapzen terrain</a>`,
		ResourceOrigins:    []string{},
	}
}

func (p MapProvider) connectSources() (string, error) {
	if p.StyleURL == "" {
		return "", fmt.Errorf("map style URL cannot be empty")
	}
	origins := []string{"'self'"}
	for _, raw := range append([]string{p.StyleURL, p.TerrainTiles}, p.ResourceOrigins...) {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\t ;'\"\\") {
			return "", fmt.Errorf("invalid map resource URL %q", raw)
		}
		if u.Scheme == "" && u.Host == "" && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
			continue
		}
		if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || strings.ContainsAny(u.Host, "*{}") {
			return "", fmt.Errorf("map resources require HTTP(S) or an absolute same-origin path")
		}
		origin := u.Scheme + "://" + u.Host
		found := false
		for _, v := range origins {
			if v == origin {
				found = true
			}
		}
		if !found {
			origins = append(origins, origin)
		}
	}
	return strings.Join(origins, " "), nil
}

func (s *Server) mapRoutes() {
	type output struct{ Body MapProvider }
	huma.Get(s.contract, Prefix+"/map-provider", func(ctx context.Context, _ *struct{}) (*output, error) {
		return &output{Body: s.runtime(ctx).provider}, nil
	})
}
