import { Button } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";
import { api, boxFiles, createWrite, type ArchiveFile } from "./api";
import { useI18n } from "./i18n";
import type { Fileset } from "./Filesets";
import { errorMessage } from "./mutations";
// Metadata is separate from upload success. Each file/extractor pair owns an
// immutable request key, retained after failures and retried without upload.
export function useExtraction() {
  const client = useQueryClient();
  const writes = useRef(new Map<string, ReturnType<typeof createWrite>>());
  const mounted = useRef(true);
  const running = useRef(false);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [error, setError] = useState<unknown>();
  const [done, setDone] = useState(0);
  const [total, setTotal] = useState(0);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  async function run(files: ArchiveFile[]) {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    setFailed(false);
    setError(undefined);
    setDone(0);
    const work = files.filter((file) => file.extraction_pending?.length);
    setTotal(
      work.reduce(
        (count, file) => count + (file.extraction_pending?.length || 0),
        0,
      ),
    );
    let next = 0,
      completed = 0,
      hasFailure = false;
    async function worker() {
      while (next < work.length && mounted.current) {
        const file = work[next++];
        // Extractors for one file must observe preceding capture updates.
        for (const extractor of file.extraction_pending || []) {
          if (!mounted.current) break;
          const id = `${file.id}/${extractor}`;
          let write = writes.current.get(id);
          if (!write) {
            write = createWrite(
              "POST",
              `/files/${file.id}/extractions/${extractor}`,
              {},
            );
            writes.current.set(id, write);
          }
          try {
            await write.run();
            writes.current.delete(id);
          } catch (error) {
            hasFailure = true;
            if (mounted.current) setError(error);
          }
          completed++;
          if (mounted.current) setDone(completed);
        }
      }
    }
    await Promise.all([worker(), worker(), worker()]);
    await client.invalidateQueries();
    running.current = false;
    if (mounted.current) {
      setBusy(false);
      setFailed(hasFailure);
    }
  }
  return { run, busy, failed, error, done, total };
}
export function ExtractionStatus({
  files,
  state,
}: {
  files: ArchiveFile[];
  state: ReturnType<typeof useExtraction>;
}) {
  const { t } = useI18n();
  const pending = files.some((file) => file.extraction_pending?.length);
  if (!pending && !state.busy && !state.failed) return null;
  return (
    <div className="extraction-status">
      <p role={state.failed ? "alert" : "status"}>
        {t(
          state.failed
            ? "extractionFailed"
            : state.busy
              ? "extracting"
              : "extractionPending",
        )}
      </p>
      {state.failed && state.error instanceof Error && (
        <p>{errorMessage(state.error, t)}</p>
      )}
      {state.busy ? (
        <progress
          value={state.done}
          max={Math.max(1, state.total)}
          aria-label={t("extracting")}
        />
      ) : (
        <Button onClick={() => state.run(files)}>
          {t(state.failed ? "retry" : "extract")}
        </Button>
      )}
    </div>
  );
}
// Paginate authoritative results after an upload; imported names may have been
// numbered by the server, so the client does not guess new file IDs or paths.
export async function pendingFiles(box: string) {
  const files: ArchiveFile[] = [];
  let cursor = "";
  do {
    const page = await api.files({
      box: [box],
      sort: "imported",
      limit: 1000,
      cursor,
    });
    files.push(
      ...(page.files || []).filter((file) => file.extraction_pending?.length),
    );
    cursor = page.cursor || "";
  } while (cursor);
  return files;
}

// A collapsed entry represents every configured member, not just its primary.
export function combinationFileIDs(files: ArchiveFile[], sets: Fileset[]) {
  const ids = new Set<string>();
  const seen = new Set<string>();
  const byID = new Map(sets.map((set) => [set.folder_id, set]));
  const add = (id?: string) => {
    if (id && id !== "00000000-0000-0000-0000-000000000000") ids.add(id);
  };
  function visit(id: string) {
    if (seen.has(id)) return;
    seen.add(id);
    const set = byID.get(id);
    if (!set) return;
    add(set.primary);
    for (const member of set.members || []) {
      add(member.file_id);
      if (member.folder_id) visit(member.folder_id);
    }
  }
  for (const file of files) {
    add(file.id);
    if (file.fileset_id) visit(file.fileset_id);
  }
  return ids;
}

export function BrowseExtractionStatus({
  files,
  sets,
  state,
}: {
  files: ArchiveFile[];
  sets: Fileset[];
  state: ReturnType<typeof useExtraction>;
}) {
  const { t } = useI18n();
  const boxes = Array.from(
    new Set(
      files
        .filter((file) => file.fileset_id)
        .map((file) => file.box_id)
        .filter((box): box is string => !!box),
    ),
  );
  // Share the complete file catalog already used by the combination reader.
  const queries = useQueries({
    queries: boxes.map((box) => ({
      queryKey: ["fileset-content", box],
      queryFn: ({ signal }: { signal: AbortSignal }) => boxFiles(box, signal),
    })),
  });
  const failed = queries.find((query) => query.isError);
  if (failed)
    return (
      <div className="extraction-status">
        <p role="alert">{errorMessage(failed.error, t)}</p>
        <Button onClick={() => void failed.refetch()}>{t("retry")}</Button>
      </div>
    );
  if (queries.some((query) => query.isPending)) return null;
  const ids = combinationFileIDs(files, sets);
  const all = new Map(files.map((file) => [file.id, file]));
  for (const query of queries)
    for (const file of query.data || [])
      if (ids.has(file.id)) all.set(file.id, file);
  return <ExtractionStatus files={Array.from(all.values())} state={state} />;
}
