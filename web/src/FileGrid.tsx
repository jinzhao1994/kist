import { iconGridColumns, iconGridStyle } from "./icon-grid";
import { TagLabels } from "./TagLabels";
import { DialogPresence } from "./Dialog";
import { SidebarMenuButton } from "./SidebarMenuButton";
import {
  EntryActions,
  EntryRename,
  type EntryActionTarget,
} from "./EntryActions";
import { createPortal } from "react-dom";
import { Menu, UnstyledButton } from "@mantine/core";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { useVirtualizer, useWindowVirtualizer } from "@tanstack/react-virtual";
import { ContextMenu, useContextMenu } from "./ContextMenu";
import { FileIcon, fileType } from "./FileIcon";
import type { Filters } from "./Filters";
import { contentURL, type ArchiveFile, type Folder } from "./api";
import { Media, hasPreview } from "./Media";
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
      .filter(hasPreview)
      .map((file) => file.id),
  );
}
export function FileGrid({
  files,
  folders = [],
  chooseFolder,
  createFolder,
  boxID,
  filters,
  onSort,
  open,
  paused = false,
  toolbarHost,
}: {
  files: ArchiveFile[];
  folders?: Folder[];
  chooseFolder?: (id: string) => void;
  createFolder?: (id: string) => void;
  boxID?: string;
  filters?: Filters;
  onSort?: (sort: NonNullable<Filters["sort"]>) => void;
  paused?: boolean;
  toolbarHost?: HTMLElement | null;
  open: (file: ArchiveFile) => void;
}) {
  const { t, prefs } = useI18n();
  const [mode, setMode] = useDisplayMode("files");
  const sortedFolders = [...folders].sort((a, b) => {
    const byName =
      a.name.localeCompare(b.name, prefs.formatLocale, {
        sensitivity: "base",
        numeric: true,
      }) || a.id.localeCompare(b.id);
    if (filters?.sort === "updated")
      return (
        ((a.updated_at || "").localeCompare(b.updated_at || "") || byName) *
        (filters.asc ? 1 : -1)
      );
    return byName * (filters?.sort === "name" && !filters.asc ? -1 : 1);
  });
  const folderItems = sortedFolders.map((folder) => ({
    id: folder.id,
    name: folder.name,
    folder,
  }));
  const fileItems = files.map((file) => ({
    id: file.id,
    name: file.name,
    file,
  }));
  const items: {
    id: string;
    name: string;
    file?: ArchiveFile;
    folder?: Folder;
  }[] =
    filters?.sort === "type" && !filters.asc
      ? [...fileItems, ...folderItems]
      : [...folderItems, ...fileItems];
  const context = useContextMenu<(typeof items)[number]>();
  const selected = context.menu?.item;
  const [entryAction, setEntryAction] = useState<EntryActionTarget>();
  const selectedBox = selected?.file?.box_id || boxID;
  const dateSort =
    filters?.sort === "captured" || filters?.sort === "imported"
      ? filters.sort
      : "updated";
  const dateLabel =
    dateSort === "captured"
      ? t("captured")
      : dateSort === "imported"
        ? t("created")
        : t("updated");
  function heading(label: string, sort: NonNullable<Filters["sort"]>) {
    return (
      <UnstyledButton
        onClick={() => onSort?.(sort)}
        disabled={!onSort}
        aria-label={`${label}${filters?.sort === sort ? `: ${t(filters.asc ? "ascending" : "descending")}` : ""}`}
      >
        {label}
        {filters?.sort === sort && (
          <span aria-hidden="true"> {filters.asc ? "↑" : "↓"}</span>
        )}
      </UnstyledButton>
    );
  }
  const rowSize =
    mode === "list"
      ? 50
      : mode === "large"
        ? 328
        : mode === "small"
          ? 168
          : 248;
  const columnCount = (width: number, value: DisplayMode) =>
    value !== "small" && window.matchMedia("(max-width: 680px)").matches
      ? 1
      : iconGridColumns(
          width,
          value,
          parseFloat(getComputedStyle(document.documentElement).fontSize) || 16,
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
  const buttons = useRef(new Map<string, HTMLElement>());
  const [columns, setColumns] = useState(3);
  useEffect(() => {
    const element = scroll.current;
    if (!element) return;
    const update = () =>
      setColumns(
        columnCount(
          (element.getBoundingClientRect().width || element.offsetWidth) -
            (parseFloat(getComputedStyle(element).paddingLeft) || 0) -
            (parseFloat(getComputedStyle(element).paddingRight) || 0),
          mode,
        ),
      );
    const observer = new ResizeObserver((entries) =>
      setColumns(columnCount(entries[0].contentRect.width, mode)),
    );
    update();
    observer.observe(element);
    return () => observer.disconnect();
  }, [mode, mobile]);
  const options = {
    count: Math.ceil(items.length / columns),
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
    ? activeImages(
        files,
        Math.max(0, first - folders.length),
        Math.max(0, last - folders.length),
      )
    : new Set<string>();
  function move(index: number, delta: number) {
    const target = Math.max(0, Math.min(items.length - 1, index + delta));
    virtual.scrollToIndex(Math.floor(target / columns), { align: "auto" });
    requestAnimationFrame(() =>
      requestAnimationFrame(() =>
        buttons.current.get(items[target].id)?.focus(),
      ),
    );
  }
  const displayControl = (
    <DisplayModeControl
      dropdown={!!toolbarHost}
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
  );
  return (
    <>
      {toolbarHost ? createPortal(displayControl, toolbarHost) : displayControl}
      {mode === "list" && (
        <div className="file-list-heading">
          <span>{heading(t("name"), "name")}</span>
          <span>{heading(t("type"), "type")}</span>
          <span>{heading(t("fileSize"), "size")}</span>
          <span>{heading(dateLabel, dateSort)}</span>
        </div>
      )}
      <div
        className={`file-scroll view-${mode}`}
        style={iconGridStyle(mode)}
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
              {items
                .slice(row.index * columns, (row.index + 1) * columns)
                .map((item, offset) => {
                  const { file, folder } = item;
                  const editing =
                    entryAction?.action === "rename" &&
                    entryAction.id === item.id;
                  const timestamp = file
                    ? dateSort === "captured"
                      ? file.captured_at
                      : dateSort === "imported"
                        ? file.created_at
                        : file.updated_at
                    : dateSort === "updated"
                      ? folder?.updated_at
                      : undefined;
                  const index = row.index * columns + offset;
                  return (
                    <div className="file-entry" key={item.id}>
                      <UnstyledButton
                        component={editing ? "div" : "button"}
                        className={`file-card ${folder ? "directory-card" : ""}`}
                        aria-label={folder ? folder.name : undefined}
                        data-item-id={item.id}
                        data-active={
                          selected?.id === item.id || editing || undefined
                        }
                        onContextMenu={
                          editing
                            ? undefined
                            : context.bindings(item).onContextMenu
                        }
                        ref={(node: HTMLElement | null) => {
                          if (node) buttons.current.set(item.id, node);
                          else buttons.current.delete(item.id);
                        }}
                        onClick={() =>
                          !editing &&
                          (file ? open(file) : chooseFolder?.(item.id))
                        }
                        onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
                          if (editing) return;
                          context.bindings(item).onKeyDown(event);
                          if (event.defaultPrevented) return;
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
                          {mode !== "list" && file && hasPreview(file) ? (
                            <Media
                              key={file.id}
                              file={file}
                              active={active.has(file.id)}
                              paused={paused}
                            />
                          ) : (
                            <FileIcon folder={!!folder} />
                          )}
                        </div>
                        <div className="file-name-cell">
                          {editing ? (
                            <div className="file-name file-name-edit">
                              <EntryRename
                                entry={entryAction}
                                close={() => setEntryAction(undefined)}
                              />
                            </div>
                          ) : (
                            <bdi className="file-name">{item.name}</bdi>
                          )}
                          {mode === "list" && (
                            <TagLabels tags={file?.display_tags} />
                          )}
                        </div>
                        {mode === "list" && (
                          <>
                            <span className="file-list-kind">
                              {folder
                                ? t("folderType")
                                : fileType(file!) || t("unknown")}
                            </span>
                            <span className="file-list-size">
                              {file?.size == null
                                ? "—"
                                : formatBytes(file.size, prefs)}
                            </span>
                            <time className="file-list-date">
                              {timestamp
                                ? formatInstant(timestamp, prefs)
                                : "—"}
                            </time>
                          </>
                        )}
                        <small>
                          <bdi>
                            {file ? file.path || file.name : t("folderType")}
                          </bdi>
                        </small>
                      </UnstyledButton>
                      {mode === "list" && !editing && (
                        <SidebarMenuButton
                          label={t("moreActions")}
                          expanded={selected?.id === item.id}
                          onClick={(event) => {
                            event.stopPropagation();
                            if (selected?.id === item.id) context.close();
                            else context.atElement(item, event.currentTarget);
                          }}
                        />
                      )}
                    </div>
                  );
                })}
            </div>
          ))}
        </div>
      </div>
      <ContextMenu anchor={context.menu} close={context.close}>
        <Menu.Item
          onClick={() => {
            if (selected?.file) open(selected.file);
            else if (selected?.folder) chooseFolder?.(selected.id);
          }}
        >
          {t("openItem")}
        </Menu.Item>
        {selected?.file && (
          <Menu.Item component="a" href={contentURL(selected.file, true)}>
            {t("download")}
          </Menu.Item>
        )}
        {selected?.folder && createFolder && (
          <Menu.Item onClick={() => createFolder(selected.id)}>
            {t("createFolder")}
          </Menu.Item>
        )}
        {selected && selectedBox && (
          <>
            <Menu.Divider />
            {(["rename", "trash"] as const).map((action) => (
              <Menu.Item
                key={action}
                color={action === "trash" ? "red" : undefined}
                onClick={() =>
                  setEntryAction({
                    id: selected.id,
                    name: selected.name,
                    box: selectedBox,
                    folder: !!selected.folder,
                    action,
                  })
                }
              >
                {t(action === "rename" ? "rename" : "trashEntry")}
              </Menu.Item>
            ))}
          </>
        )}
      </ContextMenu>
      <DialogPresence>
        {entryAction?.action === "trash" && (
          <EntryActions
            entry={entryAction}
            close={() => setEntryAction(undefined)}
          />
        )}
      </DialogPresence>
    </>
  );
}
