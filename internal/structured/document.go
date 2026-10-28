// Package structured validates and merges provider-independent structured facts.
// It has no storage, clock, network, or model dependencies.
package structured

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	Version  = 1
	MaxBytes = 128 << 10
	MaxNodes = 512
	MaxDepth = 8
)

// Document is an independently editable JSON field tree. Array children have
// stable semantic keys, not positional identities.
type Document struct {
	Version int     `json:"version,omitempty"`
	Fields  []Field `json:"fields"`
}

type Field struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Type      string  `json:"type"`
	Value     string  `json:"value,omitempty"`
	Unit      string  `json:"unit,omitempty"`
	Uncertain bool    `json:"uncertain,omitempty"`
	Children  []Field `json:"children,omitempty"`
	Source    string  `json:"source,omitempty"`
	Deleted   bool    `json:"deleted,omitempty"`
}

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var positionalKey = regexp.MustCompile(`^(item|record|person|row|entry)_?[0-9]+$`)
var integerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var durationPattern = regexp.MustCompile(`^[0-9]{2,5}:[0-5][0-9]:[0-5][0-9]$`)

func invalid(path, reason string) error {
	// Never include extracted values in errors: they can appear in task logs.
	return fmt.Errorf("invalid structured facts at %s: %s", path, reason)
}

// ParseModel accepts only the wire result. Ownership and deletion flags are
// library decisions, and cannot be supplied by a model.
func ParseModel(raw string) (Document, error) { return parse(raw, true) }
func Parse(raw string) (Document, error)      { return parse(raw, false) }

func parse(raw string, model bool) (Document, error) {
	var d Document
	if len(raw) > MaxBytes || !utf8.ValidString(raw) {
		return d, invalid("/", "size or encoding limit")
	}
	if err := uniqueJSON([]byte(raw)); err != nil {
		return d, invalid("/", "invalid or duplicate JSON properties")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Document{}, invalid("/", "unexpected JSON structure")
	}
	if d.Fields == nil {
		return Document{}, invalid("/", "fields must be an array")
	}
	if model {
		if d.Version != 0 {
			return Document{}, invalid("/", "model must not supply storage version")
		}
		d.Version = Version
	}
	count := 0
	if d.Version != Version {
		return Document{}, invalid("/", "unsupported format version")
	}
	if err := validate(d.Fields, "/", 1, &count, model, false); err != nil {
		return Document{}, err
	}
	return d, nil
}

// uniqueJSON also rejects trailing JSON, duplicate control keys and excessive
// depth before decoding the typed tree.
func uniqueJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > MaxDepth*3+4 {
			return errors.New("depth")
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return errors.New("duplicate key")
				}
				keys[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid delimiter")
		}
		_, err = dec.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func validate(fields []Field, parent string, depth int, count *int, model, array bool) error {
	if depth > MaxDepth {
		return invalid(parent, "nesting limit")
	}
	seen := map[string]bool{}
	for i := range fields {
		f := &fields[i]
		path := strings.TrimSuffix(parent, "/") + "/" + f.Key
		*count++
		if *count > MaxNodes {
			return invalid(parent, "field count limit")
		}
		if !keyPattern.MatchString(f.Key) || seen[f.Key] {
			return invalid(parent, "invalid or repeated field key")
		}
		seen[f.Key] = true
		if len(f.Label) == 0 || len(f.Label) > 256 || strings.TrimSpace(f.Label) != f.Label || strings.ContainsAny(f.Label, "\x00\n\r") {
			return invalid(path, "invalid label")
		}
		if len(f.Value) > 4096 || len(f.Unit) > 64 || strings.ContainsRune(f.Value, '\x00') || strings.ContainsAny(f.Unit, "\x00\n\r") {
			return invalid(path, "scalar size or encoding limit")
		}
		if array && (f.Type != "object" || positionalKey.MatchString(f.Key)) {
			return invalid(path, "array items require objects with semantic identities")
		}
		if model {
			if f.Source != "" || f.Deleted {
				return invalid(path, "model supplied ownership controls")
			}
			f.Source = "ai"
		} else if f.Source != "ai" && f.Source != "user" {
			return invalid(path, "invalid field source")
		}
		if f.Deleted && f.Source != "user" {
			return invalid(path, "only users may suppress fields")
		}
		if f.Type == "object" || f.Type == "array" {
			if f.Value != "" || f.Unit != "" || f.Uncertain {
				return invalid(path, "container has a scalar value")
			}
			if err := validate(f.Children, path, depth+1, count, model, f.Type == "array"); err != nil {
				return err
			}
		} else {
			if len(f.Children) > 0 {
				return invalid(path, "scalar has children")
			}
			if err := validateScalar(*f); err != nil {
				return invalid(path, err.Error())
			}
		}
	}
	return nil
}

