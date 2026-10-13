import { UnstyledButton } from "@mantine/core";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useVirtualizer, useWindowVirtualizer } from "@tanstack/react-virtual";
import type { ArchiveFile } from "./api";
import { Media, hasDerivedMedia } from "./Media";
import {
  DisplayModeControl,
  useDisplayMode,
  type DisplayMode,
} from "./DisplayMode";
import { formatBytes, formatInstant, useI18n } from "./i18n";
export function activeImages(
  files: ArchiveFile[],
  first: number,
  last: number,
) {
  return new Set(
    files
      .slice(first, last)
      .filter(hasDerivedMedia)
      .map((file) => file.id),
  );
}
export function FileGrid({
  files,
  open,
  paused = false,
}: {
  files: ArchiveFile[];
  paused?: boolean;
  open: (file: ArchiveFile) => void;
}) {
  const { t, prefs } = useI18n();
  const [mode, setMode] = useDisplayMode("files");
  const rowSize =
    mode === "list"
      ? 72
      : mode === "large"
        ? 328
        : mode === "small"
          ? 168
          : 248;
  const columnCount = (width: number, value: DisplayMode) =>
    value === "list"
      ? 1
      : Math.max(
          1,
          Math.min(
            value === "large" ? 3 : value === "small" ? 8 : 4,
            Math.floor(
              width / (value === "large" ? 300 : value === "small" ? 130 : 210),
            ),
          ),
        );
  const anchor = useRef<{ index: number; offset: number } | null>(null);
  const scroll = useRef<HTMLDivElement>(null);
  const space = useRef<HTMLDivElement>(null);
  const [mobile, setMobile] = useState(
    () => window.matchMedia("(max-width: 680px)").matches,
  );
  const [scrollMargin, setScrollMargin] = useState(0);
  useEffect(() => {
    const media = window.matchMedia("(max-width: 680px)");
    const update = () => setMobile(media.matches);
    media.addEventListener("change", update);
    update();
    return () => media.removeEventListener("change", update);
  }, []);
  useLayoutEffect(() => {
    const element = space.current;
    if (!element) return;
    const main = element.closest("main");
    const measure = () => {
      const top = element.getBoundingClientRect().top;
      setScrollMargin(
        mobile
          ? top + window.scrollY
          : top -
              (main?.getBoundingClientRect().top ?? 0) +
              (main?.scrollTop ?? 0),
      );
    };
    measure();
    const observer = new ResizeObserver(measure);
    // Ancestors and their siblings include headers, upload panels and the
    // mobile sidebar, whose height changes move the grid's scroll origin.
    for (let node: Element | null = element; node; node = node.parentElement) {
      observer.observe(node);
      for (const sibling of node.parentElement?.children ?? [])
        observer.observe(sibling);
    }
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [mobile]);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const [columns, setColumns] = useState(3);
  useEffect(() => {
    const element = scroll.current;
    if (!element) return;
    const update = () =>
      setColumns(
        columnCount(
          element.getBoundingClientRect().width || element.offsetWidth,
          mode,
        ),
      );
    const observer = new ResizeObserver((entries) =>
      setColumns(columnCount(entries[0].contentRect.width, mode)),
    );
    update();
    observer.observe(element);
    return () => observer.disconnect();
  }, [mode]);
  const options = {
    count: Math.ceil(files.length / columns),
    estimateSize: () => rowSize,
    overscan: 1,
    scrollMargin,
  };
  const elementVirtual = useVirtualizer({
    ...options,
    getScrollElement: () => scroll.current?.closest("main") ?? scroll.current,
    enabled: !mobile,
  });
  const windowVirtual = useWindowVirtualizer({
    ...options,
    enabled: mobile,
  });
  const virtual = mobile ? windowVirtual : elementVirtual;
  useLayoutEffect(() => {
    virtual.measure();
    if (anchor.current) {
      virtual.scrollToOffset(
        scrollMargin +
          Math.floor(anchor.current.index / columns) * rowSize +
          anchor.current.offset,
      );
      anchor.current = null;
    }
  }, [mode, columns, rowSize]);
  const rows = virtual.getVirtualItems();
  const top = virtual.scrollOffset || 0;
  const height = virtual.scrollRect?.height ?? 0;
  const visible = rows.filter(
    (row) => row.end > top && row.start < top + height,
  );
  const first = (visible[0]?.index || 0) * columns;
  const last = ((visible.at(-1)?.index ?? 0) + 1) * columns;
  const active = visible.length
    ? activeImages(files, first, last)
    : new Set<string>();
  function move(index: number, delta: number) {
    const target = Math.max(0, Math.min(files.length - 1, index + delta));
    virtual.scrollToIndex(Math.floor(target / columns), { align: "auto" });
    requestAnimationFrame(() =>
      requestAnimationFrame(() =>
        buttons.current.get(files[target].id)?.focus(),
      ),
    );
  }
  return (
    <>
      <DisplayModeControl
        mode={mode}
        change={(next) => {
          anchor.current = {
            index: first,
            offset: Math.max(0, top - (visible[0]?.start || scrollMargin)),
          };
          setColumns(
            columnCount(
              scroll.current?.getBoundingClientRect().width ||
                scroll.current?.offsetWidth ||
                0,
              next,
            ),
          );
          setMode(next);
        }}
      />
      {mode === "list" && (
        <div className="file-list-heading" aria-hidden="true">
          <span>{t("filename")}</span>
          <span>{t("kind")}</span>
          <span>{t("fileSize")}</span>
          <span>{t("capturedAt")}</span>
        </div>
      )}
      <div
        className={`file-scroll view-${mode}`}
        ref={scroll}
        role="region"
        aria-label={t("files")}
        tabIndex={0}
      >
        <div
          ref={space}
          className="virtual-space"
          style={{ height: virtual.getTotalSize() }}
        >
          {rows.map((row) => (
            <div
              className="file-row"
              key={row.key}
              style={{
                transform: `translateY(${row.start - scrollMargin}px)`,
                gridTemplateColumns: `repeat(${columns},minmax(0,1fr))`,
              }}
            >
              {files
                .slice(row.index * columns, (row.index + 1) * columns)
                .map((file, offset) => {
                  const index = row.index * columns + offset;
                  return (
                    <UnstyledButton
                      className="file-card"
                      key={file.id}
                      data-item-id={file.id}
                      ref={(node) => {
                        if (node) buttons.current.set(file.id, node);
                        else buttons.current.delete(file.id);
                      }}
                      onClick={() => open(file)}
                      onKeyDown={(event) => {
                        const rtl = document.documentElement.dir === "rtl";
                        const delta =
                          event.key === "ArrowDown"
                            ? columns
                            : event.key === "ArrowUp"
                              ? -columns
                              : event.key === "ArrowRight"
                                ? rtl
                                  ? -1
                                  : 1
                                : event.key === "ArrowLeft"
                                  ? rtl
                                    ? 1
                                    : -1
                                  : 0;
                        if (delta) {
                          event.preventDefault();
                          move(index, delta);
                        }
                      }}
                    >
                      <div className="file-preview">
                        <Media
                          key={file.id}
                          file={file}
                          active={active.has(file.id)}
                          paused={paused}
                        />
                      </div>
                      <bdi className="file-name">{file.name}</bdi>
                      {mode === "list" && (
                        <>
                          <span className="file-list-kind">
                            {file.media_type || t("unknown")}
                          </span>
                          <span className="file-list-size">
                            {file.size == null
                              ? "—"
                              : formatBytes(file.size, prefs)}
                          </span>
                          <time className="file-list-date">
                            {file.captured_at
                              ? formatInstant(file.captured_at, prefs)
                              : "—"}
                          </time>
                        </>
                      )}
                      <small>
                        <bdi>{file.path || file.name}</bdi>
                      </small>
                    </UnstyledButton>
                  );
                })}
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
