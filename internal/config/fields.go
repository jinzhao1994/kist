package config

import (
	"fmt"
	"math"
	"strings"

	"github.com/BurntSushi/toml"
)

// Field describes one editable setting. Identity and credentials have dedicated
// management paths and are deliberately absent from this public schema.
type Field struct {
	Key      string   `json:"key"`
	Kind     string   `json:"kind"`
	Restart  bool     `json:"restart"`
	Options  []string `json:"options,omitempty"`
	Value    any      `json:"value"`
	Default  any      `json:"default"`
	Explicit bool     `json:"explicit"`
}

func fieldDefinitions(mode string) []Field {
	fields := []Field{
		{Key: "appearance.primary_color", Kind: "select", Options: []string{"gray", "green", "yellow", "pink", "blue", "orange"}},
		{Key: "server.listen", Kind: "text", Restart: true},
		{Key: "server.allowed_origins", Kind: "list", Restart: true},
		{Key: "server.max_upload_bytes", Kind: "integer", Restart: true},
		{Key: "server.max_upload_files", Kind: "integer", Restart: true},
		{Key: "server.map_style", Kind: "text", Restart: true},
		{Key: "server.map_terrain_tiles", Kind: "text", Restart: true},
		{Key: "server.map_terrain_attribution", Kind: "text", Restart: true},
		{Key: "server.map_resource_origins", Kind: "list", Restart: true},
	}
	if mode == "client" {
		return append(fields, Field{Key: "client.cache_max_age", Kind: "text"}, Field{Key: "client.cache_max_bytes", Kind: "integer"})
	}
	return append(fields, Field{Key: "library.timezone", Kind: "text", Restart: true}, Field{Key: "storage.link_mode", Kind: "select", Restart: true, Options: []string{"", "copy", "reflink", "hardlink", "symlink"}}, Field{Key: "remote", Kind: "remotes", Restart: true})
}

func values(c *Config) (map[string]any, error) {
	raw, err := Encode(c)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	_, err = toml.Decode(string(raw), &v)
	return v, err
}
func lookup(v map[string]any, key string) any {
	parts := strings.Split(key, ".")
	for _, k := range parts[:len(parts)-1] {
		v, _ = v[k].(map[string]any)
	}
	return v[parts[len(parts)-1]]
}
func Fields(raw []byte) ([]Field, error) {
	c, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	effective, err := values(c)
	if err != nil {
		return nil, err
	}
	defaults, err := values(Defaults(c.Mode))
	if err != nil {
		return nil, err
	}
	var explicit map[string]any
	if _, err = toml.Decode(string(raw), &explicit); err != nil {
		return nil, err
	}
	fields := fieldDefinitions(c.Mode)
	for i := range fields {
		f := &fields[i]
		f.Value = lookup(effective, f.Key)
		f.Default = lookup(defaults, f.Key)
		f.Explicit = lookup(explicit, f.Key) != nil
		if f.Kind == "list" || f.Kind == "remotes" {
			if f.Value == nil {
				f.Value = []any{}
			}
			if f.Default == nil {
				f.Default = []any{}
			}
		}
		if f.Value == nil {
			f.Value = ""
		}
		if f.Default == nil {
			f.Default = ""
		}
	}
	return fields, nil
}

// NormalizeChanges accepts only editable keys and their exact JSON types.
func NormalizeChanges(mode string, changes map[string]any) (map[string]any, error) {
	allowed := map[string]Field{}
	for _, f := range fieldDefinitions(mode) {
		allowed[f.Key] = f
	}
	out := map[string]any{}
	for key, value := range changes {
		f, ok := allowed[key]
		if !ok {
			return nil, fmt.Errorf("setting is not editable: %s", key)
		}
		if value == nil {
			out[key] = nil
			continue
		}
		switch f.Kind {
		case "text", "select":
			if _, ok = value.(string); !ok {
				return nil, fmt.Errorf("%s requires text", key)
			}
		case "integer":
			switch v := value.(type) {
			case float64:
				if math.IsNaN(v) || math.IsInf(v, 0) || v < 1 || v > 9007199254740991 || v != math.Trunc(v) {
					return nil, fmt.Errorf("%s requires a positive integer", key)
				}
				value = int64(v)
			case int64:
				if v <= 0 {
					return nil, fmt.Errorf("invalid %s", key)
				}
			default:
				return nil, fmt.Errorf("%s requires an integer", key)
			}
		case "list":
			items, ok := value.([]any)
			if !ok {
				return nil, fmt.Errorf("%s requires a list", key)
			}
			list := []string{}
			for _, v := range items {
				s, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("%s requires text entries", key)
				}
				list = append(list, s)
			}
			value = list
		case "remotes":
			items, ok := value.([]any)
			if !ok {
				return nil, fmt.Errorf("remote requires a list")
			}
			list := []Remote{}
			for _, v := range items {
				m, ok := v.(map[string]any)
				if !ok || len(m) != 2 {
					return nil, fmt.Errorf("repair source requires name and path")
				}
				name, n := m["name"].(string)
				path, p := m["path"].(string)
				if !n || !p {
					return nil, fmt.Errorf("repair source requires name and path")
				}
				list = append(list, Remote{Name: name, Path: path})
			}
			value = list
		}
		out[key] = value
	}
	return out, nil
}
