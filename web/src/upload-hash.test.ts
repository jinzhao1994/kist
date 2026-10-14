import { Blob } from "node:buffer";
import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";
import { hashFile, HASH_CHUNK_BYTES } from "./upload-hash";

describe("incremental upload hashing", () => {
  it.each([
    0,
    1,
    HASH_CHUNK_BYTES - 1,
    HASH_CHUNK_BYTES,
    HASH_CHUNK_BYTES + 19,
  ])("matches independent SHA-256 for %i bytes", async (size) => {
    const bytes = Buffer.alloc(size, 173);
    const blob = new Blob([bytes]);
    const progress: number[] = [];
    const result = await hashFile(blob as unknown as globalThis.Blob, (value) =>
      progress.push(value),
    );
    expect(result).toEqual({
      sha256: createHash("sha256").update(bytes).digest("hex"),
      size,
    });
    if (size) expect(progress.at(-1)).toBe(size);
  });

  it("never materializes more than one bounded slice", async () => {
    const size = HASH_CHUNK_BYTES * 3 + 7;
    let pending = false;
    const source = {
      size,
      slice(start: number, end: number) {
        expect(pending).toBe(false);
        expect(end - start).toBeLessThanOrEqual(HASH_CHUNK_BYTES);
        pending = true;
        return {
          async arrayBuffer() {
            await Promise.resolve();
            pending = false;
            return new ArrayBuffer(Math.min(end, size) - start);
          },
        };
      },
    };
    const result = await hashFile(
      source as Blob as unknown as globalThis.Blob,
      () => {},
    );
    expect(result.size).toBe(size);
    expect(result.sha256).toBe(
      createHash("sha256").update(Buffer.alloc(size)).digest("hex"),
    );
  });
});
