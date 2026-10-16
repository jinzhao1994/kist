import { Button, UnstyledButton, TextInput } from "@mantine/core";
import { useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  api,
  APIError,
  createWrite,
  type Box,
  type HistoricalFile,
  type Tag,
} from "./api";
import { Select } from "./Select";
import { Dialog, DialogPresence } from "./Dialog";
import {
  en,
  formatInstant,
  formatLocalDate,
  useI18n,
  type MessageKey,
  type Translate,
} from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";

export function actionLabel(action: string, t: Translate) {
  const key = `action_${action}` as MessageKey;
  return key in en ? t(key) : action;
}
function QueryError({
  error,
  retry,
}: {
  error: unknown;
  retry: () => unknown;
}) {
  const { t } = useI18n();
  return (
    <div role="alert">
      <p>{errorMessage(error, t)}</p>
      <Button onClick={() => retry()}>{t("retry")}</Button>
    </div>
  );
}
export function HistoryPage({
  box,
  file,
  trash,
  boxes,
  scope,
}: {
  box?: string;
  file?: string;
  trash?: boolean;
  boxes: Box[];
  scope: (box?: string, file?: string) => void;
}) {
  const { t, prefs } = useI18n();
  const [selected, setSelected] = useState<number>();
  const [snapshot, setSnapshot] = useState<{ box: string; op: number }>();
  const [versions, setVersions] = useState<{
    file: string;
    from: number;
    to: number;
  }>();
  const query = useInfiniteQuery({
    queryKey: ["history", { box, file, trash }],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      api.history({ box, file, trash, before: pageParam, limit: 50 }, signal),
    getNextPageParam: (page) => page.before || undefined,
  });
  const ops = query.data?.pages.flatMap((page) => page.ops || []) || [];
  return (
    <section className="history-page">
      {trash && <p className="notice">{t("trashHint")}</p>}
      <div className="history-toolbar">
        <Select
          label={t("historyScope")}
          value={box || ""}
          onValueChange={(value) => scope(value || undefined)}
          options={[
            { value: "", label: t("allHistory") },
            ...(box && !boxes.some((item) => item.id === box)
              ? [{ value: box, label: box }]
              : []),
            ...boxes.map((item) => ({ value: item.id, label: item.name })),
          ]}
        />
        {file && (
          <p>
            {t("versionHistory")} · <bdi>{file}</bdi>
            <Button onClick={() => scope()}>{t("allHistory")}</Button>
          </p>
        )}
      </div>
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && (
        <QueryError error={query.error} retry={query.refetch} />
      )}
      {query.isSuccess && !ops.length && (
        <p className="empty-state">{t("historyEmpty")}</p>
      )}
      <ol className="history-list">
        {ops.map((op) => (
          <li key={op.id}>
            <UnstyledButton
              className="history-entry"
              onClick={() => setSelected(op.id)}
            >
              <span className="history-number">#{op.id}</span>
              <span>
                <strong>{actionLabel(op.action, t)}</strong>
                <small>{formatInstant(op.ts, prefs)}</small>
              </span>
              <span className="history-status">
                {op.undone_by
                  ? t("undoneBy", { id: op.undone_by })
                  : op.reverts_id
                    ? t("reverts", { id: op.reverts_id })
                    : ""}
              </span>
            </UnstyledButton>
            {file && (
              <UnstyledButton
                className="text-button"
                onClick={() =>
                  setVersions({ file, from: Math.max(0, op.id - 1), to: op.id })
                }
              >
                {t("compareVersions")}
              </UnstyledButton>
            )}
          </li>
        ))}
      </ol>
      {query.hasNextPage && (
        <Button
          disabled={query.isFetchingNextPage}
          onClick={() => query.fetchNextPage()}
        >
          {t(query.isFetchingNextPage ? "loading" : "loadMore")}
        </Button>
      )}
      <DialogPresence>
        {selected !== undefined && (
          <OperationDetail
            key={selected}
            id={selected}
            close={() => setSelected(undefined)}
            select={setSelected}
            fileHistory={(file) => scope(undefined, file)}
            viewBox={(box, op) => setSnapshot({ box, op })}
            compare={(file, op) =>
              setVersions({ file, from: Math.max(0, op - 1), to: op })
            }
          />
        )}
      </DialogPresence>
      <DialogPresence>
        {snapshot && (
          <HistoricalBoxDialog
            {...snapshot}
            close={() => setSnapshot(undefined)}
            compare={(file) =>
              setVersions({
                file,
                from: Math.max(0, snapshot.op - 1),
                to: snapshot.op,
              })
            }
          />
        )}
      </DialogPresence>
      <DialogPresence>
        {versions && (
          <VersionComparison
            key={`${versions.file}:${versions.from}:${versions.to}`}
            {...versions}
            close={() => setVersions(undefined)}
          />
        )}
      </DialogPresence>
    </section>
  );
}
function OperationDetail({
  id,
  close,
  select,
  viewBox,
  compare,
  fileHistory,
}: {
  id: number;
  close: () => void;
  select: (id: number) => void;
  viewBox: (box: string, op: number) => void;
  compare: (file: string, op: number) => void;
  fileHistory: (file: string) => void;
}) {
  const { t, prefs } = useI18n();
  const query = useQuery({
    queryKey: ["operation", id],
    queryFn: ({ signal }) => api.operation(id, signal),
  });
  const [confirm, setConfirm] = useState(false);
  const writes = useWrites(() => setConfirm(false));
  const op = query.data;
  const deleting = op?.action.startsWith("delete_");
  const blocking =
    writes.error instanceof APIError
      ? writes.error.body.blocking_op || writes.error.body.reverted_by
      : undefined;
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !writes.busy) close();
      }}
      title={t("operation", { id })}
      className="detail-dialog"
    >
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && (
        <QueryError error={query.error} retry={query.refetch} />
      )}
      {op && (
        <>
          <h3>{actionLabel(op.action, t)}</h3>
          <p>
            <time>{formatInstant(op.ts, prefs)}</time>
          </p>
          {op.reverts_id && (
            <Button onClick={() => select(op.reverts_id!)}>
              {t("reverts", { id: op.reverts_id })}
            </Button>
          )}
          {op.undone_by && (
            <Button onClick={() => select(op.undone_by!)}>
              {t("undoneBy", { id: op.undone_by })}
            </Button>
          )}
          <div className="actions">
            <Button
              disabled={
                !op.undoable || !!op.undone_by || writes.busy || writes.locked
              }
              onClick={() => setConfirm(true)}
            >
              {t(deleting ? "restore" : "undo")}
            </Button>
          </div>
          {confirm && (
            <div className="notice">
              <p>{t("undoConfirm")}</p>
              <div className="actions">
                <Button
                  variant="filled"
                  disabled={writes.busy || writes.locked}
                  onClick={() =>
                    writes.run([createWrite("POST", `/ops/${id}/undo`, {})])
                  }
                >
                  {t(writes.busy ? "saving" : deleting ? "restore" : "undo")}
                </Button>
                <Button
                  disabled={writes.busy}
                  onClick={() => setConfirm(false)}
                >
                  {t("cancel")}
                </Button>
              </div>
            </div>
          )}
          <WriteFeedback state={writes} />
          {blocking && (
            <Button onClick={() => select(blocking)}>
              {t("blockedBy", { id: blocking })} · {t("viewOperation")}
            </Button>
          )}
          {!!op.boxes?.length && (
            <>
              <h3>{t("affectedBoxes")}</h3>
              <ul className="history-targets">
                {op.boxes.map((box) => (
                  <li key={box}>
                    <bdi>{box}</bdi>{" "}
                    <Button
                      onClick={() => viewBox(box, deleting ? id - 1 : id)}
                    >
                      {t(deleting ? "viewBeforeDeletion" : "viewHistoricalBox")}
                    </Button>
                  </li>
                ))}
              </ul>
            </>
          )}
          {!!op.files?.length && (
            <>
              <h3>{t("affectedFiles")}</h3>
              <ul className="history-targets">
                {op.files.map((file) => (
                  <li key={file}>
                    <bdi>{fileName(op.payload, file)}</bdi>{" "}
                    <Button onClick={() => fileHistory(file)}>
                      {t("versionHistory")}
                    </Button>
                    <Button onClick={() => compare(file, id)}>
                      {t("compareVersions")}
                    </Button>
                  </li>
                ))}
              </ul>
            </>
          )}
          {op.payload.box &&
          typeof op.payload.box === "object" &&
          "Name" in op.payload.box &&
          typeof op.payload.box.Name === "string" ? (
            <p>
              <bdi>{op.payload.box.Name}</bdi>
            </p>
          ) : null}
          {typeof op.payload.name === "string" && (
            <p>
              <bdi>{op.payload.name}</bdi>
            </p>
          )}
        </>
      )}
    </Dialog>
  );
}
function fileName(payload: Record<string, unknown>, id: string): string {
  if (!Array.isArray(payload.files)) return id;
  const archived = payload.files.find(
    (item: unknown) =>
      typeof item === "object" &&
      item !== null &&
      "file" in item &&
      typeof item.file === "object" &&
      item.file !== null &&
      "ID" in item.file &&
      item.file.ID === id,
  );
  if (archived && typeof archived.file?.Name === "string")
    return archived.file.Name;
  const item = payload.files.find(
    (item: unknown) =>
      typeof item === "object" &&
      item !== null &&
      "file_id" in item &&
      item.file_id === id,
  );
  return item && typeof item.name === "string" ? item.name : id;
}
function HistoricalBoxDialog({
  box,
  op,
  close,
  compare,
}: {
  box: string;
  op: number;
  close: () => void;
  compare: (file: string) => void;
}) {
  const { t, prefs } = useI18n();
  const query = useInfiniteQuery({
    queryKey: ["historical-box", box, op],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) =>
      api.historicalBox(op, box, pageParam, signal),
    getNextPageParam: (page) => page.cursor || undefined,
  });
  const first = query.data?.pages[0];
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
      title={first?.box.name || t("boxes")}
      className="detail-dialog"
    >
      <p className="notice">{t("historicalState", { id: op })}</p>
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && (
        <QueryError error={query.error} retry={query.refetch} />
      )}
      {first && (
        <>
          <p>{first.box.description}</p>
          <dl className="historical-box-facts">
            <dt>{t("startAt")}</dt>
            <dd>
              {first.box.start_at
                ? formatLocalDate(first.box.start_at, prefs)
                : "—"}
            </dd>
            <dt>{t("endAt")}</dt>
            <dd>
              {first.box.end_at
                ? formatLocalDate(first.box.end_at, prefs)
                : "—"}
            </dd>
            <dt>{t("created")}</dt>
            <dd>{formatInstant(first.box.created_at, prefs)}</dd>
            <dt>{t("updated")}</dt>
            <dd>{formatInstant(first.box.updated_at, prefs)}</dd>
          </dl>
          <p>{tagNames(first.tags || [], first.box_tags || []).join(" · ")}</p>
          {!!first.folders?.filter((folder) => folder.parent_id).length && (
            <details>
              <summary>{t("folders")}</summary>
              <ul>
                {first.folders
                  .filter((folder) => folder.parent_id)
                  .map((folder) => (
                    <li key={folder.id}>
                      <bdi>{folder.name}</bdi>
                    </li>
                  ))}
              </ul>
            </details>
          )}
          <ul className="history-targets">
            {query.data?.pages
              .flatMap((page) => page.files || [])
              .map((file) => (
                <li key={file.id}>
                  <Button onClick={() => compare(file.id)}>
                    <bdi>{file.path || file.name}</bdi>
                  </Button>
                  <a
                    className="button-link"
                    href={historicalContent(file.id, op)}
                    download
                  >
                    {t("download")}
                  </a>
                </li>
              ))}
          </ul>
          {!first.files?.length && <p>{t("noFiles")}</p>}
          {query.hasNextPage && (
            <Button
              disabled={query.isFetchingNextPage}
              onClick={() => query.fetchNextPage()}
            >
              {t("loadMore")}
            </Button>
          )}
        </>
      )}
    </Dialog>
  );
}
function historicalContent(file: string, op: number) {
  return `/api/v1/files/${encodeURIComponent(file)}/content?at=${op}&download=true`;
}
function tagNames(tree: Tag[], ids: string[]): string[] {
  const names = new Map<string, string>();
  const walk = (tags: Tag[], prefix = "") => {
    for (const tag of tags) {
      const path = prefix + tag.name;
      names.set(tag.id, path);
      walk(tag.children || [], `${path} / `);
    }
  };
  walk(tree);
  return ids.map((id) => names.get(id) || id).sort();
}
async function historicalFile(op: number, file: string, signal: AbortSignal) {
  if (op === 0) return null;
  try {
    return await api.historicalFile(op, file, signal);
  } catch (error) {
    if (
      error instanceof APIError &&
      error.status === 404 &&
      error.body.code === "file_not_found"
    )
      return null;
    throw error;
  }
}
export function comparisonFields(value: HistoricalFile | null, t: Translate) {
  const fields: Record<string, string> = {};
  if (!value) return fields;
  const detail = value.file,
    file = detail.file;
  fields[t("path")] = file.path || file.name;
  fields[t("blobKey")] = file.blob_key;
  fields[t("capturedAt")] = file.captured_at || "—";
  fields[t("created")] = file.created_at;
  fields[t("updated")] = file.updated_at;
  fields[t("tags")] = tagNames(value.tags || [], detail.tags || []).join(" · ");
  for (const row of detail.metadata || [])
    fields[row.key] = `${row.value ?? ""} (${row.type}; ${row.source})`;
  return fields;
}
function VersionComparison({
  file,
  from,
  to,
  close,
}: {
  file: string;
  from: number;
  to: number;
  close: () => void;
}) {
  const { t } = useI18n();
  const [draftFrom, setDraftFrom] = useState(String(from));
  const [draftTo, setDraftTo] = useState(String(to));
  const [range, setRange] = useState({ from, to });
  const before = useQuery({
    queryKey: ["historical-file", file, range.from],
    queryFn: ({ signal }) => historicalFile(range.from, file, signal),
    retry: false,
  });
  const after = useQuery({
    queryKey: ["historical-file", file, range.to],
    queryFn: ({ signal }) => historicalFile(range.to, file, signal),
    retry: false,
  });
  const a = comparisonFields(before.data || null, t),
    b = comparisonFields(after.data || null, t);
  const keys = [...new Set([...Object.keys(a), ...Object.keys(b)])];
  const pending = before.isPending || after.isPending;
  const valid =
    [draftFrom, draftTo].every(
      (n) => /^\d+$/.test(n) && Number.isSafeInteger(Number(n)),
    ) && Number(draftFrom) < Number(draftTo);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
      title={t("compareVersions")}
      className="detail-dialog"
    >
      <form
        className="history-toolbar"
        onSubmit={(event) => {
          event.preventDefault();
          if (valid) setRange({ from: Number(draftFrom), to: Number(draftTo) });
        }}
      >
        <label>
          {t("beforeVersion")}
          <TextInput
            type="number"
            min="0"
            value={draftFrom}
            onChange={(event) => setDraftFrom(event.target.value)}
          />
        </label>
        <label>
          {t("afterVersion")}
          <TextInput
            type="number"
            min="1"
            value={draftTo}
            onChange={(event) => setDraftTo(event.target.value)}
          />
        </label>
        <Button type="submit" disabled={!valid || pending}>
          {t("compare")}
        </Button>
      </form>
      {pending && <p role="status">{t("loading")}</p>}
      {before.isError && (
        <QueryError error={before.error} retry={before.refetch} />
      )}
      {after.isError && (
        <QueryError error={after.error} retry={after.refetch} />
      )}
      {before.isSuccess && after.isSuccess && (
        <>
          <p className="notice">
            {keys.some((key) => a[key] !== b[key]) ||
            !!before.data !== !!after.data
              ? t("changedFields")
              : t("noDifferences")}
          </p>
          <div className="comparison-scroll">
            <table className="comparison-table">
              <thead>
                <tr>
                  <th>{t("field")}</th>
                  <th>#{range.from}</th>
                  <th>#{range.to}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <th>{t("files")}</th>
                  {[before.data, after.data].map((data, i) => (
                    <td key={i}>
                      {data ? (
                        <>
                          <bdi>{data.file.file.name}</bdi>
                          <br />
                          <a
                            href={historicalContent(
                              file,
                              i ? range.to : range.from,
                            )}
                            download
                          >
                            {t("download")}
                          </a>
                        </>
                      ) : (
                        t("notPresent")
                      )}
                    </td>
                  ))}
                </tr>
                {keys.map((key) => (
                  <tr key={key} className={a[key] !== b[key] ? "changed" : ""}>
                    <th>
                      <bdi>{key}</bdi>
                    </th>
                    <td>
                      <bdi>{a[key] ?? "—"}</bdi>
                    </td>
                    <td>
                      <bdi>{b[key] ?? "—"}</bdi>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </Dialog>
  );
}
