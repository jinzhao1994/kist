package library

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/jinzhao1994/kist/internal/config"
)

// settingsSnapshot reads effective values and configuration presence atomically.
// Revision covers presence as well as values, including equal-to-default writes.
func settingsSnapshot(path, section string, defaults any) (*config.Config, []string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := config.Decode(raw)
	if err != nil {
		return nil, nil, "", err
	}
	var values map[string]any
	meta, err := toml.Decode(string(raw), &values)
	if err != nil {
		return nil, nil, "", err
	}
	data, err := json.Marshal(defaults)
	if err != nil {
		return nil, nil, "", err
	}
	var keys map[string]json.RawMessage
	if err = json.Unmarshal(data, &keys); err != nil {
		return nil, nil, "", err
	}
	explicit := []string{}
	for key := range keys {
		if meta.IsDefined(section, key) {
			explicit = append(explicit, key)
		}
	}
	sort.Strings(explicit)
	return cfg, explicit, config.Revision(raw), nil
}

// prepareSettings retains untouched values and validates public field names.
// A nil changed list preserves the legacy full-form PUT contract; an explicit
// list supports changing presence without materializing unrelated defaults.
func prepareSettings[T any](input, current T, defaults any, changed, reset []string) (T, map[string]bool, error) {
	data, err := json.Marshal(defaults)
	if err != nil {
		return input, nil, err
	}
	var allowed map[string]json.RawMessage
	if err = json.Unmarshal(data, &allowed); err != nil {
		return input, nil, err
	}
	selected := map[string]bool{}
	if changed == nil {
		for key := range allowed {
			selected[key] = true
		}
	}
	for _, keys := range [][]string{changed, reset} {
		for _, key := range keys {
			if _, ok := allowed[key]; !ok {
				return input, nil, fmt.Errorf("unknown setting: %s", key)
			}
			selected[key] = true
		}
	}
	data, err = json.Marshal(current)
	if err != nil {
		return input, nil, err
	}
	var previous map[string]json.RawMessage
	if err = json.Unmarshal(data, &previous); err != nil {
		return input, nil, err
	}
	data, err = json.Marshal(input)
	if err != nil {
		return input, nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(data, &fields); err != nil {
		return input, nil, err
	}
	for key := range allowed {
		if !selected[key] {
			fields[key] = previous[key]
		}
	}
	for _, key := range reset {
		fields[key] = allowed[key]
	}
	data, err = json.Marshal(fields)
	if err != nil {
		return input, nil, err
	}
	if err = json.Unmarshal(data, &input); err != nil {
		return input, nil, err
	}
	return input, selected, nil
}
