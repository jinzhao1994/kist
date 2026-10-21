vi.mock("./upload-integrity", () => ({
  calculateIntegrity: vi.fn(async (files: File[]) =>
    files.map((file) => ({
      sha256: "0".repeat(64),
      size: file.size,
    })),
  ),
}));
import { afterEach, expect, it, vi } from "vitest";
import { APIError, createWrite } from "./api";
afterEach(() => vi.unstubAllGlobals());
it("retains the key and original body across a lost response", async () => {
  const fetch = vi
    .fn()
    .mockRejectedValueOnce(new TypeError("Disconnected"))
    .mockResolvedValueOnce(new Response('{"op_id":7}'));
  vi.stubGlobal("fetch", fetch);
  const body = { name: "First" };
  const write = createWrite("POST", "/boxes", body);
  body.name = "Changed";
  await expect(write.run()).rejects.toThrow("Disconnected");
  await expect(write.run()).resolves.toEqual({ op_id: 7 });
  expect(fetch.mock.calls[0][1].headers["Idempotency-Key"]).toBe(
    fetch.mock.calls[1][1].headers["Idempotency-Key"],
  );
  expect(fetch.mock.calls[1][1].body).toBe('{"name":"First"}');
  expect(createWrite("POST", "/boxes", body).key).not.toBe(write.key);
});
it("distinguishes a committed write from an unknown result", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValueOnce(
        new Response('{"code":"write_committed","op_id":9}', { status: 409 }),
      )
      .mockResolvedValueOnce(
        new Response('{"code":"write_outcome_unknown","message":"unknown"}', {
          status: 409,
        }),
      ),
  );
  await expect(createWrite("POST", "/boxes", {}).run()).resolves.toEqual({
    op_id: 9,
  });
  await expect(createWrite("POST", "/boxes", {}).run()).rejects.toBeInstanceOf(
    APIError,
  );
});
it("recovers unresolved JSON writes after remount and retires successful keys", async () => {
  sessionStorage.clear();
  const first = createWrite("POST", "/boxes", { name: "Archive" });
  const restored = createWrite("POST", "/boxes", { name: "Archive" });
  expect(restored.key).toBe(first.key);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response('{"op_id":11}')),
  );
  await restored.run();
  expect(createWrite("POST", "/boxes", { name: "Archive" }).key).not.toBe(
    first.key,
  );
  sessionStorage.clear();
});
it("retains upload bytes, manifest and key after a cancelled connection", async () => {
  const sent: FormData[] = [];
  const urls: string[] = [];
  const headers: string[] = [];
  const instances: XHR[] = [];
  class XHR {
    upload: {
      onload?: () => void;
      onprogress?: (event: {
        lengthComputable: boolean;
        loaded: number;
        total: number;
      }) => void;
    } = {};
    status = 200;
    responseText = '{"op_id":12}';
    onabort?: () => void;
    onload?: () => void;
    onloadend?: () => void;
    constructor() {
      instances.push(this);
    }
    open(_method: string, url: string) {
      urls.push(url);
    }
    setRequestHeader(_name: string, key: string) {
      headers.push(key);
    }
    send(form: FormData) {
      sent.push(form);
    }
    abort() {
      this.onabort?.();
      this.onloadend?.();
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  const { createUpload } = await import("./api");
  const upload = createUpload(
    "box",
    [new File(["original"], "旅行.txt", { lastModified: 1234 })],
    "child",
  );
  const progress = vi.fn();
  const first = upload.run(progress);
  const rejected = expect(first).rejects.toMatchObject({ name: "AbortError" });
  await vi.waitFor(() => expect(instances).toHaveLength(1));
  instances[0].upload.onprogress?.({
    lengthComputable: true,
    loaded: 1,
    total: 2,
  });
  upload.cancel();
  await rejected;
  expect(progress).toHaveBeenCalledWith(0.5);
  const phases = vi.fn();
  let completed = false;
  const retry = upload.run(progress, phases);
  void retry.then(() => {
    completed = true;
  });
  await vi.waitFor(() => expect(instances).toHaveLength(2));
  instances[1].upload.onload?.();
  await Promise.resolve();
  expect(phases.mock.calls.map(([phase]) => phase)).toEqual([
    "hashing",
    "uploading",
    "processing",
  ]);
  expect(completed).toBe(false);
  instances[1].onload?.();
  await expect(retry).resolves.toEqual({ op_id: 12 });
  expect(headers).toEqual([upload.key, upload.key]);
  expect(urls).toEqual([
    "/api/v1/boxes/box/uploads?folder=child",
    "/api/v1/boxes/box/uploads?folder=child",
  ]);
  expect(sent[0].get("manifest")).toBe(sent[1].get("manifest"));
  expect(JSON.parse(String(sent[1].get("manifest"))).files).toEqual([
    { path: "旅行.txt", last_modified: 1234, sha256: "0".repeat(64), size: 8 },
  ]);
  expect((sent[1].get("files") as File).size).toBe(8);
});

it("never starts upload when original-file hashing fails", async () => {
  const { calculateIntegrity } = await import("./upload-integrity");
  vi.mocked(calculateIntegrity).mockRejectedValueOnce(
    new Error("Unreadable original"),
  );
  const xhr = vi.fn();
  vi.stubGlobal("XMLHttpRequest", xhr);
  const { createUpload } = await import("./api");
  await expect(
    createUpload("box", [new File(["x"], "x.txt")]).run(() => {}),
  ).rejects.toThrow("Unreadable original");
  expect(xhr).not.toHaveBeenCalled();
});

it("preserves local receipts when recovering a client write", async () => {
  sessionStorage.clear();
  const receipt = { local_id: "offline-command-identity", noop: false };
  const fetch = vi
    .fn()
    .mockRejectedValueOnce(new TypeError("Response lost"))
    .mockResolvedValueOnce(new Response(JSON.stringify(receipt)));
  vi.stubGlobal("fetch", fetch);
  const first = createWrite("POST", "/boxes", { name: "Offline" });
  await expect(first.run()).rejects.toThrow("Response lost");
  const restored = createWrite("POST", "/boxes", { name: "Offline" });
  expect(restored.key).toBe(first.key);
  await expect(restored.run()).resolves.toEqual(receipt);
  expect(createWrite("POST", "/boxes", { name: "Offline" }).key).not.toBe(
    first.key,
  );
});

it("retires a successful local no-op without inventing an operation ID", async () => {
  sessionStorage.clear();
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response('{"noop":true}')),
  );
  const write = createWrite("PUT", "/boxes/one", { name: "Unchanged" });
  await expect(write.run()).resolves.toEqual({ noop: true });
  expect(createWrite("PUT", "/boxes/one", { name: "Unchanged" }).key).not.toBe(
    write.key,
  );
});
