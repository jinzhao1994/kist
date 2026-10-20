package projection

import (
	"fmt"
	"github.com/jinzhao1994/kist/internal/domain"
	"path"
	"slices"
	"strings"
)

type projectedNode struct {
	id       domain.ID
	suffix   string
	entry    Entry
	children []projectedGroup
}
type projectedGroup struct {
	id     domain.ID
	prefix string
	nodes  []projectedNode
}

// planFilesets allocates entire naming groups in the actual output directory.
// A collision renames every video/subtitle sibling together, never just one file.
func planFilesets(boxDir string, box domain.Box, folders []domain.Folder, files []domain.File, sets []domain.Fileset) ([]Entry, error) {
	byFolder := map[domain.FolderID]domain.Folder{}
	children := map[domain.FolderID][]domain.Folder{}
	inFolder := map[domain.FolderID][]domain.File{}
	bySet := map[domain.FolderID]domain.Fileset{}
	for _, f := range folders {
		byFolder[f.ID] = f
		if f.ParentID != nil {
			children[*f.ParentID] = append(children[*f.ParentID], f)
		}
	}
	for _, f := range files {
		inFolder[f.FolderID] = append(inFolder[f.FolderID], f)
	}
	for _, s := range sets {
		bySet[s.FolderID] = s
	}
	seen := map[domain.FolderID]bool{}
	seenFiles := 0
	var directory func(domain.FolderID) ([]projectedGroup, error)
	var flatten func(domain.FolderID, string) ([]projectedNode, error)
	flatten = func(id domain.FolderID, prefix string) ([]projectedNode, error) {
		if seen[id] {
			return nil, fmt.Errorf("cyclic fileset hierarchy")
		}
		seen[id] = true
		s := bySet[id]
		suffixes := map[domain.ID]string{}
		for i, m := range s.Members {
			target := m.FileID.ID
			if target.IsZero() {
				target = m.FolderID.ID
			}
			switch m.Role {
			case "image", "video":
				suffixes[target] = ""
			case "page":
				suffixes[target] = fmt.Sprintf(".p%03d", i+1)
			case "subtitle":
				label := m.Language
				if label == "" {
					label = fmt.Sprintf("subtitle%d", i+1)
				}
				suffixes[target] = "." + Sanitize(label)
			}
		}
		var out []projectedNode
		for _, f := range inFolder[id] {
			tail, ok := suffixes[f.ID.ID]
			if !ok {
				tail = "." + Sanitize(f.Name)
			} else {
				tail += path.Ext(f.Name)
			}
			out = append(out, projectedNode{id: f.ID.ID, suffix: prefix + tail, entry: Entry{BlobKey: f.BlobKey}})
			seenFiles++
		}
		for _, f := range children[id] {
			tail, ok := suffixes[f.ID.ID]
			if !ok {
				tail = "." + Sanitize(f.Name)
			}
			if sub, ok := bySet[f.ID]; ok && sub.Layout == "flat" {
				nodes, err := flatten(f.ID, prefix+tail)
				if err != nil {
					return nil, err
				}
				out = append(out, nodes...)
			} else {
				kids, err := directory(f.ID)
				if err != nil {
					return nil, err
				}
				out = append(out, projectedNode{id: f.ID.ID, suffix: prefix + tail, entry: Entry{IsDir: true, ModTime: f.UpdatedAt}, children: kids})
			}
		}
		return out, nil
	}
	directory = func(id domain.FolderID) ([]projectedGroup, error) {
		if seen[id] {
			return nil, fmt.Errorf("cyclic folder hierarchy")
		}
		seen[id] = true
		var groups []projectedGroup
		for _, f := range inFolder[id] {
			groups = append(groups, projectedGroup{id: f.ID.ID, prefix: f.Name, nodes: []projectedNode{{id: f.ID.ID, entry: Entry{BlobKey: f.BlobKey}}}})
			seenFiles++
		}
		for _, f := range children[id] {
			if s, ok := bySet[f.ID]; ok && s.Layout == "flat" {
				nodes, err := flatten(f.ID, "")
				if err != nil {
					return nil, err
				}
				// Distinct members can ask for the same suffix (e.g. two English subtitles).
				named := make([]Named, len(nodes))
				for i, n := range nodes {
					named[i] = Named{ID: n.id, Name: "member" + n.suffix}
				}
				resolved := Resolve(named)
				for i := range nodes {
					nodes[i].suffix = strings.TrimPrefix(resolved[nodes[i].id], "member")
				}
				groups = append(groups, projectedGroup{id: f.ID.ID, prefix: f.Name, nodes: nodes})
			} else {
				kids, err := directory(f.ID)
				if err != nil {
					return nil, err
				}
				groups = append(groups, projectedGroup{id: f.ID.ID, prefix: f.Name, nodes: []projectedNode{{id: f.ID.ID, entry: Entry{IsDir: true, ModTime: f.UpdatedAt}, children: kids}}})
			}
		}
		return groups, nil
	}
	root, ok := byFolder[box.RootFolderID]
	if !ok || root.ParentID != nil {
		return nil, fmt.Errorf("missing box root")
	}
	groups, err := directory(root.ID)
	if err != nil {
		return nil, err
	}
	if len(seen) != len(folders) || seenFiles != len(files) {
		return nil, fmt.Errorf("unreachable projection members")
	}
	out := []Entry{{Path: boxDir, IsDir: true, ModTime: box.UpdatedAt}}
	var emit func(string, []projectedGroup) error
	emit = func(dir string, groups []projectedGroup) error {
		slices.SortFunc(groups, func(a, b projectedGroup) int { return strings.Compare(a.id.String(), b.id.String()) })
		used := map[string]bool{}
		for _, g := range groups {
			base := Sanitize(g.prefix)
			candidate := base
			for attempt := 0; ; attempt++ {
				collision := false
				for _, n := range g.nodes {
					if used[domain.Fold(Sanitize(candidate+n.suffix))] {
						collision = true
						break
					}
				}
				if !collision {
					break
				}
				stem, ext := domain.SplitName(base)
				candidate = stem + "~" + strings.ReplaceAll(g.id.String(), "-", "")
				if attempt > 0 {
					candidate += fmt.Sprintf("-%d", attempt+1)
				}
				candidate += ext
			}
			for _, n := range g.nodes {
				name := Sanitize(candidate + n.suffix)
				used[domain.Fold(name)] = true
				e := n.entry
				e.Path = path.Join(dir, name)
				out = append(out, e)
				if e.IsDir {
					if err := emit(e.Path, n.children); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err = emit(boxDir, groups); err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}
