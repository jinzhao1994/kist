import { APIError, get, requestKey, result, type Receipt } from "./api";
import type { components } from "./api-schema";
import { calculateIntegrity } from "./upload-integrity";

type FileSpec = components["schemas"]["UploadFile"];
type WireState = components["schemas"]["UploadState"];
type State = Omit<WireState, "manifest" | "offsets"> & {
  manifest: Omit<WireState["manifest"], "files"> & { files: FileSpec[] };
  offsets: number[];
};
type Status = components["schemas"]["SessionStatus"];
export type UploadPhase = "hashing" | "uploading" | "processing";
export type PendingUpload = {
  version: 1;
  key: string;
  box: string;
  folder: string;
  id?: string;
  files?: FileSpec[];
};

export class ResumeError extends Error {
  constructor(
    public readonly kind: "source" | "storage" | "protocol" | "selection",
  ) {
    super(`Cannot resume upload: ${kind}`);
  }
}

export const pendingUploadKey = (box: string, folder: string) =>
  `kist.resumable-upload.v1:${box}:${folder}`;

export function loadUpload(
  box: string,
  folder: string,
): PendingUpload | undefined {
  let raw: string | null;
  try {
    raw = localStorage.getItem(pendingUploadKey(box, folder));
  } catch {
    throw new ResumeError("storage");
  }
  if (!raw) return undefined;
  try {
    const value = JSON.parse(raw) as PendingUpload;
    if (
      value.version !== 1 ||
      value.box !== box ||
      value.folder !== folder ||
      typeof value.key !== "string" ||
      !/^[\w.-]{8,128}$/.test(value.key) ||
      (value.id !== undefined && !/^[0-9a-f]{64}$/.test(value.id)) ||
      (value.files !== undefined &&
        (!Array.isArray(value.files) ||
          !value.files.length ||
          value.files.some(
            (file) =>
              typeof file.path !== "string" ||
              typeof file.last_modified !== "number" ||
              !Number.isSafeInteger(file.size) ||
              file.size < 0 ||
              !/^[0-9a-f]{64}$/.test(file.sha256),
          )))
    )
      throw new Error("Invalid stored upload");
    return value;
  } catch {
    throw new ResumeError("storage");
  }
}

function sameFiles(a: FileSpec[], b: FileSpec[]) {
  const expected = new Map(a.map((file) => [file.path, file]));
  return (
    a.length === b.length &&
    expected.size === a.length &&
    new Set(b.map((file) => file.path)).size === b.length &&
    b.every((file) => {
      const before = expected.get(file.path);
      return (
        before &&
        before.size === file.size &&
        before.sha256 === file.sha256 &&
        before.last_modified === file.last_modified
      );
    })
  );
}

