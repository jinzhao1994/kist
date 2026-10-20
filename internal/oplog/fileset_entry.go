package oplog

// FilesetEntryPayload couples reference edits to an existing entry deletion.
// Both sides are explicit so replay never chooses a replacement member.
type FilesetEntryPayload struct {
	Entry   []byte           `json:"entry"`
	Changes []FilesetPayload `json:"changes"`
}
type filesetEntry struct{ restore bool }

func init() {
	Register(ActionDeleteFilesetEntry, filesetEntry{})
	Register(ActionRestoreFilesetEntry, filesetEntry{true})
}
func (h filesetEntry) Apply(tx Tx, raw []byte) error {
	p, err := Payload[FilesetEntryPayload](raw)
	if err != nil {
		return err
	}
	if h.restore {
		if err = (entryContents{}).Apply(tx, p.Entry); err != nil {
			return err
		}
	}
	for _, c := range p.Changes {
		if h.restore {
			c.Before, c.After = c.After, c.Before
		}
		b, e := Marshal(c)
		if e != nil {
			return e
		}
		if e = (filesetOp{}).Apply(tx, b); e != nil {
			return e
		}
	}
	if !h.restore {
		return (entryContents{deleting: true}).Apply(tx, p.Entry)
	}
	return nil
}
func (h filesetEntry) Inverse(raw []byte) (Action, []byte, error) {
	if _, err := Payload[FilesetEntryPayload](raw); err != nil {
		return "", nil, err
	}
	if h.restore {
		return ActionDeleteFilesetEntry, raw, nil
	}
	return ActionRestoreFilesetEntry, raw, nil
}
func (filesetEntry) Targets(raw []byte) (Targets, error) {
	p, err := Payload[FilesetEntryPayload](raw)
	if err != nil {
		return Targets{}, err
	}
	t, err := (entryContents{}).Targets(p.Entry)
	if err != nil {
		return t, err
	}
	for _, c := range p.Changes {
		b, err := Marshal(c)
		if err != nil {
			return t, err
		}
		x, err := (filesetOp{}).Targets(b)
		if err != nil {
			return t, err
		}
		t.Filesets = append(t.Filesets, x.Filesets...)
		t.Files = append(t.Files, x.Files...)
		t.FolderAncestors = append(t.FolderAncestors, x.FolderAncestors...)
	}
	return t, nil
}