func validateScalar(f Field) error {
	if f.Uncertain && f.Value == "" {
		switch f.Type {
		case "text", "integer", "number", "date", "duration", "boolean":
			return nil
		}
	}
	switch f.Type {
	case "text":
		if strings.TrimSpace(f.Value) == "" {
			return errors.New("empty text must be marked uncertain")
		}
	case "integer":
		if !integerPattern.MatchString(f.Value) {
			return errors.New("invalid integer")
		}
		if _, err := strconv.ParseInt(f.Value, 10, 64); err != nil {
			return errors.New("integer out of range")
		}
	case "number":
		n, err := strconv.ParseFloat(f.Value, 64)
		if !numberPattern.MatchString(f.Value) || err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return errors.New("invalid finite number")
		}
	case "date":
		layout := ""
		switch len(f.Value) {
		case 4:
			layout = "2006"
		case 7:
			layout = "2006-01"
		case 10:
			layout = "2006-01-02"
		}
		d, err := time.Parse(layout, f.Value)
		if layout == "" || err != nil || d.Year() < 1 {
			return errors.New("invalid date or precision")
		}
	case "duration":
		if !durationPattern.MatchString(f.Value) {
			return errors.New("duration must be HH:MM:SS")
		}
	case "boolean":
		if f.Value != "true" && f.Value != "false" {
			return errors.New("invalid boolean")
		}
	default:
		return errors.New("unknown scalar type")
	}
	if f.Unit != "" && f.Type != "integer" && f.Type != "number" {
		return errors.New("unit requires a numeric field")
	}
	return nil
}

// Encode canonicalizes field order so model ordering alone does not churn ops.
func Encode(d Document) (string, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	copy, err := Parse(string(raw))
	if err != nil {
		return "", err
	}
	sortFields(copy.Fields)
	raw, err = json.Marshal(copy)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
func sortFields(fields []Field) {
	sort.Slice(fields, func(i, j int) bool { return fields[i].Key < fields[j].Key })
	for i := range fields {
		sortFields(fields[i].Children)
	}
}

// Merge retains omitted facts and every human field/tombstone. The result owns
// its memory and cannot mutate a prepared snapshot used by concurrency checks.
func Merge(before, next Document) (Document, error) {
	if err := aiFieldsOnly(next.Fields); err != nil {
		return Document{}, err
	}
	raw, err := Encode(before)
	if err != nil {
		return Document{}, err
	}
	out, _ := Parse(raw)
	if _, err := Encode(next); err != nil {
		return Document{}, err
	}
	if err := mergeFields(&out.Fields, next.Fields, ""); err != nil {
		return Document{}, err
	}
	if _, err := Encode(out); err != nil {
		return Document{}, err
	}
	return out, nil
}
func aiFieldsOnly(fields []Field) error {
	for _, f := range fields {
		if f.Source != "ai" || f.Deleted {
			return invalid("", "merge input must contain AI fields only")
		}
		if err := aiFieldsOnly(f.Children); err != nil {
			return err
		}
	}
	return nil
}
func mergeFields(old *[]Field, next []Field, parent string) error {
	for _, n := range next {
		path := parent + "/" + n.Key
		if n.Source != "ai" || n.Deleted {
			return invalid(path, "merge input must contain AI fields only")
		}
		index := -1
		for i, o := range *old {
			if o.Key == n.Key {
				index = i
				break
			}
		}
		if index < 0 {
			for _, o := range *old {
				if strings.EqualFold(o.Label, n.Label) {
					return invalid(path, "ambiguous renamed field; reuse its existing key")
				}
			}
			raw, _ := json.Marshal(n)
			var copy Field
			_ = json.Unmarshal(raw, &copy)
			*old = append(*old, copy)
			continue
		}
		o := &(*old)[index]
		if o.Source == "user" || o.Deleted {
			continue
		}
		if o.Type != n.Type {
			return invalid(path, "field type changed; existing value was preserved")
		}
		if o.Type == "object" || o.Type == "array" {
			if err := mergeFields(&o.Children, n.Children, path); err != nil {
				return err
			}
			o.Label = n.Label
		} else {
			*o = n
		}
	}
	return nil
}

// Edit modifies one scalar or suppresses an entire subtree by stable key path.
func Edit(d Document, path, value string, remove bool) (Document, error) {
	raw, err := Encode(d)
	if err != nil {
		return Document{}, err
	}
	out, _ := Parse(raw)
	if !strings.HasPrefix(path, "/") || path == "/" {
		return Document{}, invalid("/", "invalid edit path")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	fields := out.Fields
	for i, key := range parts {
		index := -1
		for j := range fields {
			if fields[j].Key == key {
				index = j
				break
			}
		}
		if index < 0 || fields[index].Deleted {
			return Document{}, invalid(path, "field is unavailable")
		}
		f := &fields[index]
		if i == len(parts)-1 {
			if remove {
				f.Deleted = true
			} else {
				if f.Type == "object" || f.Type == "array" {
					return Document{}, invalid(path, "edit a scalar field")
				}
				if f.Value == value && !f.Uncertain {
					return out, nil
				}
				f.Value = value
				f.Uncertain = false
			}
			f.Source = "user"
			if _, err := Encode(out); err != nil {
				return Document{}, err
			}
			return out, nil
		}
		fields = f.Children
	}
	return Document{}, invalid(path, "field is unavailable")
}

// Outline exposes only stable paths and labels, never previous personal values.
func Outline(d Document) string {
	type node struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Type     string `json:"type"`
		Children []node `json:"children,omitempty"`
	}
	var convert func([]Field) []node
	convert = func(fields []Field) []node {
		out := make([]node, 0, len(fields))
		for _, f := range fields {
			out = append(out, node{f.Key, f.Label, f.Type, convert(f.Children)})
		}
		return out
	}
	raw, _ := json.Marshal(convert(d.Fields))
	return string(raw)
}
