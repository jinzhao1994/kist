package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/library"
)

type settingsInput struct {
	Body struct {
		Revision string         `json:"revision" minLength:"1"`
		Changes  map[string]any `json:"changes"`
	}
}
type settingsResponse struct {
	library.InstanceSettings
	Effective       map[string]any `json:"effective"`
	Overrides       []string       `json:"overrides"`
	RestartRequired []string       `json:"restart_required"`
}

func (s *Server) currentSettings(ctx context.Context) (library.InstanceSettings, error) {
	if s.client != nil {
		return s.client.InstanceSettings(ctx)
	}
	return s.library.InstanceSettings(ctx)
}
func (s *Server) settingsView(ctx context.Context) (settingsResponse, error) {
	settings, err := s.currentSettings(ctx)
	if err != nil {
		return settingsResponse{}, err
	}
	out := settingsResponse{InstanceSettings: settings, Effective: map[string]any{}, Overrides: s.settingOverrides, RestartRequired: []string{}}
	runtime := s.current.Load()
	var timezone, projection string
	if s.library != nil {
		timezone, projection = s.library.RuntimeStorageSettings()
	}
	for _, f := range settings.Fields {
		v := f.Value
		if s.library != nil {
			switch f.Key {
			case "library.timezone":
				v = timezone
			case "storage.link_mode":
				v = projection
			}
		}
		if strings.HasPrefix(f.Key, "server.") {
			if effective, ok := runtime.service[strings.TrimPrefix(f.Key, "server.")]; ok {
				v = effective
			}
		}
		if f.Restart {
			v = s.startupSettings[f.Key]
			if !reflect.DeepEqual(jsonValue(v), jsonValue(f.Value)) {
				out.RestartRequired = append(out.RestartRequired, f.Key)
			}
		}
		out.Effective[f.Key] = v
	}
	return out, nil
}
func jsonValue(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Server) settingsRoutes(opt Options) error {
	initial, err := s.currentSettings(context.Background())
	if err != nil {
		return err
	}
	s.startupSettings = map[string]any{}
	s.settingOverrides = opt.SettingOverrides
	if s.settingOverrides == nil {
		s.settingOverrides = []string{}
	}
	for _, f := range initial.Fields {
		s.startupSettings[f.Key] = f.Value
	}
	if opt.ServiceSettings != nil {
		raw, _ := json.Marshal(opt.ServiceSettings)
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		for key, value := range fields {
			s.startupSettings["server."+key] = value
		}
	}
	huma.Get(s.contract, Prefix+"/settings", func(ctx context.Context, _ *struct{}) (*output[settingsResponse], error) {
		out, err := s.settingsView(ctx)
		return response(out, err)
	})
	huma.Get(s.contract, Prefix+"/settings/storage", func(ctx context.Context, _ *struct{}) (*output[library.StorageUsage], error) {
		if s.client != nil {
			usage, err := s.client.StorageUsage(ctx)
			return response(usage, err)
		}
		usage, err := s.library.StorageUsage(ctx)
		return response(usage, err)
	})

	huma.Put(s.contract, Prefix+"/settings", func(ctx context.Context, in *settingsInput) (*output[settingsResponse], error) {
		s.reloadMu.Lock()
		defer s.reloadMu.Unlock()
		changes := in.Body.Changes
		if changes == nil {
			changes = map[string]any{}
		}
		err := s.reloadSettings(ctx, in.Body.Revision, changes)
		if errors.Is(err, config.ErrChanged) {
			return nil, huma.Error409Conflict("Settings changed; reload before saving.")
		}
		if err != nil {
			return nil, invalid("Invalid settings; check the values and reload the configuration.")
		}
		out, err := s.settingsView(ctx)
		return response(out, err)
	})
	if s.client == nil {
		huma.Get(s.contract, Prefix+"/settings/credentials", func(ctx context.Context, _ *struct{}) (*output[[]library.SyncCredential], error) {
			out, err := s.library.SyncCredentials(ctx)
			return response(out, err)
		})
		huma.Post(s.contract, Prefix+"/settings/credentials", func(ctx context.Context, in *struct {
			Body struct {
				Note string `json:"note,omitempty" maxLength:"200"`
			}
		}) (*output[library.NewSyncCredential], error) {
			out, err := s.library.AuthorizeSyncWithNote(ctx, in.Body.Note)
			return response(out, err)
		})
		huma.Put(s.contract, Prefix+"/settings/credentials/{id}", func(ctx context.Context, in *struct {
			ID   string `path:"id"`
			Body struct {
				Note string `json:"note" maxLength:"200"`
			}
		}) (*output[struct{}], error) {
			found, err := s.library.SetSyncCredentialNote(ctx, in.ID, in.Body.Note)
			if err != nil {
				return response(struct{}{}, err)
			}
			if !found {
				return nil, huma.Error404NotFound("No such credential")
			}
			return response(struct{}{}, nil)
		})

		huma.Delete(s.contract, Prefix+"/settings/credentials/{id}", func(ctx context.Context, in *struct {
			ID string `path:"id"`
		}) (*output[struct{}], error) {
			_, err := s.library.RevokeSync(ctx, in.ID)
			return response(struct{}{}, err)
		})
	}
	return nil
}
