// Package sheetpreview reads bounded spreadsheet pages without evaluating formulas.
package sheetpreview

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jinzhao1994/kist/internal/textpreview"
	"github.com/xuri/excelize/v2"
)

const MaxBytes = 32 << 20
const maxExpanded = 128 << 20
const maxRows = 1000000
const maxCols = 16384

var errLimit = errors.New("spreadsheet resource limit")

type Options struct {
	Sheet, Row, Column int
	Detail             bool
	Encoding           string
}
type Cell struct {
	Text          string `json:"text"`
	MissingResult bool   `json:"missing_result,omitempty"`
}
type Merge struct {
	Row     int `json:"row"`
	Column  int `json:"column"`
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
}
type SheetPreview struct {
	Supported   bool      `json:"supported"`
	Reason      string    `json:"reason,omitempty"`
	Sheets      []string  `json:"sheets"`
	Sheet       int       `json:"sheet"`
	Row         int       `json:"row"`
	Column      int       `json:"column"`
	Cells       [][]Cell  `json:"cells"`
	Widths      []float64 `json:"widths"`
	Merges      []Merge   `json:"merges"`
	MoreRows    bool      `json:"more_rows"`
	MoreColumns bool      `json:"more_columns"`
	Truncated   bool      `json:"truncated"`
	Encoding    string    `json:"encoding,omitempty"`
	Legacy      bool      `json:"legacy,omitempty"`
}

func limits(o Options) (int, int) {
	if o.Detail {
		return 100, 32
	}
	return 6, 5
}
func cell(s string, p *SheetPreview) Cell {
	if len(s) > 4096 {
		s = s[:4096]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
		p.Truncated = true
	}
	return Cell{Text: s}
}

// Source retains random access and size information without exposing a disk path.
// Cache readers can keep their eviction lease for the whole preview operation.
type Source interface {
	io.ReadSeeker
	io.ReaderAt
	Stat() (os.FileInfo, error)
}

func Read(ctx context.Context, f Source, name string, o Options) (out SheetPreview, err error) {
	out = SheetPreview{Sheet: o.Sheet, Row: o.Row, Column: o.Column, Cells: [][]Cell{}, Sheets: []string{}, Widths: []float64{}, Merges: []Merge{}}
	defer func() {
		if recover() != nil {
			out.Supported = false
			out.Reason = "invalid"
			out.Cells = [][]Cell{}
		}
	}()
	if o.Row < 0 || o.Row >= maxRows || o.Column < 0 || o.Column >= maxCols || o.Sheet < 0 {
		out.Reason = "range"
		return
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	stat, e := f.Stat()
	if e != nil {
		return out, e
	}
	if stat.Size() > MaxBytes {
		out.Reason = "limit"
		return
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv", ".tsv":
		err = readCSV(ctx, f, name, o, &out)
	case ".xlsx":
		err = readXLSX(ctx, f, stat.Size(), o, &out)
	case ".xls":
		err = readXLS(ctx, f, o, &out)
	default:
		out.Reason = "unsupported"
		return
	}
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out.Reason = "invalid"
		if errors.Is(err, errLimit) {
			out.Reason = "limit"
		}
		err = nil
		return
	}
	out.Supported = out.Reason == ""
	return
}
func readCSV(ctx context.Context, f Source, name string, o Options, p *SheetPreview) error {
	if o.Sheet != 0 {
		p.Reason = "range"
		return nil
	}
	text, err := textpreview.ReadLimit(f, MaxBytes, o.Encoding)
	if err != nil {
		return err
	}
	if !text.Supported {
		p.Reason = text.Reason
		return nil
	}
	p.Encoding = text.Encoding
	p.Sheets = []string{name}
	p.Truncated = text.Truncated
	delimiter := ','
	if strings.EqualFold(filepath.Ext(name), ".tsv") {
		delimiter = '\t'
	} else {
		// Prefer the separator with the most consistently populated sample records.
		best := 0
		for _, candidate := range []rune{',', ';', '\t'} {
			r := csv.NewReader(strings.NewReader(text.Text))
			r.Comma = candidate
			r.FieldsPerRecord = -1
			score, count, width := 0, 0, 0
			for count < 8 {
				row, e := r.Read()
				if e != nil {
					break
				}
				if count == 0 {
					width = len(row)
				}
				if len(row) != width {
					score = 0
					break
				}
				score += len(row) - 1
				count++
			}
			if score > best {
				best = score
				delimiter = candidate
			}
		}
	}
	r := csv.NewReader(strings.NewReader(text.Text))
	r.Comma = delimiter
	r.FieldsPerRecord = -1
	nr, nc := limits(o)
	nc = min(nc, maxCols-o.Column)
	for row := 0; row <= o.Row+nr; row++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		values, e := r.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			return e
		}
		if row < o.Row {
			continue
		}
		if row == o.Row+nr {
			p.MoreRows = true
			break
		}
		if len(values) > o.Column+nc {
			p.MoreColumns = true
		}
		cells := []Cell{}
		for col := o.Column; col < min(len(values), o.Column+nc); col++ {
			cells = append(cells, cell(values[col], p))
		}
		p.Cells = append(p.Cells, cells)
	}
	return nil
}

