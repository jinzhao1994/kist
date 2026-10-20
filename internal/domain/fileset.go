package domain

import (
	"fmt"
	"slices"
)

// Fileset is a presentation of a folder, not a second ownership hierarchy.
// A zero Primary selects the type's automatic primary. Members are ordered.
type Fileset struct {
	FolderID FolderID        `json:"folder_id"`
	Kind     string          `json:"kind"`
	Primary  FileID          `json:"primary"`
	Layout   string          `json:"layout"`
	Members  []FilesetMember `json:"members"`
}

// FilesetMember describes a direct child's role. Exactly one target is set.
type FilesetMember struct {
	FileID   FileID   `json:"file_id"`
	FolderID FolderID `json:"folder_id"`
	Role     string   `json:"role"`
	Language string   `json:"language"`
	Default  bool     `json:"default"`
}

func (s Fileset) Validate() error {
	if s.FolderID.IsZero() {
		return fmt.Errorf("fileset requires a folder")
	}
	if s.Kind != "live_photo" && s.Kind != "video_subtitles" && s.Kind != "document_pages" {
		return fmt.Errorf("unknown fileset type")
	}
	if s.Layout != "flat" && s.Layout != "directory" {
		return fmt.Errorf("unknown fileset layout")
	}
	if len(s.Members) > 10000 {
		return fmt.Errorf("too many fileset members")
	}
	seen := map[ID]bool{}
	roles := map[string]int{}
	defaults := 0
	for _, m := range s.Members {
		if m.FileID.IsZero() == m.FolderID.IsZero() {
			return fmt.Errorf("member requires exactly one target")
		}
		id := m.FileID.ID
		if id.IsZero() {
			id = m.FolderID.ID
		}
		if seen[id] {
			return fmt.Errorf("duplicate fileset member")
		}
		seen[id] = true
		allowed := s.Kind == "document_pages" && m.Role == "page" || s.Kind == "live_photo" && (m.Role == "image" || m.Role == "video") || s.Kind == "video_subtitles" && (m.Role == "video" || m.Role == "subtitle")
		if !allowed || (!m.FolderID.IsZero() && m.Role != "page") {
			return fmt.Errorf("invalid member role")
		}
		roles[m.Role]++
		if m.Role != "page" && m.Role != "subtitle" && roles[m.Role] > 1 {
			return fmt.Errorf("duplicate media role")
		}
		if len(m.Language) > 64 || (m.Language != "" && m.Role != "subtitle") || (m.Default && m.Role != "subtitle") {
			return fmt.Errorf("invalid subtitle attributes")
		}
		if m.Default {
			defaults++
		}
	}
	if defaults > 1 {
		return fmt.Errorf("multiple default subtitles")
	}
	return nil
}

func EqualFileset(a, b *Fileset) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.FolderID == b.FolderID && a.Kind == b.Kind && a.Primary == b.Primary && a.Layout == b.Layout && slices.Equal(a.Members, b.Members)
}

// FilesetPrimary resolves automatic selection without persisting a second truth.
// Missing or cyclic configurations have no effective primary.
func FilesetPrimary(id FolderID, sets map[FolderID]Fileset) FileID {
	seen := map[FolderID]bool{}
	for !seen[id] {
		seen[id] = true
		s, ok := sets[id]
		if !ok {
			return FileID{}
		}
		if !s.Primary.IsZero() {
			return s.Primary
		}
		var next FolderID
		for _, m := range s.Members {
			if s.Kind == "live_photo" && m.Role != "image" || s.Kind == "video_subtitles" && m.Role != "video" {
				continue
			}
			if !m.FileID.IsZero() {
				return m.FileID
			}
			next = m.FolderID
			break
		}
		if next.IsZero() {
			return FileID{}
		}
		id = next
	}
	return FileID{}
}
