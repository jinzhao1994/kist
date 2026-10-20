import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ActionIcon,
  Button,
  Checkbox,
  Group,
  Stack,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {
  api,
  createWrite,
  get,
  contentURL,
  type ArchiveFile,
  type Folder,
} from "./api";
import type { components } from "./api-schema";
import { Dialog } from "./Dialog";
import { Select } from "./Select";
import { Media } from "./Media";
import { ExtractionStatus, useExtraction } from "./Extraction";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback, errorMessage } from "./mutations";
export type Fileset = components["schemas"]["Fileset"];
type Member = components["schemas"]["FilesetMember"];
export const zeroID = "00000000-0000-0000-0000-000000000000";
export const presentID = (id?: string) => !!id && id !== zeroID;
export function useFilesets() {
  return useQuery({
    queryKey: ["filesets"],
    queryFn: ({ signal }) => get<{ filesets: Fileset[] }>("/filesets", signal),
  });
}
export function primaryFile(
  set: Fileset,
  sets: Fileset[],
  seen = new Set<string>(),
): string | undefined {
  if (seen.has(set.folder_id)) return;
  seen.add(set.folder_id);
  if (presentID(set.primary)) return set.primary;
  const member = set.members?.find(
    (m) =>
      set.kind === "document_pages" ||
      m.role === (set.kind === "live_photo" ? "image" : "video"),
  );
  if (!member) return;
  if (presentID(member.file_id)) return member.file_id;
  const child = sets.find((s) => s.folder_id === member.folder_id);
  return child ? primaryFile(child, sets, seen) : undefined;
}
export function filesetPageIndex(set: Fileset, page = 1) {
  return Number.isSafeInteger(page) &&
    page >= 1 &&
    page <= (set.members?.length || 0)
    ? page - 1
    : 0;
}
export function readingFile(
  set: Fileset,
  sets: Fileset[],
  path: number[],
  seen = new Set<string>(),
): string | undefined {
  if (seen.has(set.folder_id)) return;
  seen.add(set.folder_id);
  if (set.kind !== "document_pages") return primaryFile(set, sets);
  const member = set.members?.[filesetPageIndex(set, path[0])];
  if (presentID(member?.file_id)) return member!.file_id;
  const child = sets.find(
    (candidate) => candidate.folder_id === member?.folder_id,
  );
  return child ? readingFile(child, sets, path.slice(1), seen) : undefined;
}
export async function boxFiles(box: string, signal?: AbortSignal) {
  const out: ArchiveFile[] = [];
  let cursor = "";
  do {
    const page = await api.files({ box: [box], limit: 1000, cursor }, signal);
    out.push(...(page.files || []));
    cursor = page.cursor || "";
  } while (cursor);
  return out;
}
function descendants(folders: Folder[], root: string) {
  const ids = new Set([root]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const f of folders)
      if (f.parent_id && ids.has(f.parent_id) && !ids.has(f.id)) {
        ids.add(f.id);
        changed = true;
      }
  }
  return ids;
}

