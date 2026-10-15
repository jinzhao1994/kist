import { beforeEach, expect, it, vi } from "vitest";
import { createHash } from "node:crypto";
import {
  resumableUpload,
  loadUpload,
  pendingUploadKey,
  ResumeError,
} from "./resumable-upload";
import { UploadStream } from "./upload-stream";
import { calculateIntegrity } from "./upload-integrity";

function bytes(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as ArrayBuffer);
    reader.onerror = reject;
    reader.readAsArrayBuffer(blob);
  });
}
vi.mock("./upload-integrity", () => ({
  calculateIntegrity: vi.fn(
    async (
      files: Blob[],
      progress: (n: number) => void,
      signal: AbortSignal,
    ) => {
      const result = [];
      for (const file of files) {
        signal.throwIfAborted();
        const raw = await bytes(file);
        result.push({
          sha256: createHash("sha256")
            .update(new Uint8Array(raw))
            .digest("hex"),
          size: raw.byteLength,
        });
        progress(1);
      }
      return result;
    },
  ),
}));
vi.mock("./upload-stream-client", () => ({
  uploadStreamWorker: (signal: AbortSignal, measured: (ms: number) => void) => {
    let stream: UploadStream;
    return {
      async open(file: Blob, offset: number) {
        signal.throwIfAborted();
        stream = new UploadStream({
          size: file.size,
          slice: (start: number, end: number) => ({
            arrayBuffer: () => bytes(file.slice(start, end)),
          }),
        } as Blob);
        return { prefix: await stream.prefix(offset) };
      },
      async next(size: number) {
        signal.throwIfAborted();
        const value = await stream.next(size);
        measured(1);
        return { ...value, hashMs: 1 };
      },
      close() {},
    };
  },
}));
const id = "a".repeat(64);
const original = () =>
  new File(["abcdefghij"], "source.bin", { lastModified: 1000 });
const identity = (value: string) =>
  createHash("sha256").update(value).digest("hex");
let expectedDigests: string[];
let state: any;
let archived: number;
let sent: { offset: number; data: string }[];
let loseResponse: boolean;
let rejectChunk: boolean;
let holdChunk: boolean;
let creates: number;
let completes: number;
let canceled: number;
function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}
function status() {
  return archived ? { op_id: archived } : { session: structuredClone(state) };
}
beforeEach(() => {
  localStorage.clear();
  state = undefined;
  expectedDigests = [identity("abcdefghij")];
  archived = 0;
  sent = [];
  creates = 0;
  completes = 0;
  canceled = 0;
  loseResponse = false;
  rejectChunk = false;
  holdChunk = false;
  vi.clearAllMocks();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      options?.signal?.throwIfAborted();
      if (path === "/api/v1/upload-sessions") {
        creates++;
        const body = JSON.parse(options!.body as string);
        state ??= {
          version: 1,
          id,
          manifest: {
            box: body.box,
            folder: body.folder,
            target: "b".repeat(64),
            protocol: body.version === 2 ? 2 : undefined,
            files: [...body.files].sort((a, b) =>
              a.path < b.path ? -1 : a.path > b.path ? 1 : 0,
            ),
          },
          offsets: body.files.map(() => 0),
          expires: "2030-01-01T00:00:00Z",
        };
      } else if (path.endsWith("/prefix")) {
        const body = JSON.parse(options!.body as string);
        const prefix = sent
          .filter((chunk) => chunk.offset < body.offset)
          .map((chunk) => chunk.data)
          .join("")
          .slice(0, body.offset);
        if (body.sha256 !== identity(prefix))
          return response(
            { code: "upload_integrity_mismatch", message: "wrong prefix" },
            400,
          );
      } else if (path.endsWith("/complete")) {
        if (state.manifest.protocol === 2) {
          const body = JSON.parse(options!.body as string);
          expect(body.digests).toEqual(expectedDigests);
        }
        completes++;
        archived = 31;
      } else if (options?.method === "DELETE") {
        canceled++;
        return response({});
      }
      return response(status());
    }),
  );
  class XHR {
    upload: { onprogress?: (event: any) => void } = {};
    status = 200;
    responseText = "";
    onload?: () => void;
    onerror?: () => void;
    onabort?: () => void;
    url = "";
    headers: Record<string, string> = {};
    aborted = false;
    open(_method: string, url: string) {
      this.url = url;
    }
    setRequestHeader(key: string, value: string) {
      this.headers[key] = value;
    }
    abort() {
      this.aborted = true;
      this.onabort?.();
    }
    async send(blob: Blob) {
      const raw = await bytes(blob);
      if (this.aborted) return;
      const offset = Number(this.url.split("offset=")[1]);
      const data = new TextDecoder().decode(raw);
      sent.push({ offset, data });
      expect(this.headers["Upload-SHA256"]).toBe(identity(data));
      expect(this.headers["Upload-Length"]).toBe(String(raw.byteLength));
      if (holdChunk) return;
      if (rejectChunk) {
        rejectChunk = false;
        this.status = 400;
        this.responseText = JSON.stringify({
          code: "upload_chunk_integrity",
          class: "invalid",
          message: "bad block",
        });
        this.onload?.();
        return;
      }
      const index = Number(this.url.match(/\/files\/(\d+)/)![1]);
      state.offsets[index] = offset + raw.byteLength;
      if (loseResponse) {
        loseResponse = false;
        this.onerror?.();
        return;
      }
      this.responseText = JSON.stringify(status());
      this.onload?.();
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
});

