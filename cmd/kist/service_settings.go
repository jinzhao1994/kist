package main

import (
	"github.com/jinzhao1994/kist/internal/api"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/spf13/cobra"
)

// Explicit flags win without rewriting the user's persisted configuration.
func resolveServiceSettings(cmd *cobra.Command, stored config.Server) (config.Server, []string, error) {
	overrides := []string{}
	stringFlags := map[string]*string{"listen": &stored.Listen, "map-style": &stored.MapStyle, "map-terrain-tiles": &stored.MapTerrainTiles, "map-terrain-attribution": &stored.MapTerrainAttribution}
	keys := map[string]string{"listen": "listen", "allow-origin": "allowed_origins", "max-upload-bytes": "max_upload_bytes", "max-upload-files": "max_upload_files", "map-style": "map_style", "map-terrain-tiles": "map_terrain_tiles", "map-terrain-attribution": "map_terrain_attribution", "map-resource-origin": "map_resource_origins"}
	for flag, target := range stringFlags {
		if cmd.Flags().Changed(flag) {
			value, err := cmd.Flags().GetString(flag)
			if err != nil {
				return stored, nil, err
			}
			*target = value
			overrides = append(overrides, "server."+keys[flag])
		}
	}
	for flag, target := range map[string]*[]string{"allow-origin": &stored.AllowedOrigins, "map-resource-origin": &stored.MapResourceOrigins} {
		if cmd.Flags().Changed(flag) {
			value, err := cmd.Flags().GetStringSlice(flag)
			if err != nil {
				return stored, nil, err
			}
			*target = value
			overrides = append(overrides, "server."+keys[flag])
		}
	}
	if cmd.Flags().Changed("max-upload-bytes") {
		v, err := cmd.Flags().GetInt64("max-upload-bytes")
		if err != nil {
			return stored, nil, err
		}
		stored.MaxUploadBytes = v
		overrides = append(overrides, "server.max_upload_bytes")
	}
	if cmd.Flags().Changed("max-upload-files") {
		v, err := cmd.Flags().GetInt("max-upload-files")
		if err != nil {
			return stored, nil, err
		}
		stored.MaxUploadFiles = v
		overrides = append(overrides, "server.max_upload_files")
	}
	return stored, overrides, nil
}
func applyServiceSettings(opt *api.Options, settings config.Server, overrides []string) {
	opt.ServiceSettings = &settings
	opt.SettingOverrides = overrides
	opt.MaxUploadBytes = settings.MaxUploadBytes
	opt.MaxUploadFiles = settings.MaxUploadFiles
	opt.MapProvider = &api.MapProvider{StyleURL: settings.MapStyle, TerrainTiles: settings.MapTerrainTiles, TerrainAttribution: settings.MapTerrainAttribution, ResourceOrigins: settings.MapResourceOrigins}
}

// reloadResolver preserves the actual listener while reapplying explicit flags.
func reloadResolver(cmd *cobra.Command, listen string) func(config.Server) (api.Options, error) {
	return func(stored config.Server) (api.Options, error) {
		effective, overrides, err := resolveServiceSettings(cmd, stored)
		if err != nil {
			return api.Options{}, err
		}
		effective.Listen = listen
		opt, err := serveOptions(effective.Listen, effective.AllowedOrigins)
		if err != nil {
			return opt, err
		}
		applyServiceSettings(&opt, effective, overrides)
		return opt, nil
	}
}