export function FilesetEditor({
  folder,
  box,
  sets,
  close,
}: {
  folder: Folder;
  box: string;
  sets: Fileset[];
  close: () => void;
}) {
  const { t } = useI18n();
  const [before] = useState(
    () => sets.find((s) => s.folder_id === folder.id) || null,
  );
  const [value, setValue] = useState<Fileset>(
    () =>
      before || {
        folder_id: folder.id,
        kind: "document_pages",
        layout: "directory",
        primary: zeroID,
        members: [],
      },
  );
  const writes = useWrites(close);
  const data = useQuery({
    queryKey: ["fileset-editor", box, folder.id],
    queryFn: async ({ signal }) => ({
      files: await boxFiles(box, signal),
      folders: (await api.folders(box, signal)).folders || [],
    }),
  });
  const direct =
    data.data?.files.filter((f) => f.folder_id === folder.id) || [];
  const nested = (data.data?.folders || []).filter(
    (f) => f.parent_id === folder.id && sets.some((s) => s.folder_id === f.id),
  );
  const subtree = descendants(data.data?.folders || [], folder.id);
  const members = value.members || [];
  const selected = new Set(
    members.map((m) => (presentID(m.file_id) ? m.file_id : m.folder_id)),
  );
  const names = new Map(
    [...(data.data?.files || []), ...(data.data?.folders || [])].map((f) => [
      f.id,
      f.name,
    ]),
  );
  const update = (i: number, patch: Partial<Member>) =>
    setValue({
      ...value,
      members: members.map((m, j) => (i === j ? { ...m, ...patch } : m)),
    });
  const move = (i: number, step: number) => {
    const next = [...members];
    [next[i], next[i + step]] = [next[i + step], next[i]];
    setValue({ ...value, members: next });
  };
  const [drag, setDrag] = useState<{ from: number; over: number } | null>(null);
  const [orderNotice, setOrderNotice] = useState("");
  const reorder = (from: number, to: number) => {
    if (from === to || to < 0 || to >= members.length) return;
    const next = [...members];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    setValue({ ...value, members: next });
    setOrderNotice(t("filesetReordered", { page: to + 1 }));
  };
  const [initialized, setInitialized] = useState(!!before);
  useEffect(() => {
    if (initialized || !data.data) return;
    setInitialized(true);
    setValue((current) => ({
      ...current,
      members: [
        ...data.data.files
          .filter((f) => f.folder_id === folder.id)
          .map((f) => ({
            file_id: f.id,
            folder_id: zeroID,
            role: "page",
            language: "",
            default: false,
          })),
        ...data.data.folders
          .filter(
            (f) =>
              f.parent_id === folder.id &&
              sets.some((s) => s.folder_id === f.id),
          )
          .map((f) => ({
            file_id: zeroID,
            folder_id: f.id,
            role: "page",
            language: "",
            default: false,
          })),
      ],
    }));
  }, [initialized, data.data, folder.id, sets]);
  const write = (after: Fileset | null) =>
    void writes.run([
      createWrite("PUT", `/filesets/${folder.id}`, {
        box,
        ...(before ? { before } : {}),
        ...(after ? { after } : {}),
      }),
    ]);
  const roles =
    value.kind === "live_photo"
      ? ["image", "video"]
      : value.kind === "video_subtitles"
        ? ["video", "subtitle"]
        : ["page"];
  const add = (id: string, isFolder: boolean) =>
    setValue({
      ...value,
      members: [
        ...members,
        {
          file_id: isFolder ? zeroID : id,
          folder_id: isFolder ? id : zeroID,
          role:
            roles.find(
              (role) =>
                role === "page" ||
                role === "subtitle" ||
                !members.some((m) => m.role === role),
            ) || roles[0],
          language: "",
          default: false,
        },
      ],
    });
  return (
    <Dialog
      open
      title={`${t("filesetConfigure")} · ${folder.name}`}
      onOpenChange={(open) => {
        if (!open && !writes.busy) close();
      }}
    >
      <form
        className="fileset-editor"
        onSubmit={(e) => {
          e.preventDefault();
          write(value);
        }}
      >
        <Stack gap="sm">
          <Select
            label={t("filesetPrimary")}
            disabled={writes.busy || writes.locked || !data.data}
            value={value.primary}
            onValueChange={(primary) => setValue({ ...value, primary })}
            options={[
              {
                value: zeroID,
                label: t(
                  value.kind === "document_pages"
                    ? "filesetFirstPage"
                    : "filesetAutomatic",
                ),
              },
              ...(data.data?.files || [])
                .filter((f) => subtree.has(f.folder_id))
                .map((f) => ({ value: f.id, label: f.path || f.name })),
            ]}
          />
          {data.isPending && <p role="status">{t("loading")}</p>}
          {data.error && <p role="alert">{errorMessage(data.error, t)}</p>}
          {value.kind === "document_pages" ? (
            <section className="fileset-pages-editor">
              <div className="fileset-section-heading">
                <h3>{t("filesetPages")}</h3>
                <Tooltip
                  label={`${t("filesetPageOrderHint")} ${t("filesetExcludedHint")}`}
                  multiline
                  w={280}
                  events={{ hover: true, focus: true, touch: true }}
                >
                  <ActionIcon
                    type="button"
                    variant="subtle"
                    color="gray"
                    size="sm"
                    aria-label={t("filesetPagesHelp")}
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 20 20"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.4"
                      aria-hidden="true"
                    >
                      <circle cx="10" cy="10" r="7.5" />
                      <path d="M10 9v5" />
                      <circle
                        cx="10"
                        cy="6"
                        r=".7"
                        fill="currentColor"
                        stroke="none"
                      />
                    </svg>
                  </ActionIcon>
                </Tooltip>
              </div>
              <span className="sr-only" id="fileset-sort-hint">
                {t("filesetPageOrderHint")}
              </span>
              <ol className="fileset-page-list" aria-label={t("filesetPages")}>
                {members.map((member, index) => {
                  const id = presentID(member.file_id)
                    ? member.file_id
                    : member.folder_id;
                  const name = names.get(id) || t("notFound");
                  return (
                    <li
                      key={id}
                      className={`fileset-page-row${drag?.from === index ? " is-dragging" : ""}${drag?.over === index && drag.from !== index ? (drag.from < index ? " is-drop-after" : " is-drop-before") : ""}`}
                    >
                      <ActionIcon
                        type="button"
                        variant="subtle"
                        color="gray"
                        className="fileset-drag-handle"
                        aria-label={t("filesetDragPage", { name })}
                        aria-describedby="fileset-sort-hint"
                        disabled={writes.busy || writes.locked}
                        onPointerDown={(event) => {
                          if (event.button !== 0) return;
                          event.preventDefault();
                          event.currentTarget.focus();
                          event.currentTarget.setPointerCapture(
                            event.pointerId,
                          );
                          setDrag({ from: index, over: index });
                        }}
                        onPointerMove={(event) => {
                          if (!drag) return;
                          const rows = event.currentTarget
                            .closest("ol")!
                            .querySelectorAll(".fileset-page-row");
                          let over = members.length - 1;
                          for (let i = 0; i < rows.length; i++) {
                            const rect = rows[i].getBoundingClientRect();
                            if (event.clientY < rect.bottom) {
                              over = i;
                              break;
                            }
                          }
                          setDrag({ ...drag, over });
                        }}
                        onPointerUp={() => {
                          if (drag) reorder(drag.from, drag.over);
                          setDrag(null);
                        }}
                        onPointerCancel={() => setDrag(null)}
                        onLostPointerCapture={() => setDrag(null)}
                        onKeyDown={(event) => {
                          if (event.key === "Escape" && drag) {
                            event.stopPropagation();
                            setDrag(null);
                          }
                          if (
                            event.key === "ArrowUp" ||
                            event.key === "ArrowDown"
                          ) {
                            event.preventDefault();
                            event.stopPropagation();
                            reorder(
                              index,
                              index + (event.key === "ArrowUp" ? -1 : 1),
                            );
                          }
                        }}
                      >
                        <svg
                          width="16"
                          height="20"
                          viewBox="0 0 16 20"
                          fill="currentColor"
                          aria-hidden="true"
                        >
                          {[5, 10, 15].map((y) => (
                            <g key={y}>
                              <circle cx="5" cy={y} r="1.2" />
                              <circle cx="11" cy={y} r="1.2" />
                            </g>
                          ))}
                        </svg>
                      </ActionIcon>
                      <span className="fileset-page-position">{index + 1}</span>
                      <bdi className="fileset-page-name" title={name}>
                        {name}
                      </bdi>
                      <ActionIcon
                        type="button"
                        variant="subtle"
                        color="gray"
                        aria-label={t("filesetRemovePage", { name })}
                        className="fileset-remove-page"
                        title={t("filesetRemovePage", { name })}
                        disabled={writes.busy || writes.locked}
                        onClick={() =>
                          setValue({
                            ...value,
                            members: members.filter((_, i) => i !== index),
                          })
                        }
                      >
                        <span aria-hidden="true">×</span>
                      </ActionIcon>
                    </li>
                  );
                })}
              </ol>
              {!members.length && (
                <p className="supporting-text">{t("filesetNoPages")}</p>
              )}
              <span className="sr-only" role="status">
                {orderNotice}
              </span>
              <Select
                label={t("filesetAddPage")}
                value=""
                searchable
                disabled={writes.busy || writes.locked || !data.data}
                onValueChange={(id) => {
                  if (id)
                    add(
                      id,
                      nested.some((f) => f.id === id),
                    );
                }}
                options={[
                  { value: "", label: t("filesetChoosePage") },
                  ...[...direct, ...nested]
                    .filter((f) => !selected.has(f.id))
                    .map((f) => ({ value: f.id, label: f.name })),
                ]}
              />
            </section>
          ) : (
            <>
              <p className="supporting-text">{t("filesetOrderHint")}</p>
              {members.map((m, i) => (
                <Stack
                  gap="xs"
                  key={presentID(m.file_id) ? m.file_id : m.folder_id}
                  className="fileset-member-editor"
                >
                  <Group justify="space-between">
                    <span>
                      {i + 1}.{" "}
                      {names.get(
                        presentID(m.file_id) ? m.file_id : m.folder_id,
                      ) || t("notFound")}
                    </span>
                    <Group gap="xs">
                      <Button
                        size="compact-xs"
                        aria-label={t("filesetMoveUp")}
                        disabled={i === 0}
                        onClick={() => move(i, -1)}
                      >
                        ↑
                      </Button>
                      <Button
                        size="compact-xs"
                        aria-label={t("filesetMoveDown")}
                        disabled={i === members.length - 1}
                        onClick={() => move(i, 1)}
                      >
                        ↓
                      </Button>
                      <Button
                        size="compact-xs"
                        onClick={() =>
                          setValue({
                            ...value,
                            members: members.filter((_, j) => i !== j),
                          })
                        }
                      >
                        {t("filesetRemoveMember")}
                      </Button>
                    </Group>
                  </Group>
                  <Select
                    label={t("filesetRole")}
                    value={m.role}
                    onValueChange={(role) =>
                      update(i, { role, language: "", default: false })
                    }
                    options={roles.map((role) => ({
                      value: role,
                      label: t(`fileset_role_${role}` as "fileset_role_page"),
                    }))}
                  />
                  {m.role === "subtitle" && (
                    <>
                      <TextInput
                        label={t("filesetLanguage")}
                        value={m.language}
                        onChange={(e) =>
                          update(i, { language: e.target.value })
                        }
                      />
                      <Checkbox
                        label={t("filesetDefaultSubtitle")}
                        checked={m.default}
                        onChange={(e) =>
                          setValue({
                            ...value,
                            members: members.map((entry, j) => ({
                              ...entry,
                              default: j === i && e.target.checked,
                            })),
                          })
                        }
                      />
                    </>
                  )}
                </Stack>
              ))}
              <Group>
                {direct
                  .filter((f) => !selected.has(f.id))
                  .map((f) => (
                    <Button
                      key={f.id}
                      variant="default"
                      size="xs"
                      onClick={() => add(f.id, false)}
                    >
                      ＋ {f.name}
                    </Button>
                  ))}
                {value.kind === "document_pages" &&
                  nested
                    .filter((f) => !selected.has(f.id))
                    .map((f) => (
                      <Button
                        key={f.id}
                        variant="default"
                        size="xs"
                        onClick={() => add(f.id, true)}
                      >
                        ＋ {f.name}
                      </Button>
                    ))}
              </Group>
            </>
          )}
          <details className="fileset-advanced">
            <summary>
              <svg
                className="fileset-advanced-chevron"
                width="14"
                height="14"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                aria-hidden="true"
              >
                <path d="m6 3 5 5-5 5" />
              </svg>
              {t("filesetAdvanced")}
            </summary>
            <Stack gap="sm">
              <Select
                label={t("filesetType")}
                value={value.kind}
                disabled={writes.busy || writes.locked || !initialized}
                onValueChange={(kind) =>
                  setValue({
                    ...value,
                    kind,
                    members: [],
                    primary: zeroID,
                    layout: kind === "document_pages" ? "directory" : "flat",
                  })
                }
                options={[
                  "live_photo",
                  "video_subtitles",
                  "document_pages",
                ].map((kind) => ({
                  value: kind,
                  label: t(`fileset_${kind}` as "fileset_live_photo"),
                }))}
              />
              <Select
                label={t("filesetLayout")}
                value={value.layout}
                onValueChange={(layout) => setValue({ ...value, layout })}
                options={[
                  { value: "flat", label: t("filesetFlat") },
                  { value: "directory", label: t("filesetDirectory") },
                ]}
              />
              {before && (
                <Button
                  color="red"
                  variant="subtle"
                  disabled={writes.busy || writes.locked}
                  onClick={() => write(null)}
                >
                  {t("filesetCancel")}
                </Button>
              )}
            </Stack>
          </details>
          <WriteFeedback state={writes} />
          <Group justify="flex-end">
            <Button
              type="button"
              variant="default"
              disabled={writes.busy}
              onClick={close}
            >
              {t("cancel")}
            </Button>
            <Button
              type="submit"
              disabled={writes.busy || writes.locked || !data.data}
            >
              {t("save")}
            </Button>
          </Group>
        </Stack>
      </form>
    </Dialog>
  );
}

