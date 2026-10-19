import {
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from "react";
import { useComputedColorScheme } from "@mantine/core";
import { createUniver, LocaleType } from "@univerjs/presets";
import {
  UniverSheetsCorePreset,
  CalculationMode,
  type FWorksheet,
} from "@univerjs/preset-sheets-core";
import enUS from "@univerjs/preset-sheets-core/locales/en-US";
import zhCN from "@univerjs/preset-sheets-core/locales/zh-CN";
import "@univerjs/preset-sheets-core/lib/index.css";
import { useI18n } from "./i18n";
import {
  columnName,
  sheetModel,
  type SheetData,
  type Selection,
} from "./sheetModel";
export type SheetHandle = { select: (row: number, col: number) => void };
export default function UniverSheet({
  data,
  onSelection,
  onCopySelection,
  ref,
}: {
  data: SheetData;
  onSelection: (s: Selection | null) => void;
  onCopySelection: () => void;
  ref: Ref<SheetHandle>;
}) {
  const container = useRef<HTMLDivElement>(null);
  const worksheet = useRef<FWorksheet | null>(null);
  const { prefs, t } = useI18n();
  const scheme = useComputedColorScheme("light");
  const [state, setState] = useState("loading");
  const missing = t("sheetMissingFormula");
  useImperativeHandle(
    ref,
    () => ({
      select(row, col) {
        const sheet = worksheet.current;
        if (sheet) {
          sheet.setActiveRange(sheet.getRange(row, col, 1, 1));
          sheet.scrollToCell(row, col);
        }
      },
    }),
    [],
  );
  useEffect(() => {
    const copy = (event: KeyboardEvent) => {
      if (
        (event.metaKey || event.ctrlKey) &&
        event.key.toLowerCase() === "c" &&
        event.target instanceof Element &&
        container.current?.parentElement?.contains(event.target) &&
        !event.target.closest("input, textarea")
      ) {
        event.preventDefault();
        event.stopImmediatePropagation();
        onCopySelection();
      }
    };
    // Univer handles keyboard input before React's delegated event handlers.
    window.addEventListener("keydown", copy, true);
    return () => window.removeEventListener("keydown", copy, true);
  }, [onCopySelection]);
  useEffect(() => {
    if (!container.current) return;
    setState("loading");
    let disposed = false;
    const { univer, univerAPI } = createUniver({
      locale: prefs.language === "zh" ? LocaleType.ZH_CN : LocaleType.EN_US,
      locales: { [LocaleType.ZH_CN]: zhCN, [LocaleType.EN_US]: enUS },
      darkMode: scheme === "dark",
      presets: [
        UniverSheetsCorePreset({
          container: container.current,
          header: false,
          toolbar: false,
          formulaBar: false,
          contextMenu: false,
          footer: {
            sheetBar: false,
            statisticBar: false,
            menus: false,
            zoomSlider: true,
          },
          formula: { initialFormulaComputing: CalculationMode.NO_CALCULATION },
          sheets: {
            disableForceStringAlert: true,
            disableForceStringMark: true,
          },
        }),
      ],
    });
    const book = univerAPI.createWorkbook(sheetModel(data, missing));
    const sheet = book.getActiveSheet();
    worksheet.current = sheet;
    const subscription = book.onSelectionChange((selections) => {
      const range = selections[selections.length - 1];
      onSelection(
        range
          ? {
              start: { row: range.startRow, col: range.startColumn },
              end: { row: range.endRow, col: range.endColumn },
            }
          : null,
      );
    });
    let permissionsReady = false;
    const ready = () => {
      if (
        !disposed &&
        permissionsReady &&
        univerAPI.getCurrentLifecycleStage() >=
          univerAPI.Enum.LifecycleStages.Rendered
      )
        setState("ready");
    };
    const lifecycle = univerAPI.addEvent(
      univerAPI.Event.LifeCycleChanged,
      ready,
    );
    void book
      .getWorkbookPermission()
      .setMode("viewer")
      .then(async () => {
        if (disposed) return;
        await book
          .getWorkbookPermission()
          .setPoint(univerAPI.Enum.WorkbookPermissionPoint.CopyContent, true);
        if (disposed) return;
        sheet.customizeRowHeader({
          rowsCfg: Object.fromEntries(
            (data.cells || []).map((_, r) => [r, String(data.row + r + 1)]),
          ),
        });
        const columns = sheet.getMaxColumns();
        sheet.customizeColumnHeader({
          columnsCfg: Object.fromEntries(
            Array.from({ length: columns }, (_, c) => [
              c,
              columnName(data.column + c),
            ]),
          ),
        });
        permissionsReady = true;
        ready();
      })
      .catch(() => {
        if (!disposed) setState("failed");
      });
    return () => {
      disposed = true;
      worksheet.current = null;
      subscription.dispose();
      lifecycle.dispose();
      univer.dispose();
    };
  }, [data, prefs.language, scheme, missing, onSelection]);
  return (
    <div
      className="univer-sheet"
      role="region"
      tabIndex={0}
      aria-label={t("sheetCells")}
      data-state={state}
    >
      <div
        ref={container}
        className="univer-container"
        style={{ visibility: state === "ready" ? "visible" : "hidden" }}
      />
      {state !== "ready" && (
        <p className="univer-status" role="status">
          {t(state === "failed" ? "sheetFailed" : "loading")}
        </p>
      )}
    </div>
  );
}
