package sheetpreview

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strconv"

	"github.com/richardlehane/mscfb"
	xls "github.com/ronohara/go-xls-reader"
	"github.com/xuri/excelize/v2"
)

type legacySheet struct {
	merges   []Merge
	widths   map[int]float64
	formulas map[[2]int]bool
}

// Inspect workbook record bounds and shared-string counts before the legacy
// decoder allocates memory from them. CFB extraction is independently bounded.
func legacyMetadata(ctx context.Context, f *os.File) ([]legacySheet, error) {
	compound, err := mscfb.New(f)
	if err != nil {
		return nil, err
	}
	var data []byte
	for _, entry := range compound.File {
		if entry.Name == "Workbook" || entry.Name == "Book" {
			if entry.Size > MaxBytes {
				return nil, errors.New("workbook too large")
			}
			data, err = io.ReadAll(io.LimitReader(entry, MaxBytes+1))
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if len(data) < 4 {
		return nil, errors.New("missing workbook")
	}
	var sheets []legacySheet
	var order []int
	offsets := map[int]int{}
	sheet := -1
	for offset := 0; offset+4 <= len(data); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		recordOffset := offset
		id := binary.LittleEndian.Uint16(data[offset:])
		size := int(binary.LittleEndian.Uint16(data[offset+2:]))
		offset += 4
		if size > len(data)-offset {
			return nil, io.ErrUnexpectedEOF
		}
		record := data[offset : offset+size]
		offset += size
		if id == 0x0085 && size >= 8 {
			order = append(order, int(binary.LittleEndian.Uint32(record)))
			if len(order) > 256 {
				return nil, errLimit
			}
		}
		if id == 0x002f {
			return nil, errors.New("encrypted workbook")
		}
		if id == 0xfc && size >= 8 && binary.LittleEndian.Uint32(record[4:]) > 200000 {
			return nil, errors.New("shared strings exceed limit")
		}
		if id == 0x0809 && size >= 4 && binary.LittleEndian.Uint16(record[2:]) == 0x0010 {
			sheets = append(sheets, legacySheet{widths: map[int]float64{}, formulas: map[[2]int]bool{}})
			sheet++
			offsets[recordOffset] = sheet
			if len(sheets) > 256 {
				return nil, errors.New("too many sheets")
			}
		}
		if sheet < 0 {
			continue
		}
		meta := &sheets[sheet]
		if id == 0x0006 && size >= 14 {
			meta.formulas[[2]int{int(binary.LittleEndian.Uint16(record)), int(binary.LittleEndian.Uint16(record[2:]))}] = true
		}
		if id == 0x007d && size >= 6 {
			first, last, width := int(binary.LittleEndian.Uint16(record)), int(binary.LittleEndian.Uint16(record[2:])), float64(binary.LittleEndian.Uint16(record[4:]))/256
			if last > 256 || first > last {
				return nil, errors.New("invalid column")
			}
			for col := first; col <= min(last, 255); col++ {
				meta.widths[col] = width
			}
		}
		if id == 0x00e5 && size >= 2 {
			count := int(binary.LittleEndian.Uint16(record))
			if count > (size-2)/8 {
				return nil, errors.New("invalid merges")
			}
			for i := 0; i < count; i++ {
				p := record[2+i*8:]
				r1, r2, c1, c2 := int(binary.LittleEndian.Uint16(p)), int(binary.LittleEndian.Uint16(p[2:])), int(binary.LittleEndian.Uint16(p[4:])), int(binary.LittleEndian.Uint16(p[6:]))
				if r2 >= r1 && c2 >= c1 {
					meta.merges = append(meta.merges, Merge{r1, c1, r2 - r1 + 1, c2 - c1 + 1})
				}
			}
		}
	}
	ordered := make([]legacySheet, len(order))
	for i, offset := range order {
		if index, ok := offsets[offset]; ok {
			ordered[i] = sheets[index]
		}
	}
	return ordered, nil
}
func readXLS(ctx context.Context, f *os.File, o Options, p *SheetPreview) error {
	meta, err := legacyMetadata(ctx, f)
	if err != nil {
		return err
	}
	_, _ = f.Seek(0, io.SeekStart)
	book, err := xls.OpenReader(f)
	if err != nil {
		return err
	}
	if book == nil || book.NumSheets() == 0 {
		return errors.New("missing sheets")
	}
	if book.NumSheets() > 256 {
		p.Reason = "limit"
		return nil
	}
	for i := 0; i < book.NumSheets(); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.Sheets = append(p.Sheets, book.GetSheet(i).Name)
	}
	if o.Sheet >= book.NumSheets() {
		p.Reason = "range"
		return nil
	}
	sheet := book.GetSheet(o.Sheet)
	nr, nc := limits(o)
	cols := sheet.MaxCol() + 1
	p.MoreRows = int(sheet.MaxRow)+1 > o.Row+nr
	p.MoreColumns = cols > o.Column+nc
	p.Legacy = true
	formatter := excelize.NewFile()
	defer formatter.Close()
	styles := map[uint16]int{}
	for row := o.Row; row < min(int(sheet.MaxRow)+1, o.Row+nr); row++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		cells := []Cell{}
		for col := o.Column; col < min(cols, o.Column+nc); col++ {
			value := book.TypedCell(o.Sheet, row, col)
			text := ""
			switch value.Kind {
			case "text":
				text = value.Text
			case "bool":
				if value.Bool {
					text = "TRUE"
				} else {
					text = "FALSE"
				}
			case "error":
				text = xls.ErrorText(value.Error)
			case "number":
				text = strconv.FormatFloat(value.Number, 'f', -1, 64)
				if book.IsDate(value.FormatNo) && book.DateMode() == 1 {
					value.Number += 1462
				}
				style, ok := styles[value.FormatNo]
				if !ok {
					format := book.FormatText(value.FormatNo)
					spec := &excelize.Style{NumFmt: int(value.FormatNo)}
					if format != "" {
						spec = &excelize.Style{CustomNumFmt: &format}
					}
					style, _ = formatter.NewStyle(spec)
					styles[value.FormatNo] = style
				}
				_ = formatter.SetCellValue("Sheet1", "A1", value.Number)
				_ = formatter.SetCellStyle("Sheet1", "A1", "A1", style)
				if formatted, e := formatter.GetCellValue("Sheet1", "A1"); e == nil {
					text = formatted
				}
			}
			c := cell(text, p)
			if o.Sheet < len(meta) && meta[o.Sheet].formulas[[2]int{row, col}] && value.Kind == "blank" {
				c.MissingResult = true
			}
			cells = append(cells, c)
		}
		p.Cells = append(p.Cells, cells)
	}
	if o.Sheet < len(meta) {
		m := meta[o.Sheet]
		for col := o.Column; col < min(cols, o.Column+nc); col++ {
			p.Widths = append(p.Widths, m.widths[col])
		}
		for _, merge := range m.merges {
			if merge.Row >= o.Row && merge.Row < o.Row+nr && merge.Column >= o.Column && merge.Column < o.Column+nc {
				merge.Rows = min(merge.Rows, o.Row+nr-merge.Row)
				merge.Columns = min(merge.Columns, o.Column+nc-merge.Column)
				p.Merges = append(p.Merges, merge)
			}
		}
	}
	return nil
}
