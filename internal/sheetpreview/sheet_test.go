package sheetpreview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func preview(t *testing.T, name string, data []byte, options Options) SheetPreview {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := Read(t.Context(), f, name, options)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestDelimitedFiles(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       string
	}{
		{"quoted.csv", "name,note\nAlice,\"line one\nline two\"\n", "line one\nline two"},
		{"semicolon.csv", "name;note\nAlice;hello\n", "hello"},
		{"tab.tsv", "name\tnote\nAlice\t=1+1\n", "=1+1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := preview(t, tc.name, []byte(tc.text), Options{Detail: true})
			if !p.Supported || p.Cells[1][1].Text != tc.want {
				t.Fatalf("%+v", p)
			}
		})
	}
	gb, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("标题,内容\n归档,中文日志\n"))
	p := preview(t, "中文.csv", gb, Options{})
	if !p.Supported || p.Encoding != "GB18030" || p.Cells[1][1].Text != "中文日志" {
		t.Fatal(p)
	}
	var b strings.Builder
	for r := 0; r < 105; r++ {
		for c := 0; c < 35; c++ {
			if c > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d:%d", r, c)
		}
		b.WriteByte('\n')
	}
	p = preview(t, "page.csv", []byte(b.String()), Options{Detail: true})
	if len(p.Cells) != 100 || len(p.Cells[0]) != 32 || !p.MoreRows || !p.MoreColumns {
		t.Fatal(p)
	}
	p = preview(t, "page.csv", []byte(b.String()), Options{Detail: true, Row: 100, Column: 32})
	if len(p.Cells) != 5 || p.Cells[0][0].Text != "100:32" || p.MoreRows || p.MoreColumns {
		t.Fatal(p)
	}
	p = preview(t, "empty.csv", nil, Options{})
	if !p.Supported || len(p.Cells) != 0 {
		t.Fatal(p)
	}
	p = preview(t, "bad.csv", []byte("a,\"unterminated"), Options{})
	if p.Supported {
		t.Fatal(p)
	}
}
func TestWorkbookValuesAndPages(t *testing.T) {
	book := excelize.NewFile()
	defer book.Close()
	_ = book.SetSheetName("Sheet1", "费用")
	_, _ = book.NewSheet("备注")
	_ = book.SetCellValue("费用", "A1", "旅行费用")
	_ = book.MergeCell("费用", "A1", "C1")
	_ = book.SetColWidth("费用", "A", "A", 24)
	_ = book.SetCellValue("费用", "A2", 0.25)
	style, _ := book.NewStyle(&excelize.Style{NumFmt: 10})
	_ = book.SetCellStyle("费用", "A2", "A2", style)
	_ = book.SetCellFormula("费用", "B2", "1+2")
	_ = book.SetCellValue("费用", "AG101", "later")
	_ = book.SetCellValue("备注", "A1", "second worksheet")
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	p := preview(t, "sample.xlsx", buf.Bytes(), Options{Detail: true})
	if !p.Supported || len(p.Sheets) != 2 || p.Cells[1][0].Text != "25.00%" || !p.Cells[1][1].MissingResult || len(p.Merges) != 1 || p.Widths[0] != 24 || !p.MoreRows {
		t.Fatalf("%+v", p)
	}
	p = preview(t, "sample.xlsx", buf.Bytes(), Options{Detail: true, Row: 100, Column: 32})
	if !p.Supported || p.Cells[0][0].Text != "later" {
		t.Fatal(p)
	}
	p = preview(t, "sample.xlsx", buf.Bytes(), Options{Sheet: 1})
	if !p.Supported || p.Cells[0][0].Text != "second worksheet" {
		t.Fatal(p)
	}
	p = preview(t, "sample.xlsx", buf.Bytes(), Options{Sheet: 2})
	if p.Supported || p.Reason != "range" {
		t.Fatal(p)
	}
}
func TestInvalidAndCancelled(t *testing.T) {
	for _, name := range []string{"bad.xls", "bad.xlsx", "bad.csv"} {
		p := preview(t, name, []byte{0, 1, 2, 3}, Options{})
		if p.Supported {
			t.Fatal(p)
		}
	}
	p := preview(t, "data.csv", []byte("a,b"), Options{Row: -1})
	if p.Reason != "range" {
		t.Fatal(p)
	}
	path := filepath.Join(t.TempDir(), "large.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.Truncate(MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	p, err = Read(t.Context(), f, "large.csv", Options{})
	if err != nil || p.Reason != "limit" {
		t.Fatal(p, err)
	}
	_ = f.Truncate(0)
	_, _ = f.WriteAt([]byte("a,b\n"), 0)
	_, _ = f.Seek(0, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = Read(ctx, f, "data.csv", Options{})
	if err != context.Canceled {
		t.Fatal(err)
	}
}

func TestLegacyWorkbook(t *testing.T) {
	data, err := os.ReadFile("testdata/travel.xls")
	if err != nil {
		t.Fatal(err)
	}
	p := preview(t, "travel.xls", data, Options{Detail: true})
	if !p.Supported || !p.Legacy || len(p.Sheets) != 2 || p.Cells[0][0].Text != "山海之旅 · 费用明细" || p.Cells[2][2].Text != "40.00%" || p.Cells[6][1].Text != "3,200.00" || len(p.Merges) != 1 {
		t.Fatalf("%+v", p)
	}
	p = preview(t, "travel.xls", data, Options{Detail: true, Sheet: 1, Row: 100, Column: 32})
	if !p.Supported || len(p.Cells) != 10 || p.Cells[0][0].Text != "最后一列" || p.MoreRows || p.MoreColumns {
		t.Fatalf("%+v", p)
	}
}
