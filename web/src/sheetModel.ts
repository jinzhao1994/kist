import type { IWorkbookData, ICellData } from "@univerjs/presets";
import type { components } from "./api-schema";
export type SheetData = components["schemas"]["SheetPreview"];
export type Selection = {
  start: { row: number; col: number };
  end: { row: number; col: number };
};
export function columnName(index: number) {
  let name = "";
  for (let n = index + 1; n > 0; n = Math.floor((n - 1) / 26))
    name = String.fromCharCode(65 + ((n - 1) % 26)) + name;
  return name;
}
export function sheetModel(
  data: SheetData,
  missing: string,
): Partial<IWorkbookData> {
  const rows = data.cells || [];
  const columns = Math.max(
    1,
    ...rows.map((row) => row?.length || 0),
    ...(data.merges || []).map((m) => m.column - data.column + m.columns),
  );
  const cellData: Record<number, Record<number, ICellData>> = {};
  rows.forEach((row, r) => {
    cellData[r] = {};
    row?.forEach((cell, c) => {
      cellData[r][c] = { v: cell.missing_result ? missing : cell.text, t: 1 };
    });
  });
  return {
    id: "archive-preview",
    name: data.sheets?.[data.sheet] || "Sheet",
    sheetOrder: ["page"],
    sheets: {
      page: {
        id: "page",
        name: data.sheets?.[data.sheet] || "Sheet",
        rowCount: Math.max(1, rows.length),
        columnCount: columns,
        defaultRowHeight: 32,
        defaultStyle: { vt: 2, pd: { l: 8, r: 8, t: 4, b: 4 }, fs: 12 },
        defaultColumnWidth: 124,
        rowHeader: { width: 52 },
        columnHeader: { height: 28 },
        cellData,
        columnData: Object.fromEntries(
          Array.from({ length: columns }, (_, c) => [
            c,
            {
              w: Math.max(80, Math.min(320, (data.widths?.[c] || 16) * 7 + 12)),
            },
          ]),
        ),
        mergeData: (data.merges || []).map((m) => ({
          startRow: m.row - data.row,
          endRow: m.row - data.row + m.rows - 1,
          startColumn: m.column - data.column,
          endColumn: m.column - data.column + m.columns - 1,
        })),
      },
    },
  };
}
