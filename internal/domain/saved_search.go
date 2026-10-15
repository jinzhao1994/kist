package domain

import (
	"fmt"
	"slices"
)

// SearchFilters is the versioned, portable subset of the UI query contract.
// Pagination, transient input, and presentation preferences are not saved.
type SearchFilters struct {
	Name   string   `json:"name,omitempty"`
	Sort   string   `json:"sort"`
	Asc    bool     `json:"asc"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
	Boxes  []string `json:"box,omitempty"`
	Kinds  []string `json:"kind,omitempty"`
	TagAll []string `json:"tag_all,omitempty"`
	TagAny []string `json:"tag_any,omitempty"`
	TagNot []string `json:"tag_not,omitempty"`
}

// SearchDefinition preserves IDs even when referenced entities disappear.
// References are not foreign keys: deleting a tag must not rewrite a search.
type SearchDefinition struct {
	Version  int           `json:"version"`
	Entity   string        `json:"entity"`
	BoxID    string        `json:"box_id,omitempty"`
	FolderID string        `json:"folder_id,omitempty"`
	Filters  SearchFilters `json:"filters"`
}

type SavedSearch struct {
	ID         SavedSearchID    `json:"id"`
	Name       string           `json:"name"`
	Definition SearchDefinition `json:"definition"`
}

// Normalize returns a detached canonical value suitable for durable payloads.
func (d SearchDefinition) Normalize() (SearchDefinition, error) {
	if d.Version != 1 {
		return d, fmt.Errorf("unsupported search version %d", d.Version)
	}
	if d.Entity != "files" && d.Entity != "boxes" {
		return d, fmt.Errorf("invalid search entity")
	}
	if d.Entity == "boxes" && (d.BoxID != "" || d.FolderID != "" || len(d.Filters.Boxes) > 0 || len(d.Filters.Kinds) > 0) {
		return d, fmt.Errorf("file scope in box search")
	}
	if d.FolderID != "" && d.BoxID == "" {
		return d, fmt.Errorf("folder scope requires a box")
	}
	if d.BoxID != "" && len(d.Filters.Boxes) > 0 {
		return d, fmt.Errorf("ambiguous box scope")
	}
	allowed := []string{"captured", "imported", "name", "updated", "type", "size"}
	if d.Entity == "boxes" {
		allowed = []string{"updated", "activity", "name"}
	}
	if !slices.Contains(allowed, d.Filters.Sort) {
		return d, fmt.Errorf("invalid search sort")
	}
	for _, raw := range []string{d.BoxID, d.FolderID} {
		if raw != "" {
			id, err := ParseID(raw)
			if err != nil || id.IsZero() {
				return d, fmt.Errorf("invalid search scope ID")
			}
		}
	}
	for _, list := range []*[]string{&d.Filters.Boxes, &d.Filters.TagAll, &d.Filters.TagAny, &d.Filters.TagNot} {
		if len(*list) > 1000 {
			return d, fmt.Errorf("too many search references")
		}
		ids := make([]string, 0, len(*list))
		for _, raw := range *list {
			id, err := ParseID(raw)
			if err != nil || id.IsZero() {
				return d, fmt.Errorf("invalid search reference")
			}
			ids = append(ids, id.String())
		}
		slices.Sort(ids)
		*list = slices.Compact(ids)
		if len(*list) == 0 {
			*list = nil
		}
	}
	d.Filters.Kinds = slices.Clone(d.Filters.Kinds)
	for _, kind := range d.Filters.Kinds {
		if !slices.Contains([]string{"image", "video", "audio", "document", "other"}, kind) {
			return d, fmt.Errorf("invalid media kind")
		}
	}
	slices.Sort(d.Filters.Kinds)
	d.Filters.Kinds = slices.Compact(d.Filters.Kinds)
	if len(d.Filters.Kinds) == 0 {
		d.Filters.Kinds = nil
	}
	var dates [2]LocalDate
	for i, raw := range []string{d.Filters.From, d.Filters.To} {
		if raw != "" {
			date, err := ParseLocalDate(raw)
			if err != nil {
				return d, err
			}
			dates[i] = date
		}
	}
	if !dates[0].IsZero() && !dates[1].IsZero() && dates[0].Start() > dates[1].End() {
		return d, fmt.Errorf("invalid search date range")
	}
	if len(d.Filters.Name) > 4096 {
		return d, fmt.Errorf("search text is too long")
	}
	return d, nil
}

func NewSavedSearch(id SavedSearchID, name string, definition SearchDefinition) (SavedSearch, error) {
	name = Normalize(name)
	if id.IsZero() {
		return SavedSearch{}, fmt.Errorf("missing search ID")
	}
	if err := ValidateName(name); err != nil {
		return SavedSearch{}, err
	}
	definition, err := definition.Normalize()
	return SavedSearch{ID: id, Name: name, Definition: definition}, err
}
