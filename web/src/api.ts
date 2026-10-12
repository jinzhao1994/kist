import type { components, operations } from "./api-schema";
export type Box = components["schemas"]["Box"];
export type ArchiveFile = components["schemas"]["File"];
export type FileDetail = components["schemas"]["FileDetail"];
export type Tag = components["schemas"]["Tag"];
export type Receipt = components["schemas"]["RequestResult"];
export type HistoryOp = components["schemas"]["HistoryOp"];
export type HistoryDetail = components["schemas"]["HistoryDetail"];
export type HistoricalFile = components["schemas"]["HistoricalFile"];
export type HistoryQuery = NonNullable<
  operations["get-api-v1-ops"]["parameters"]["query"]
>;
export type APIErrorBody = components["schemas"]["HTTPError"];
export type FileQuery = NonNullable<
  operations["get-api-v1-files"]["parameters"]["query"]
>;
export class APIError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: APIErrorBody,
  ) {
    super(body.message);
  }
}
async function result<T>(response: Response): Promise<T> {
  const body = await response.json();
  if (!response.ok) {
    // A committed write can report a failed projection or post-commit read.
    // It must never be repeated as a new operation.
    if (body.code === "write_committed" && body.op_id > 0)
      return { op_id: body.op_id } as T;
    throw new APIError(response.status, body);
  }
  return body;
}
export async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  return result<T>(await fetch(`/api/v1${path}`, { signal }));
}
export const api = {
  boxDeletionPreview: (id: string, signal?: AbortSignal) =>
    get<components["schemas"]["BoxDeletionPreview"]>(
      `/boxes/${encodeURIComponent(id)}/deletion-preview`,
      signal,
    ),
  history: (query: HistoryQuery, signal?: AbortSignal) => {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query))
      if (value !== undefined && value !== "") params.set(key, String(value));
    return get<components["schemas"]["HistoryList"]>(`/ops?${params}`, signal);
  },
  operation: (id: number, signal?: AbortSignal) =>
    get<HistoryDetail>(`/ops/${id}`, signal),
  historicalBox: (op: number, box: string, cursor = "", signal?: AbortSignal) =>
    get<components["schemas"]["HistoricalBox"]>(
      `/history/${op}/boxes/${encodeURIComponent(box)}?cursor=${encodeURIComponent(cursor)}`,
      signal,
    ),
  historicalFile: (op: number, file: string, signal?: AbortSignal) =>
    get<HistoricalFile>(
      `/history/${op}/files/${encodeURIComponent(file)}`,
      signal,
    ),
  boxes: (signal?: AbortSignal) =>
    get<components["schemas"]["BoxList"]>("/boxes", signal),
  info: (signal?: AbortSignal) =>
    get<components["schemas"]["LibraryInfo"]>("/info", signal),
  tags: (signal?: AbortSignal) =>
    get<components["schemas"]["TagTree"]>("/tags", signal),
  file: (id: string, signal?: AbortSignal) =>
    get<FileDetail>(`/files/${encodeURIComponent(id)}`, signal),
  files: (query: FileQuery, signal?: AbortSignal) => {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query))
      if (value !== undefined && value !== "")
        params.set(key, Array.isArray(value) ? value.join(",") : String(value));
    return get<components["schemas"]["FileList"]>(`/files?${params}`, signal);
  },
  receipt: (key: string) =>
    get<Receipt>(`/requests/${encodeURIComponent(key)}`),
};
const pendingStorage = "kist.pending-writes.v1";
type PendingWrite = {
  key: string;
  method: string;
  path: string;
  body?: string;
};
function pendingWrites(): PendingWrite[] {
  try {
    const value = JSON.parse(sessionStorage.getItem(pendingStorage) || "[]");
    return Array.isArray(value)
      ? value.filter(
          (item) =>
            typeof item.key === "string" &&
            typeof item.method === "string" &&
            typeof item.path === "string" &&
            (item.body === undefined || typeof item.body === "string"),
        )
      : [];
  } catch {
    return [];
  }
}
function savePending(writes: PendingWrite[]) {
  try {
    sessionStorage.setItem(pendingStorage, JSON.stringify(writes));
  } catch {
    /* The live operation still retains its key when storage is unavailable. */
  }
}
export function requestKey(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
export function createWrite(
  method: "POST" | "PUT" | "DELETE",
  path: string,
  body?: unknown,
) {
  const serialized = body === undefined ? undefined : JSON.stringify(body);
  const pending = pendingWrites();
  const key =
    pending.find(
      (item) =>
        item.method === method &&
        item.path === path &&
        item.body === serialized,
    )?.key || requestKey();
  if (!pending.some((item) => item.key === key))
    savePending([...pending, { key, method, path, body: serialized }]);
  const discard = () =>
    savePending(pendingWrites().filter((item) => item.key !== key));
  return Object.freeze({
    key,
    discard,
    run: async (signal?: AbortSignal): Promise<Receipt> => {
      const receipt = await result<Receipt>(
        await fetch(`/api/v1${path}`, {
          method,
          signal,
          headers: {
            "Content-Type": "application/json",
            "Idempotency-Key": key,
          },
          body: serialized,
        }),
      );
      discard();
      return receipt;
    },
  });
}

export function contentURL(file: ArchiveFile, download = false) {
  return `/api/v1/files/${encodeURIComponent(file.id)}/content${download ? "?download=true" : ""}`;
}
export function createUpload(box: string, files: readonly File[]) {
  const key = requestKey();
  const batch = files.map((file) => ({
    file,
    path: file.webkitRelativePath || file.name,
  }));
  let active: XMLHttpRequest | undefined;
  return Object.freeze({
    key,
    cancel: () => active?.abort(),
    run: (progress: (fraction: number) => void): Promise<Receipt> =>
      new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        active = xhr;
        xhr.open("POST", `/api/v1/boxes/${encodeURIComponent(box)}/uploads`);
        xhr.setRequestHeader("Idempotency-Key", key);
        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable) progress(event.loaded / event.total);
        };
        xhr.onerror = () => reject(new Error("Upload connection failed"));
        xhr.onabort = () =>
          reject(new DOMException("Upload cancelled", "AbortError"));
        xhr.onload = () => {
          try {
            const body = JSON.parse(xhr.responseText);
            if (
              (xhr.status >= 200 && xhr.status < 300) ||
              (body.code === "write_committed" && body.op_id > 0)
            )
              resolve({ op_id: body.op_id });
            else reject(new APIError(xhr.status, body));
          } catch (error) {
            reject(error);
          }
        };
        xhr.onloadend = () => {
          if (active === xhr) active = undefined;
        };
        const form = new FormData();
        form.append(
          "manifest",
          JSON.stringify({
            files: batch.map(({ file, path }) => ({
              path,
              last_modified: file.lastModified,
            })),
          }),
        );
        for (const { file, path } of batch) form.append("files", file, path);
        xhr.send(form);
      }),
  });
}

export type MediaJob = components["schemas"]["MediaJob"];
export function mediaPath(file: ArchiveFile, recipe: string) {
  return `/files/${encodeURIComponent(file.id)}/derived/${encodeURIComponent(recipe)}`;
}
export async function mediaAction(
  path: string,
  action: "request" | "retry" | "cancel",
  signal?: AbortSignal,
): Promise<MediaJob> {
  return result<MediaJob>(
    await fetch(`/api/v1${path}`, {
      method: action === "cancel" ? "DELETE" : "POST",
      headers: { "Content-Type": "application/json" },
      body:
        action === "cancel"
          ? undefined
          : JSON.stringify({ retry: action === "retry" }),
      signal,
    }),
  );
}
