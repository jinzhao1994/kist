package library

import (
	"encoding/json"
	"fmt"
)

// resetSettings derives the resettable keys from public default values. Private
// activation boundaries and credentials are not part of this JSON contract.
func resetSettings[T any](input T, defaults any, keys []string) (T, error) {
	if len(keys) == 0 {
		return input, nil
	}
	raw, err := json.Marshal(defaults)
	if err != nil {
		return input, err
	}
	values := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &values); err != nil {
		return input, err
	}
	raw, err = json.Marshal(input)
	if err != nil {
		return input, err
	}
	fields := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &fields); err != nil {
		return input, err
	}
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			return input, fmt.Errorf("unknown setting to reset: %s", key)
		}
		fields[key] = value
	}
	raw, err = json.Marshal(fields)
	if err != nil {
		return input, err
	}
	err = json.Unmarshal(raw, &input)
	return input, err
}