// Persist identity before creating any remote staging. Progress always comes
// from the server; local storage is only a pointer and an original-file identity.
export function resumableUpload(
  box: string,
  folder: string,
  chunkBytes: number,
  changed: (pending: PendingUpload | undefined) => void = () => {},
) {
  if (
    !Number.isSafeInteger(chunkBytes) ||
    chunkBytes < 1 ||
    chunkBytes > 8 * 1024 * 1024
  )
    throw new ResumeError("protocol");
  let pending = loadUpload(box, folder);
  let selected: readonly File[] | undefined;
  let verified: FileSpec[] | undefined;
  let controller: AbortController | undefined;
  let confirmed = 0;
  const storage = pendingUploadKey(box, folder);

  function save(value: PendingUpload) {
    try {
      const existing = loadUpload(box, folder);
      if (existing && existing.key !== value.key)
        throw new ResumeError("storage");
      localStorage.setItem(storage, JSON.stringify(value));
    } catch {
      throw new ResumeError("storage");
    }
    pending = value;
    changed(value);
  }
  function clear() {
    try {
      if (loadUpload(box, folder)?.key === pending?.key)
        localStorage.removeItem(storage);
    } catch {
      /* A retained pointer can still recover the durable receipt. */
    }
    pending = undefined;
    selected = undefined;
    verified = undefined;
    changed(undefined);
  }
  function done(status: Status): Receipt | undefined {
    if (status.op_id && status.op_id > 0) {
      clear();
      return { op_id: status.op_id };
    }
    return undefined;
  }
  function stateOf(status: Status): State {
    const state = status.session;
    if (!state) throw new ResumeError("protocol");
    const files = state.manifest.files;
    const offsets = state.offsets;
    if (
      !state ||
      !pending?.files ||
      state.version !== 1 ||
      !/^[0-9a-f]{64}$/.test(state.id) ||
      (pending.id && pending.id !== state.id) ||
      state.manifest.box !== box ||
      state.manifest.folder !== folder ||
      !files ||
      !offsets ||
      !sameFiles(pending.files, files) ||
      offsets.length !== files.length ||
      offsets.some(
        (offset, index) =>
          !Number.isSafeInteger(offset) ||
          offset < 0 ||
          offset > files[index].size,
      )
    )
      throw new ResumeError("protocol");
    if (!pending.id) save({ ...pending, id: state.id });
    const total = files.reduce((sum, file) => sum + file.size, 0);
    confirmed = total
      ? offsets.reduce((sum, offset) => sum + offset, 0) / total
      : 1;
    return { ...state, manifest: { ...state.manifest, files }, offsets };
  }
  async function command(
    path: string,
    method: "POST" | "DELETE",
    signal: AbortSignal,
    body?: unknown,
    key?: string,
  ): Promise<Status> {
    return result<Status>(
      await fetch(`/api/v1/upload-sessions${path}`, {
        method,
        signal,
        headers: {
          "Content-Type": "application/json",
          ...(key ? { "Idempotency-Key": key } : {}),
        },
        body: method === "DELETE" ? undefined : JSON.stringify(body ?? {}),
      }),
    );
  }
  async function status(signal: AbortSignal): Promise<Status> {
    if (!pending?.files) throw new ResumeError("selection");
    if (pending.id)
      return get<Status>(`/upload-sessions/${pending.id}`, signal);
    return command(
      "",
      "POST",
      signal,
      { version: 1, box, folder, files: pending.files },
      pending.key,
    );
  }
  function begin() {
    if (controller) throw new Error("Upload operation already running");
    controller = new AbortController();
    return controller;
  }

  function put(
    id: string,
    index: number,
    offset: number,
    chunk: Blob,
    digest: string,
    signal: AbortSignal,
    progress: (bytes: number) => void,
  ) {
    return new Promise<Status>((resolve, reject) => {
      signal.throwIfAborted();
      const xhr = new XMLHttpRequest();
      const abort = () => {
        xhr.abort();
        reject(signal.reason);
      };
      const cleanup = () => signal.removeEventListener("abort", abort);
      xhr.open(
        "PUT",
        `/api/v1/upload-sessions/${id}/files/${index}?offset=${offset}`,
      );
      xhr.setRequestHeader("Content-Type", "application/octet-stream");
      xhr.setRequestHeader("Upload-Length", String(chunk.size));
      xhr.setRequestHeader("Upload-SHA256", digest);
      xhr.upload.onprogress = (event) =>
        progress(Math.min(event.loaded, chunk.size));
      xhr.onerror = () => {
        cleanup();
        reject(new Error("Upload connection failed"));
      };
      xhr.onabort = () => {
        cleanup();
        reject(signal.reason ?? new DOMException("Paused", "AbortError"));
      };
      xhr.onload = () => {
        cleanup();
        try {
          const body = JSON.parse(xhr.responseText);
          if (
            (xhr.status >= 200 && xhr.status < 300) ||
            (body.code === "write_committed" && body.op_id > 0)
          )
            resolve(body);
          else reject(new APIError(xhr.status, body));
        } catch (error) {
          reject(error);
        }
      };
      signal.addEventListener("abort", abort, { once: true });
      try {
        xhr.send(chunk);
      } catch (error) {
        cleanup();
        reject(error);
      }
    });
  }

  return {
    pending: () => pending,
    hasFiles: () => !!selected,
    confirmedProgress: () => confirmed,
    select(files: readonly File[]) {
      if (controller) throw new Error("Cannot replace an active upload");
      selected = [...files];
      verified = undefined;
    },
    pause() {
      controller?.abort(new DOMException("Paused", "AbortError"));
    },
    async check(): Promise<{ receipt?: Receipt; state?: State }> {
      const attempt = begin();
      try {
        const response = await status(attempt.signal);
        const receipt = done(response);
        return receipt ? { receipt } : { state: stateOf(response) };
      } finally {
        controller = undefined;
      }
    },
    async cancel(): Promise<Receipt | undefined> {
      // Pause first and wait for run() to settle before calling cancel().
      const attempt = begin();
      try {
        if (!pending?.files) {
          clear();
          return;
        }
        const response = await status(attempt.signal);
        const receipt = done(response);
        if (receipt) return receipt;
        const state = stateOf(response);
        const canceled = await command(
          `/${state.id}`,
          "DELETE",
          attempt.signal,
        );
        const committed = done(canceled);
        if (committed) return committed;
        clear();
      } catch (error) {
        if (
          !pending?.id &&
          error instanceof APIError &&
          [
            "upload_quota",
            "upload_invalid",
            "invalid_request",
            "request_conflict",
            "box_not_found",
            "folder_in_another_box",
          ].includes(error.body.code)
        ) {
          clear();
          return;
        }
        if (
          error instanceof APIError &&
          [
            "upload_session_missing",
            "upload_session_expired",
            "upload_session_canceled",
          ].includes(error.body.code)
        ) {
          // Expired sessions can still be explicitly discarded server-side.
          if (pending?.id) {
            const response = await command(
              `/${pending.id}`,
              "DELETE",
              attempt.signal,
            );
            const receipt = done(response);
            if (receipt) return receipt;
          }
          clear();
          return;
        }
        throw error;
      } finally {
        controller = undefined;
      }
    },
    async run(
      progress: (fraction: number) => void,
      phase: (phase: UploadPhase) => void,
    ): Promise<Receipt> {
      const attempt = begin();
      const signal = attempt.signal;
      try {
        // Refresh recovery checks for a committed receipt before requiring
        // originals; otherwise identity must be proven before sending any block.
        if (pending?.files) {
          const response = await status(signal);
          const receipt = done(response);
          if (receipt) return receipt;
          stateOf(response);
        }
        if (!selected?.length) throw new ResumeError("selection");
        if (!pending) save({ version: 1, key: requestKey(), box, folder });
        phase("hashing");
        progress(0);
        if (!verified) {
          const hashes = await calculateIntegrity(selected, progress, signal);
          signal.throwIfAborted();
          const identities = selected.map((file, index) => ({
            path: file.webkitRelativePath || file.name,
            last_modified: file.lastModified,
            ...hashes[index],
          }));
          if (pending!.files && !sameFiles(pending!.files, identities))
            throw new ResumeError("source");
          if (!sameFiles(identities, identities))
            throw new ResumeError("source");
          save({ ...pending!, files: identities });
          verified = identities;
        }
        let response = await status(signal);
        let receipt = done(response);
        if (receipt) return receipt;
        let state = stateOf(response);
        response = await command(`/${state.id}/resume`, "POST", signal);
        receipt = done(response);
        if (receipt) return receipt;
        state = stateOf(response);
        phase("uploading");
        const byPath = new Map(
          selected.map((file) => [file.webkitRelativePath || file.name, file]),
        );
        const total = state.manifest.files.reduce(
          (sum, file) => sum + file.size,
          0,
        );
        const received = () =>
          state.offsets.reduce((sum, offset) => sum + offset, 0);
        progress(total ? received() / total : 1);
        for (let index = 0; index < state.manifest.files.length; index++) {
          let failures = 0;
          const file = byPath.get(state.manifest.files[index].path)!;
          while (state.offsets[index] < file.size) {
            signal.throwIfAborted();
            const offset = state.offsets[index];
            const chunk = file.slice(
              offset,
              Math.min(file.size, offset + chunkBytes),
            );
            const [identity] = await calculateIntegrity(
              [chunk],
              () => {},
              signal,
            );
            signal.throwIfAborted();
            const before = received();
            try {
              response = await put(
                state.id,
                index,
                offset,
                chunk,
                identity.sha256,
                signal,
                (bytes) => progress(total ? (before + bytes) / total : 1),
              );
              receipt = done(response);
              if (receipt) return receipt;
              const next = stateOf(response);
              if (next.offsets[index] < offset + chunk.size)
                throw new ResumeError("protocol");
              state = next;
              failures = 0;
              progress(total ? received() / total : 1);
            } catch (error) {
              signal.throwIfAborted();
              if (
                !(error instanceof APIError) ||
                !["upload_chunk_integrity", "upload_offset"].includes(
                  error.body.code,
                ) ||
                ++failures > 2
              )
                throw error;
              response = await status(signal);
              receipt = done(response);
              if (receipt) return receipt;
              state = stateOf(response);
            }
          }
        }
        phase("processing");
        response = await command(`/${state.id}/complete`, "POST", signal);
        receipt = done(response);
        if (!receipt) throw new ResumeError("protocol");
        return receipt;
      } finally {
        controller = undefined;
      }
    },
  };
}
