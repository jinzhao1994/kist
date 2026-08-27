package oplog

import "github.com/jinzhao1994/kist/internal/domain"

// Targets are the entities an op touched. They become rows in op_boxes and
// op_files.
//
// Those tables are indexes over the payload, nothing more (spec 2.2.1), which
// is why every handler must be able to produce them from the payload alone:
// that is the direction invariant 4 requires — payload is self-sufficient, and
// every index table rebuilds from it.
type Targets struct {
	Boxes []domain.BoxID
	Files []domain.FileID
}