export function captionText(raw: string) {
  if (raw.trimStart().startsWith("WEBVTT")) return raw;
  if (!/\d{2}:\d{2}:\d{2}[,.]\d{3}\s+-->/.test(raw))
    throw new Error("Unsupported subtitle format");
  return (
    "WEBVTT\n\n" +
    raw.replace(/^\uFEFF/, "").replace(/(\d{2}:\d{2}:\d{2}),(\d{3})/g, "$1.$2")
  );
}
function useSubtitle(file: ArchiveFile | undefined) {
  const [state, setState] = useState<{ url?: string; error?: boolean }>({});
  useEffect(() => {
    setState({});
    if (!file) return;
    const control = new AbortController();
    let url: string | undefined;
    void (async () => {
      const response = await fetch(contentURL(file), {
        signal: control.signal,
      });
      if (!response.ok || !response.body)
        throw new Error("Subtitle unavailable");
      const reader = response.body.getReader();
      const chunks: Uint8Array[] = [];
      let size = 0;
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          size += value.length;
          if (size > 4 * 1024 * 1024) throw new Error("Subtitle too large");
          chunks.push(value);
        }
      } finally {
        await reader.cancel();
      }
      const bytes = new Uint8Array(size);
      let at = 0;
      for (const c of chunks) {
        bytes.set(c, at);
        at += c.length;
      }
      const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
      if (control.signal.aborted) return;
      url = URL.createObjectURL(
        new Blob([captionText(text)], { type: "text/vtt" }),
      );
      setState({ url });
    })().catch(() => {
      if (!control.signal.aborted) setState({ error: true });
    });
    return () => {
      control.abort();
      if (url) URL.revokeObjectURL(url);
    };
  }, [file?.id, file?.blob_key]);
  return state;
}