it("persists identity and sends only missing blocks after a lost response and remount", async () => {
  const first = resumableUpload("box", "folder", 4);
  first.select([original()]);
  loseResponse = true;
  await expect(
    first.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow("connection");
  expect(loadUpload("box", "folder")?.files?.[0].sha256).toBe(
    identity("abcdefghij"),
  );
  expect(state.offsets).toEqual([4]);
  const restored = resumableUpload("box", "folder", 4);
  restored.select([original()]);
  expect(
    await restored.run(
      () => {},
      () => {},
    ),
  ).toEqual({ op_id: 31 });
  expect(sent).toEqual([
    { offset: 0, data: "abcd" },
    { offset: 4, data: "efgh" },
    { offset: 8, data: "ij" },
  ]);
  expect(completes).toBe(1);
  expect(loadUpload("box", "folder")).toBeUndefined();
});

it("rehashes reselected originals and refuses equal name size and mtime with different content", async () => {
  const first = resumableUpload("box", "folder", 4);
  first.select([original()]);
  loseResponse = true;
  await expect(
    first.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  const restored = resumableUpload("box", "folder", 4);
  restored.select([
    new File(["XXXXXXXXXX"], "source.bin", { lastModified: 1000 }),
  ]);
  await expect(
    restored.run(
      () => {},
      () => {},
    ),
  ).rejects.toMatchObject({ kind: "source" });
  expect(sent).toHaveLength(1);
  expect(completes).toBe(0);
  restored.select([original()]);
  expect(
    await restored.run(
      () => {},
      () => {},
    ),
  ).toEqual({ op_id: 31 });
});

it("retries only the rejected chunk and never restarts the whole batch", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  rejectChunk = true;
  await operation.run(
    () => {},
    () => {},
  );
  expect(sent.map((block) => block.offset)).toEqual([0, 0, 4, 8]);
  expect(completes).toBe(1);
});

it("pause retains staging and resumes from server progress", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  holdChunk = true;
  const running = operation.run(
    () => {},
    () => {},
  );
  const stopped = expect(running).rejects.toMatchObject({ name: "AbortError" });
  await vi.waitFor(() => expect(sent).toHaveLength(1));
  operation.pause();
  await stopped;
  expect(loadUpload("box", "folder")).toBeDefined();
  expect(canceled).toBe(0);
  holdChunk = false;
  await operation.run(
    () => {},
    () => {},
  );
  expect(sent.map((block) => block.offset)).toEqual([0, 0, 4, 8]);
});

it("recovers a receipt without reselecting files and clears the pointer", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  loseResponse = true;
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  archived = 42;
  const restored = resumableUpload("box", "folder", 4);
  expect(await restored.check()).toEqual({ receipt: { op_id: 42 } });
  expect(sent).toHaveLength(1);
  expect(completes).toBe(0);
  expect(loadUpload("box", "folder")).toBeUndefined();
});

it("cancel deletes staging but reports an already committed receipt", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  loseResponse = true;
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  await operation.cancel();
  expect(canceled).toBe(1);
  expect(loadUpload("box", "folder")).toBeUndefined();
});