// Validate declared and actual ZIP expansion before passing data to the parser.
func validateXLSX(ctx context.Context, f Source, size int64) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}
	if len(z.File) > 10000 {
		return errLimit
	}
	var total uint64
	for _, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		total += entry.UncompressedSize64
		if total > maxExpanded {
			return errLimit
		}
		if strings.HasPrefix(entry.Name, "xl/worksheets/") && strings.HasSuffix(entry.Name, ".xml") {
			r, e := entry.Open()
			if e != nil {
				return e
			}
			d := xml.NewDecoder(io.LimitReader(r, maxExpanded+1))
			var previousRow, previousCol, paddedCells int
			for {
				tok, e := d.Token()
				if errors.Is(e, io.EOF) {
					break
				}
				if e != nil {
					r.Close()
					return e
				}
				if err := ctx.Err(); err != nil {
					r.Close()
					return err
				}
				start, ok := tok.(xml.StartElement)
				if !ok {
					continue
				}
				if start.Name.Local == "row" {
					paddedCells += previousCol
					if paddedCells > 1000000 {
						r.Close()
						return errLimit
					}
					previousCol = 0
					for _, a := range start.Attr {
						if a.Name.Local == "r" {
							_, row, e := excelize.CellNameToCoordinates("A" + a.Value)
							if e != nil || row <= previousRow || row > maxRows {
								r.Close()
								return errors.New("invalid row")
							}
							if row > 200000 {
								r.Close()
								return errLimit
							}
							previousRow = row
						}
					}
				}
				if start.Name.Local == "c" {
					for _, a := range start.Attr {
						if a.Name.Local == "r" {
							col, row, e := excelize.CellNameToCoordinates(a.Value)
							if e != nil || col <= previousCol || col > maxCols || row > maxRows {
								r.Close()
								return errors.New("invalid cell")
							}
							previousCol = col
						}
					}
				}
			}
			if paddedCells+previousCol > 1000000 {
				r.Close()
				return errLimit
			}
			r.Close()
		}
	}
	return nil
}
func readXLSX(ctx context.Context, f Source, size int64, o Options, p *SheetPreview) error {
	if err := validateXLSX(ctx, f, size); err != nil {
		return err
	}
	_, _ = f.Seek(0, io.SeekStart)
	book, err := excelize.OpenReader(f, excelize.Options{UnzipSizeLimit: maxExpanded, UnzipXMLSizeLimit: 8 << 20})
	if err != nil {
		return err
	}
	defer book.Close()
	p.Sheets = book.GetSheetList()
	if len(p.Sheets) > 256 {
		p.Reason = "limit"
		return nil
	}
	if o.Sheet >= len(p.Sheets) {
		p.Reason = "range"
		return nil
	}
	sheet := p.Sheets[o.Sheet]
	nr, nc := limits(o)
	nc = min(nc, maxCols-o.Column)
	rows, err := book.Rows(sheet)
	if err != nil {
		return err
	}
	defer rows.Close()
	for row := 0; rows.Next(); row++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if row < o.Row {
			continue
		}
		if row >= o.Row+nr {
			p.MoreRows = true
			break
		}
		values, e := rows.Columns()
		if e != nil {
			return e
		}
		if len(values) > o.Column+nc {
			p.MoreColumns = true
		}
		cells := []Cell{}
		for col := o.Column; col < min(len(values), o.Column+nc); col++ {
			c := cell(values[col], p)
			axis, _ := excelize.CoordinatesToCellName(col+1, row+1)
			formula, e := book.GetCellFormula(sheet, axis)
			if e != nil {
				return e
			}
			if formula != "" && c.Text == "" {
				c.MissingResult = true
			}
			cells = append(cells, c)
		}
		p.Cells = append(p.Cells, cells)
	}
	if err := rows.Error(); err != nil {
		return err
	}
	for col := o.Column; col < o.Column+nc; col++ {
		axis, _ := excelize.ColumnNumberToName(col + 1)
		width, e := book.GetColWidth(sheet, axis)
		if e != nil {
			return e
		}
		p.Widths = append(p.Widths, width)
	}
	merges, err := book.GetMergeCells(sheet)
	if err != nil {
		return err
	}
	for _, m := range merges {
		c1, r1, e := excelize.CellNameToCoordinates(m.GetStartAxis())
		if e != nil {
			continue
		}
		c2, r2, e := excelize.CellNameToCoordinates(m.GetEndAxis())
		if e != nil {
			continue
		}
		if r1 > o.Row && r1 <= o.Row+nr && c1 > o.Column && c1 <= o.Column+nc {
			p.Merges = append(p.Merges, Merge{r1 - 1, c1 - 1, min(r2, o.Row+nr) - r1 + 1, min(c2, o.Column+nc) - c1 + 1})
		}
	}
	return nil
}
