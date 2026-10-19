import { Button, NativeSelect, TextInput } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { get, type ArchiveFile } from "./api";
import { columnName, type SheetData, type Selection } from "./sheetModel";
import type { SheetHandle } from "./UniverSheet";
const UniverSheet = lazy(() => import("./UniverSheet"));
export { columnName } from "./sheetModel";
import { useI18n } from "./i18n";
import { TextPreview } from "./TextPreview";

export function selectedText(
  cells: { text: string }[][],
  selection: Selection,
) {
  const r1 = Math.min(selection.start.row, selection.end.row),
    r2 = Math.max(selection.start.row, selection.end.row),
    c1 = Math.min(selection.start.col, selection.end.col),
    c2 = Math.max(selection.start.col, selection.end.col);
  return cells
    .slice(r1, r2 + 1)
    .map((row) =>
      Array.from({ length: c2 - c1 + 1 }, (_, i) => {
        const value = row[c1 + i]?.text || "";
        return /[\t\n\r"]/.test(value)
          ? `"${value.replaceAll('"', '""')}"`
          : value;
      }).join("\t"),
    )
    .join("\n");
}
export default function SheetPreview({
  file,
  detail,
  active,
  paused,
}: {
  file: ArchiveFile;
  detail: boolean;
  active: boolean;
  paused: boolean;
}) {
  const { t } = useI18n();
  const csv = /\.(csv|tsv)$/i.test(file.name);
  const [sheet, setSheet] = useState(0),
    [row, setRow] = useState(0),
    [column, setColumn] = useState(0),
    [encoding, setEncoding] = useState("auto"),
    [source, setSource] = useState(csv && /^#L\d/.test(location.hash));
  const [selection, setSelection] = useState<Selection | null>(null),
    [search, setSearch] = useState(""),
    [copyState, setCopyState] = useState("");
  const grid = useRef<SheetHandle>(null);
  const query = useQuery({
    queryKey: [
      "sheet-preview",
      file.id,
      file.blob_key,
      detail,
      sheet,
      row,
      column,
      encoding,
    ],
    enabled: active && !paused && !source,
    queryFn: ({ signal }) =>
      get<SheetData>(
        `/files/${encodeURIComponent(file.id)}/sheet-preview?detail=${detail}&sheet=${sheet}&row=${row}&column=${column}&encoding=${encoding}`,
        signal,
      ),
  });
  const data = query.data;
  const cells = (data?.cells || []).map((row) => row || []);
  const columns = Math.max(
    0,
    ...cells.map((r) => r.length),
    ...(data?.merges || []).map((m) => m.column - column + m.columns),
  );
  useEffect(() => {
    setSelection(null);
    setCopyState("");
  }, [sheet, row, column, encoding]);
  useEffect(() => {
    const onHash = () => {
      if (csv && /^#L\d/.test(location.hash)) setSource(true);
    };
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, [csv]);
  const encodingControl = csv && detail && (
    <NativeSelect
      aria-label={t("textEncoding")}
      value={encoding}
      data={[
        { value: "auto", label: t("textEncodingAuto") },
        { value: "gb18030", label: "GBK / GB18030" },
      ]}
      onChange={(e) => {
        setEncoding(e.currentTarget.value);
        setRow(0);
        setColumn(0);
      }}
    />
  );
  const sourceToggle = csv && detail && (
    <div className="markdown-modes" role="group" aria-label={t("sheetView")}>
      <Button
        aria-pressed={!source}
        onClick={() => {
          setSource(false);
          if (/^#L\d/.test(location.hash)) {
            const url = new URL(location.href);
            url.hash = "";
            history.replaceState(history.state, "", url);
          }
        }}
      >
        {t("sheetTable")}
      </Button>
      <Button aria-pressed={source} onClick={() => setSource(true)}>
        {t("markdownSource")}
      </Button>
    </div>
  );
  if (source)
    return (
      <div className="sheet-source">
        <div className="text-toolbar">{sourceToggle}</div>
        <TextPreview file={file} detail active={active} paused={paused} />
      </div>
    );
  if (!data?.supported)
    return (
      <div className="media-placeholder">
        <span aria-hidden="true">▦</span>
        {detail && (
          <>
            <p role="status">
              {query.isError
                ? t("sheetFailed")
                : !data
                  ? t("loading")
                  : data.reason === "limit"
                    ? t("sheetLimit")
                    : data.reason === "encoding"
                      ? t("textEncodingUnknown")
                      : t("sheetFailed")}
            </p>
            {sourceToggle}
            {encodingControl}
            <Button onClick={() => void query.refetch()}>{t("retry")}</Button>
          </>
        )}
      </div>
    );
  const matches = search
    ? cells.flatMap((r, ri) =>
        r.flatMap((c, ci) =>
          c.text.toLocaleLowerCase().includes(search.toLocaleLowerCase())
            ? [{ row: ri, col: ci }]
            : [],
        ),
      )
    : [];
  const find = () => {
    if (!matches.length) return;
    const current = selection?.end;
    const index = matches.findIndex(
      (p) => p.row === current?.row && p.col === current.col,
    );
    const next = matches[(index + 1) % matches.length];
    setSelection({ start: next, end: next });
    grid.current?.select(next.row, next.col);
  };
  const copySelection = async () => {
    if (!selection) return;
    try {
      await navigator.clipboard.writeText(selectedText(cells, selection));
      setCopyState(t("sheetCopied"));
    } catch {
      setCopyState(t("sheetCopyFailed"));
    }
  };
  const widths = Array.from({ length: columns }, (_, col) =>
    detail
      ? Math.max(80, Math.min(320, (data.widths?.[col] || 16) * 7 + 12))
      : 70,
  );
  const table = (
    <table
      className="sheet-table"
      style={{
        width: widths.reduce((sum, width) => sum + width, detail ? 48 : 20),
      }}
    >
      <colgroup>
        <col style={{ width: detail ? 48 : 20 }} />
        {Array.from({ length: columns }, (_, col) => (
          <col
            key={col}
            style={{
              width: detail
                ? Math.max(
                    80,
                    Math.min(320, (data.widths?.[col] || 16) * 7 + 12),
                  )
                : 70,
            }}
          />
        ))}
      </colgroup>
      <thead>
        <tr>
          <th></th>
          {Array.from({ length: columns }, (_, c) => (
            <th scope="col" key={c}>
              {columnName(column + c)}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {cells.map((values, r) => (
          <tr key={r}>
            <th scope="row">{row + r + 1}</th>
            {Array.from({ length: columns }, (_, c) => {
              const merge = data.merges?.find(
                (m) => m.row === row + r && m.column === column + c,
              );
              const covered = data.merges?.some(
                (m) =>
                  row + r >= m.row &&
                  row + r < m.row + m.rows &&
                  column + c >= m.column &&
                  column + c < m.column + m.columns &&
                  (m.row !== row + r || m.column !== column + c),
              );
              if (covered) return null;
              const value = values[c];
              return (
                <td
                  key={c}
                  data-cell={`${r}:${c}`}
                  rowSpan={merge?.rows}
                  colSpan={merge?.columns}
                  title={
                    value?.missing_result
                      ? t("sheetMissingFormula")
                      : value?.text
                  }
                >
                  {value?.missing_result ? (
                    <span className="sheet-formula">
                      {t("sheetMissingFormula")}
                    </span>
                  ) : (
                    value?.text
                  )}
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  );
  if (!detail)
    return (
      <div
        className="sheet-thumbnail"
        aria-label={t("sheetPreview")}
        aria-hidden={false}
      >
        {table}
      </div>
    );
  return (
    <section className="text-reader sheet-reader" aria-label={t("sheetReader")}>
      <div className="text-toolbar">
        {sourceToggle}
        <NativeSelect
          aria-label={t("sheetWorksheet")}
          value={sheet}
          data={(data.sheets || []).map((name, index) => ({
            value: String(index),
            label: name,
          }))}
          onChange={(e) => {
            setSheet(Number(e.currentTarget.value));
            setRow(0);
            setColumn(0);
          }}
        />
        {encodingControl}
        <TextInput
          aria-label={t("sheetSearch")}
          placeholder={t("sheetSearch")}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") find();
          }}
        />
        <Button disabled={!matches.length} onClick={find}>
          {t("pdfNextMatch")}
        </Button>
        <Button disabled={!selection} onClick={copySelection}>
          {t("sheetCopy")}
        </Button>
      </div>
      <p className="text-notice">
        {csv ? data.encoding : t("sheetCachedValues")}
        {data.legacy ? ` · ${t("sheetLegacy")}` : ""}
        {data.truncated ? ` · ${t("sheetTruncated")}` : ""}
        {search ? ` · ${matches.length} ${t("sheetMatches")}` : ""}
      </p>
      {selection && (
        <output className="sheet-selection" aria-live="polite">
          {columnName(column + selection.start.col)}
          {row + selection.start.row + 1}–
          {columnName(column + selection.end.col)}
          {row + selection.end.row + 1}
          {selection.start.row === selection.end.row &&
          selection.start.col === selection.end.col
            ? ` · ${cells[selection.start.row]?.[selection.start.col]?.text || ""}`
            : ""}
        </output>
      )}
      {copyState && (
        <p className="text-notice" role="status">
          {copyState}
        </p>
      )}
      <Suspense fallback={<p className="text-notice">{t("loading")}</p>}>
        <UniverSheet
          data={data}
          onSelection={setSelection}
          onCopySelection={() => void copySelection()}
          ref={grid}
        />
      </Suspense>
      <div className="sheet-pagination">
        <Button disabled={!row} onClick={() => setRow(Math.max(0, row - 100))}>
          {t("sheetPreviousRows")}
        </Button>
        <span>
          {cells.length ? row + 1 : 0}–{row + cells.length}
        </span>
        <Button disabled={!data.more_rows} onClick={() => setRow(row + 100)}>
          {t("sheetNextRows")}
        </Button>
        <Button
          disabled={!column}
          onClick={() => setColumn(Math.max(0, column - 32))}
        >
          {t("sheetPreviousColumns")}
        </Button>
        <span>
          {columnName(column)}–{columnName(column + Math.max(0, columns - 1))}
        </span>
        <Button
          disabled={!data.more_columns}
          onClick={() => setColumn(column + 32)}
        >
          {t("sheetNextColumns")}
        </Button>
      </div>
    </section>
  );
}
