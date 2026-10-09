import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api, createWrite, type ArchiveFile } from "./api";
import { useI18n } from "./i18n";
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
        <button onClick={() => state.run(files)}>
          {t(state.failed ? "retry" : "extract")}
        </button>
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
