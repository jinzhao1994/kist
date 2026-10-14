import { UnstyledButton } from "@mantine/core";
import { useLayoutEffect, useRef } from "react";
import { DisplayModeControl, useDisplayMode } from "./DisplayMode";
import type { BoxFilters } from "./Filters";
import type { Box } from "./api";
import { formatInstant, formatLocalDate, useI18n } from "./i18n";

export function BoxGrid({
  boxes,
  open,
  filters,
  onSort,
}: {
  boxes: Box[];
  open: (box: Box) => void;
  filters?: BoxFilters;
  onSort?: (sort: NonNullable<BoxFilters["sort"]>) => void;
}) {
  const { prefs, t } = useI18n();
  const [mode, setMode] = useDisplayMode("boxes");
  const root = useRef<HTMLDivElement>(null);
  const layout = useRef<HTMLDivElement>(null);
  const measure = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const probe = measure.current;
    if (!probe) return;
    const update = () =>
      layout.current?.style.setProperty(
        "--box-date-width",
        `${Math.ceil(probe.getBoundingClientRect().width)}px`,
      );
    update();
    const observer = new ResizeObserver(update);
    observer.observe(probe);
    return () => observer.disconnect();
  }, [prefs.formatLocale]);
  const columns = [
    ["activity", "activityDate"],
    ["name", "name"],
    ["updated", "updated"],
  ] as const;
  const anchor = useRef<{ id: string; top: number } | null>(null);
  useLayoutEffect(() => {
    if (!anchor.current) return;
    const item = root.current?.querySelector<HTMLElement>(
      `[data-item-id="${CSS.escape(anchor.current.id)}"]`,
    );
    if (item) {
      const delta = item.getBoundingClientRect().top - anchor.current.top;
      if (window.matchMedia("(max-width: 680px)").matches)
        window.scrollBy(0, delta);
      else root.current?.closest("main")?.scrollBy(0, delta);
    }
    anchor.current = null;
  }, [mode]);
  return (
    <div ref={layout} className="box-results">
      <div ref={measure} className="box-date-measure" aria-hidden="true">
        {Array.from({ length: 12 }, (_, month) => (
          <span key={month}>
            {formatLocalDate(
              `8888-${String(month + 1).padStart(2, "0")}-28`,
              prefs,
            )}
          </span>
        ))}
      </div>
      {!onSort && (
        <DisplayModeControl
          mode={mode}
          change={(next) => {
            const top = window.matchMedia("(max-width: 680px)").matches
              ? 0
              : root.current?.closest("main")?.getBoundingClientRect().top || 0;
            const item = Array.from(
              root.current?.querySelectorAll<HTMLElement>("[data-item-id]") ||
                [],
            ).find((item) => item.getBoundingClientRect().bottom > top);
            if (item)
              anchor.current = {
                id: item.dataset.itemId!,
                top: item.getBoundingClientRect().top,
              };
            setMode(next);
          }}
        />
      )}
      {mode === "list" && (
        <div className="box-list-heading">
          {columns.map(([sort, label]) => (
            <div key={sort}>
              {onSort ? (
                <UnstyledButton
                  className="box-sort-heading"
                  onClick={() => onSort(sort)}
                  aria-label={`${t(label)}: ${t(filters?.sort === sort && !filters.asc ? "descending" : "ascending")}`}
                >
                  {t(label)}{" "}
                  <span aria-hidden="true">
                    {filters?.sort === sort ? (filters.asc ? "↑" : "↓") : null}
                  </span>
                </UnstyledButton>
              ) : (
                t(label)
              )}
            </div>
          ))}
        </div>
      )}
      <div ref={root} className={`box-grid view-${mode}`}>
        {boxes.map((box) => (
          <UnstyledButton
            className="box-card"
            data-item-id={box.id}
            key={box.id}
            onClick={() => open(box)}
          >
            <span className="box-icon" aria-hidden="true">
              ▤
            </span>
            <span className="box-summary">
              <h2>
                <bdi>{box.name}</bdi>
              </h2>
              {box.description && (
                <span
                  className="box-summary-description"
                  title={box.description}
                >
                  <bdi>{box.description}</bdi>
                </span>
              )}
            </span>
            <span
              className="box-activity"
              title={
                mode === "list" && box.end_at
                  ? `${box.start_at ? formatLocalDate(box.start_at, prefs) : "—"} — ${formatLocalDate(box.end_at, prefs)}`
                  : undefined
              }
            >
              <time>
                {box.start_at
                  ? formatLocalDate(box.start_at, prefs)
                  : mode === "list"
                    ? "—"
                    : ""}
              </time>
              {mode !== "list" && box.end_at && (
                <>
                  <span aria-hidden="true">—</span>
                  <time>{formatLocalDate(box.end_at, prefs)}</time>
                </>
              )}
            </span>
            {mode === "list" && (
              <time className="box-updated">
                {formatInstant(box.updated_at, prefs)}
              </time>
            )}
          </UnstyledButton>
        ))}
      </div>
    </div>
  );
}