export function FilesetPreview({
  set,
  sets,
  files,
  open,
  pagePath,
  changePage,
}: {
  set: Fileset;
  sets: Fileset[];
  files: ArchiveFile[];
  open: (file: ArchiveFile) => void;
  pagePath?: number[];
  changePage?: (path: number[]) => void;
}) {
  const { t } = useI18n();
  const extraction = useExtraction();
  const [localPath, setLocalPath] = useState<number[]>([]);
  const path = pagePath || localPath;
  const updatePath = changePage || setLocalPath;
  const setPage = (index: number) => updatePath([index + 1]);
  const [motion, setMotion] = useState(false);
  const [subtitle, setSubtitle] = useState(
    () => set.members?.find((m) => m.default)?.file_id || "",
  );
  const captions = useSubtitle(files.find((f) => f.id === subtitle));
  const pages = set.members || [];
  const index = filesetPageIndex(set, path[0]);
  const member = pages[index];
  const child = member && sets.find((s) => s.folder_id === member.folder_id);
  const target =
    set.kind === "document_pages"
      ? presentID(member?.file_id)
        ? member.file_id
        : undefined
      : motion
        ? set.members?.find((m) => m.role === "video")?.file_id
        : primaryFile(set, sets);
  const file = files.find((f) => f.id === target);
  return (
    <section
      className={`fileset-preview${set.kind === "document_pages" ? " fileset-page-reader" : ""}`}
      aria-label={t("filesetPreview")}
      onKeyDown={(event) => {
        // Page controls must not trigger navigation between gallery files.
        if (set.kind === "document_pages" && event.key !== "Escape")
          event.stopPropagation();
      }}
    >
      {set.kind === "document_pages" && (
        <div
          className="pdf-toolbar fileset-page-toolbar"
          role="toolbar"
          aria-label={t("filesetPageTools")}
        >
          <Button
            disabled={index === 0 || !pages.length}
            onClick={() => setPage(index - 1)}
          >
            {t("filesetPrevious")}
          </Button>
          <span role="status" aria-label={t("pdfPage")}>
            {pages.length ? index + 1 : 0} / {pages.length}
          </span>
          <Button
            disabled={index + 1 >= pages.length}
            onClick={() => setPage(index + 1)}
          >
            {t("filesetNext")}
          </Button>
        </div>
      )}
      {file && <ExtractionStatus files={[file]} state={extraction} />}
      {captions.error && <p role="alert">{t("filesetSubtitleError")}</p>}
      {set.kind === "document_pages" && child ? (
        <FilesetPreview
          key={child.folder_id}
          pagePath={path.slice(1)}
          changePage={(childPath) => updatePath([index + 1, ...childPath])}
          set={child}
          sets={sets}
          files={files}
          open={open}
        />
      ) : file ? (
        <div className="fileset-media">
          <Media
            key={file.id}
            file={file}
            detail
            tracks={
              captions.url
                ? [
                    {
                      src: captions.url,
                      label:
                        pages.find((m) => m.file_id === subtitle)?.language ||
                        files.find((f) => f.id === subtitle)?.name ||
                        t("fileset_role_subtitle"),
                      language:
                        pages.find((m) => m.file_id === subtitle)?.language ||
                        "und",
                    },
                  ]
                : undefined
            }
          />
        </div>
      ) : (
        <p className="supporting-text">{t("filesetIncomplete")}</p>
      )}
      {set.kind !== "document_pages" && (
        <Group className="fileset-controls" justify="center" gap="sm">
          {set.kind === "live_photo" && (
            <Button
              variant="subtle"
              size="compact-sm"
              onClick={() => setMotion(!motion)}
            >
              {t(motion ? "filesetStill" : "filesetMotion")}
            </Button>
          )}
          {set.kind === "video_subtitles" && (
            <Select
              aria-label={t("fileset_role_subtitle")}
              value={subtitle}
              onValueChange={setSubtitle}
              options={[
                { value: "", label: t("filesetNoSubtitle") },
                ...pages
                  .filter((m) => m.role === "subtitle")
                  .map((m) => ({
                    value: m.file_id,
                    label:
                      m.language ||
                      files.find((f) => f.id === m.file_id)?.name ||
                      m.file_id,
                  })),
              ]}
            />
          )}
          {file && (
            <Button
              variant="subtle"
              size="compact-sm"
              onClick={() => open(file)}
            >
              {t("filesetOpenMember")}
            </Button>
          )}
        </Group>
      )}
    </section>
  );
}
