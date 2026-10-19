import { expect, it } from "vitest";
import { sheetModel, type SheetData } from "./sheetModel";
const base: SheetData = {
  supported: true,
  sheets: [],
  sheet: 0,
  row: 0,
  column: 0,
  cells: [],
  widths: [],
  merges: [],
  more_rows: false,
  more_columns: false,
  truncated: false,
};
it("preserves display values without exposing formulas or active content", () => {
  const model = sheetModel(
    {
      ...base,
      cells: [
        [
          { text: '=HYPERLINK("https://example.org")' },
          { text: "<script>alert(1)</script>" },
          { text: "", missing_result: true },
        ],
      ],
      sheets: ["Data"],
      sheet: 0,
      row: 100,
      column: 32,
      widths: [24],
      merges: [{ row: 100, column: 32, rows: 1, columns: 2 }],
    } as SheetData,
    "No saved result",
  );
  const sheet = model.sheets!.page;
  expect(sheet.cellData![0][0]).toEqual({
    v: '=HYPERLINK("https://example.org")',
    t: 1,
  });
  expect(sheet.cellData![0][1]).toEqual({
    v: "<script>alert(1)</script>",
    t: 1,
  });
  expect(sheet.cellData![0][2]).toEqual({ v: "No saved result", t: 1 });
  expect(sheet.mergeData).toEqual([
    { startRow: 0, endRow: 0, startColumn: 0, endColumn: 1 },
  ]);
  expect(sheet.rowCount).toBe(1);
  expect(sheet.columnCount).toBe(3);
});
it("supports empty sheets and sparse rows", () => {
  const model = sheetModel(
    {
      ...base,
      cells: [null, [{ text: "value" }]],
      sheets: [],
      sheet: 0,
      row: 0,
      column: 0,
      merges: [],
      widths: [],
    } as SheetData,
    "missing",
  );
  expect(model.sheets!.page.cellData![0]).toEqual({});
  expect(model.sheets!.page.cellData![1][0].v).toBe("value");
  const empty = sheetModel(
    {
      ...base,
      cells: [],
      row: 0,
      column: 0,
      sheet: 0,
      supported: true,
      sheets: [],
      widths: [],
      merges: [],
      more_rows: false,
      more_columns: false,
      truncated: false,
    } as SheetData,
    "missing",
  );
  expect(empty.sheets!.page.rowCount).toBe(1);
});