it("requires originals after remount when there is no committed receipt", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  loseResponse = true;
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  await expect(
    resumableUpload("box", "folder", 4).run(
      () => {},
      () => {},
    ),
  ).rejects.toMatchObject({ kind: "selection" });
  expect(sent).toHaveLength(1);
});

it("saves before creating a session and fails clearly when persistence is unavailable", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  const save = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("storage full");
  });
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).rejects.toBeInstanceOf(ResumeError);
  expect(creates).toBe(0);
  expect(sent).toHaveLength(0);
  save.mockRestore();
  localStorage.setItem(pendingUploadKey("box", "folder"), "{broken");
  expect(() => loadUpload("box", "folder")).toThrow(ResumeError);
});

it("keeps a creation key across a lost creation response", async () => {
  const actual = fetch;
  let lost = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      const response = await actual(path, options);
      if (path === "/api/v1/upload-sessions" && lost) {
        lost = false;
        throw new Error("lost create");
      }
      return response;
    }),
  );
  const operation = resumableUpload("box", "folder", 4);
  operation.select([original()]);
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow("lost create");
  const key = loadUpload("box", "folder")!.key;
  const restored = resumableUpload("box", "folder", 4);
  restored.select([original()]);
  await restored.run(
    () => {},
    () => {},
  );
  const calls = vi
    .mocked(fetch)
    .mock.calls.filter(([path]) => path === "/api/v1/upload-sessions");
  expect(calls).toHaveLength(2);
  for (const [, options] of calls)
    expect(
      (options!.headers as Record<string, string>)["Idempotency-Key"],
    ).toBe(key);
});

it("uses the server manifest order while hashing and matching originals by path", async () => {
  const operation = resumableUpload("box", "folder", 4);
  operation.select([
    new File(["12345"], "z.bin", { lastModified: 1000 }),
    new File(["abc"], "a.bin", { lastModified: 1000 }),
  ]);
  await operation.run(
    () => {},
    () => {},
  );
  expect(sent).toEqual([
    { offset: 0, data: "abc" },
    { offset: 0, data: "1234" },
    { offset: 4, data: "5" },
  ]);
});

it("streams before whole-file hashing and resumes only missing bytes after remount", async () => {
  const first = resumableUpload("box", "folder", 4, () => {}, 2);
  first.select([original()]);
  loseResponse = true;
  await expect(
    first.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow("connection");
  expect(calculateIntegrity).not.toHaveBeenCalled();
  expect(sent).toEqual([{ offset: 0, data: "abcd" }]);
  const restored = resumableUpload("box", "folder", 4, () => {}, 2);
  restored.select([original()]);
  const metrics: any[] = [];
  await expect(
    restored.run(
      () => {},
      () => {},
      (m) => metrics.push(m),
    ),
  ).resolves.toEqual({ op_id: 31 });
  expect(sent).toEqual([
    { offset: 0, data: "abcd" },
    { offset: 4, data: "efgh" },
    { offset: 8, data: "ij" },
  ]);
  expect(metrics.at(-1).bytes).toBe(6);
  expect(completes).toBe(1);
});
it("rejects a reselected mismatching durable prefix before sending new bytes", async () => {
  const first = resumableUpload("box", "folder", 4, () => {}, 2);
  first.select([original()]);
  loseResponse = true;
  await expect(
    first.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  const restored = resumableUpload("box", "folder", 4, () => {}, 2);
  restored.select([
    new File(["XXXXefghij"], "source.bin", { lastModified: 1000 }),
  ]);
  await expect(
    restored.run(
      () => {},
      () => {},
    ),
  ).rejects.toThrow();
  expect(sent).toHaveLength(1);
  expect(completes).toBe(0);
});

it("seals multiple streamed files including empty files in server manifest order", async () => {
  expectedDigests = [identity("abc"), identity(""), identity("12345")];
  const operation = resumableUpload("box", "folder", 4, () => {}, 2);
  operation.select([
    new File(["12345"], "z.txt", { lastModified: 1000 }),
    new File([], "empty.txt", { lastModified: 1000 }),
    new File(["abc"], "a.txt", { lastModified: 1000 }),
  ]);
  await expect(
    operation.run(
      () => {},
      () => {},
    ),
  ).resolves.toEqual({ op_id: 31 });
  expect(sent).toEqual([
    { offset: 0, data: "abc" },
    { offset: 0, data: "1234" },
    { offset: 4, data: "5" },
  ]);
  expect(calculateIntegrity).not.toHaveBeenCalled();
});
