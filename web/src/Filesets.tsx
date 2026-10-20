import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button, Checkbox, Group, Stack, TextInput } from "@mantine/core";
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
        onSubmit={(e) => {
          e.preventDefault();
          write(value);
        }}
      >
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
            options={["live_photo", "video_subtitles", "document_pages"].map(
              (kind) => ({
                value: kind,
                label: t(`fileset_${kind}` as "fileset_live_photo"),
              }),
            )}
          />
          <Select
            label={t("filesetPrimary")}
            value={value.primary}
            onValueChange={(primary) => setValue({ ...value, primary })}
            options={[
              { value: zeroID, label: t("filesetAutomatic") },
              ...(data.data?.files || [])
                .filter((f) => subtree.has(f.folder_id))
                .map((f) => ({ value: f.id, label: f.path || f.name })),
            ]}
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
          {data.isPending && <p role="status">{t("loading")}</p>}
          {data.error && <p role="alert">{errorMessage(data.error, t)}</p>}
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
                  {names.get(presentID(m.file_id) ? m.file_id : m.folder_id) ||
                    t("notFound")}
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
                    onChange={(e) => update(i, { language: e.target.value })}
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
          <WriteFeedback state={writes} />
          <Group justify="space-between">
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
