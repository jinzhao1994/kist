import { UnstyledButton } from "@mantine/core";
import { useLayoutEffect, useRef } from "react";
import { DisplayModeControl, useDisplayMode } from "./DisplayMode";
import type { Box } from "./api";
import { formatInstant, formatLocalDate, useI18n } from "./i18n";

export function BoxGrid({
  boxes,
  open,
}: {
  boxes: Box[];
  open: (box: Box) => void;
}) {
  const { prefs, t } = useI18n();
  const [mode, setMode] = useDisplayMode("boxes");
  const root = useRef<HTMLDivElement>(null);
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
    <>
      <DisplayModeControl
        mode={mode}
        change={(next) => {
          const top = window.matchMedia("(max-width: 680px)").matches
            ? 0
            : root.current?.closest("main")?.getBoundingClientRect().top || 0;
          const item = Array.from(
            root.current?.querySelectorAll<HTMLElement>("[data-item-id]") || [],
          ).find((item) => item.getBoundingClientRect().bottom > top);
          if (item)
            anchor.current = {
              id: item.dataset.itemId!,
              top: item.getBoundingClientRect().top,
            };
          setMode(next);
        }}
      />
      {mode === "list" && (
        <div className="box-list-heading" aria-hidden="true">
          <span>{t("name")}</span>
          <span>{t("description")}</span>
          <span>{t("activityDate")}</span>
          <span>{t("updated")}</span>
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
            <h2>
              <bdi>{box.name}</bdi>
            </h2>
            <p>
              <bdi>{box.description}</bdi>
            </p>
            <time>
              {box.start_at ? formatLocalDate(box.start_at, prefs) : ""}
              {box.end_at ? ` — ${formatLocalDate(box.end_at, prefs)}` : ""}
            </time>
            {mode === "list" && (
              <time className="box-updated">
                {formatInstant(box.updated_at, prefs)}
              </time>
            )}
          </UnstyledButton>
        ))}
      </div>
    </>
  );
}
