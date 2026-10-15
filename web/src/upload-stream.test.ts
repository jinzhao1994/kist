import { Blob } from "node:buffer";
import { createHash } from "node:crypto";
import { expect, it, vi } from "vitest";
import { UploadStream } from "./upload-stream";
const digest = (bytes: Uint8Array) =>
  createHash("sha256").update(bytes).digest("hex");
it.each([0, 1, 19, 1024 * 1024 + 3])(
  "hashes and returns each byte only once (%i bytes)",
  async (size) => {
    const bytes = Buffer.alloc(size, 173);
    const source = new Blob([bytes]);
    let read = 0;
    const file = {
      size,
      slice(start: number, end: number) {
        read += Math.min(size, end) - start;
        return source.slice(start, end);
      },
    };
    const stream = new UploadStream(file as unknown as globalThis.Blob);
    const empty = await stream.prefix(0);
    let whole = empty;
    for (let offset = 0; offset < size;) {
      const chunk = await stream.next(65536);
      expect(chunk.offset).toBe(offset);
      expect(chunk.sha256).toBe(digest(new Uint8Array(chunk.data)));
      offset += chunk.data.byteLength;
      if (chunk.whole) whole = chunk.whole;
    }
    expect(whole).toBe(digest(bytes));
    expect(read).toBe(size);
  },
);
it("continues whole-file hashing after verifying the resumed prefix", async () => {
  const bytes = Buffer.from("abcdefghij");
  const stream = new UploadStream(
    new Blob([bytes]) as unknown as globalThis.Blob,
  );
  expect(await stream.prefix(4)).toBe(digest(bytes.subarray(0, 4)));
  const chunk = await stream.next(8);
  expect(new Uint8Array(chunk.data)).toEqual(new Uint8Array(bytes.subarray(4)));
  expect(chunk.whole).toBe(digest(bytes));
});

it("rejects a source that stops returning bytes during prefix verification", async () => {
  const stream = new UploadStream({
    size: 4,
    slice: () => new Blob([]),
  } as unknown as globalThis.Blob);
  await expect(stream.prefix(4)).rejects.toThrow("Unreadable file");
});
it("retains correct block and whole hashes when WASM cannot initialize", async () => {
  vi.resetModules();
  vi.doMock("hash-wasm", () => ({
    createSHA256: async () => {
      throw new Error("unavailable");
    },
  }));
  try {
    const { UploadStream: FallbackStream } = await import("./upload-stream");
    const data = Buffer.from("fallback-check");
    const stream = new FallbackStream(
      new Blob([data]) as unknown as globalThis.Blob,
    );
    expect(await stream.prefix(3)).toBe(digest(data.subarray(0, 3)));
    const chunk = await stream.next(100);
    expect(chunk.sha256).toBe(digest(data.subarray(3)));
    expect(chunk.whole).toBe(digest(data));
  } finally {
    vi.doUnmock("hash-wasm");
    vi.resetModules();
  }
});
