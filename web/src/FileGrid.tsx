import { useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { ArchiveFile } from "./api";
import { Media, hasDerivedMedia } from "./Media";
import { useI18n } from "./i18n";
export const MAX_ACTIVE_IMAGES = 8;
export function activeImages(
  files: ArchiveFile[],
  first: number,
  last: number,
) {
  return new Set(
    files
      .slice(first, last)
      .filter(hasDerivedMedia)
      .slice(0, MAX_ACTIVE_IMAGES)
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
  const { t } = useI18n();
  const scroll = useRef<HTMLDivElement>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const [columns, setColumns] = useState(3);
  useEffect(() => {
    const element = scroll.current;
    if (!element) return;
    const observer = new ResizeObserver((entries) =>
      setColumns(
        Math.max(
          1,
          Math.min(4, Math.floor(entries[0].contentRect.width / 210)),
        ),
      ),
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const virtual = useVirtualizer({
    count: Math.ceil(files.length / columns),
    getScrollElement: () => scroll.current,
    estimateSize: () => 248,
    overscan: 1,
  });
  const rows = virtual.getVirtualItems();
  const top = virtual.scrollOffset || 0;
  const height = scroll.current?.clientHeight || 600;
  const visible = rows.filter(
    (row) => row.end > top && row.start < top + height,
  );
  const first = (visible[0]?.index || 0) * columns;
  const last = ((visible.at(-1)?.index ?? 0) + 1) * columns;
  const active = activeImages(files, first, last);
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
    <div
      className="file-scroll"
      ref={scroll}
      role="region"
      aria-label={t("files")}
      tabIndex={0}
    >
      <div className="virtual-space" style={{ height: virtual.getTotalSize() }}>
        {rows.map((row) => (
          <div
            className="file-row"
            key={row.key}
            style={{
              transform: `translateY(${row.start}px)`,
              gridTemplateColumns: `repeat(${columns},minmax(0,1fr))`,
            }}
          >
            {files
              .slice(row.index * columns, (row.index + 1) * columns)
              .map((file, offset) => {
                const index = row.index * columns + offset;
                return (
                  <button
                    className="file-card"
                    key={file.id}
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
                        active={!paused && active.has(file.id)}
                      />
                    </div>
                    <bdi className="file-name">{file.name}</bdi>
                    <small>
                      <bdi>{file.path || file.name}</bdi>
                    </small>
                  </button>
                );
              })}
          </div>
        ))}
      </div>
    </div>
  );
}
