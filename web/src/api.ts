import type { components, operations } from "./api-schema";
export type Box = components["schemas"]["Box"];
export type ArchiveFile = components["schemas"]["File"];
export type FileDetail = components["schemas"]["FileDetail"];
export type Tag = components["schemas"]["Tag"];
export type Receipt = components["schemas"]["RequestResult"];
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
export function createWrite(
  method: "POST" | "PUT" | "DELETE",
  path: string,
  body?: unknown,
) {
  const key = crypto.randomUUID();
  const serialized = body === undefined ? undefined : JSON.stringify(body);
  return Object.freeze({
    key,
    run: async (signal?: AbortSignal): Promise<Receipt> =>
      result<Receipt>(
        await fetch(`/api/v1${path}`, {
          method,
          signal,
          headers: {
            "Content-Type": "application/json",
            "Idempotency-Key": key,
          },
          body: serialized,
        }),
      ),
  });
}
export function contentURL(file: ArchiveFile, download = false) {
  return `/api/v1/files/${encodeURIComponent(file.id)}/content${download ? "?download=true" : ""}`;
}
export function createUpload(box: string, files: readonly File[]) {
  const key = crypto.randomUUID();
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
