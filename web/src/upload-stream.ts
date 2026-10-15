import { createSHA256 } from "hash-wasm";
import { sha256 } from "@noble/hashes/sha2.js";
import { bytesToHex } from "@noble/hashes/utils.js";

// WASM is bundled locally and works over HTTP. Keep a checked JS fallback for
// browsers whose WebAssembly initialization is unavailable or blocked.
async function digestPair() {
  try {
    const whole = await createSHA256();
    const block = await createSHA256();
    return {
      update: (bytes: Uint8Array) => {
        whole.update(bytes);
      },
      peek: () => {
        const state = whole.save();
        const digest = whole.digest("hex");
        whole.load(state);
        return digest;
      },
      block: (bytes: Uint8Array) => block.init().update(bytes).digest("hex"),
    };
  } catch {
    const whole = sha256.create();
    return {
      update: (bytes: Uint8Array) => {
        whole.update(bytes);
      },
      peek: () => bytesToHex(whole.clone().digest()),
      block: (bytes: Uint8Array) => bytesToHex(sha256(bytes)),
    };
  }
}

// One file reader with bounded memory; the bytes hashed are the bytes sent.
export class UploadStream {
  private hashes = digestPair();
  private offset = 0;
  constructor(private file: Blob) {}

  async prefix(offset: number) {
    if (!Number.isSafeInteger(offset) || offset < 0 || offset > this.file.size)
      throw new Error("Invalid prefix");
    const hash = await this.hashes;
    while (this.offset < offset) {
      const bytes = await this.file
        .slice(this.offset, Math.min(offset, this.offset + 4 * 1024 * 1024))
        .arrayBuffer();
      if (!bytes.byteLength) throw new Error("Unreadable file");
      hash.update(new Uint8Array(bytes));
      this.offset += bytes.byteLength;
    }
    return hash.peek();
  }

  async next(size: number) {
    const hash = await this.hashes;
    const offset = this.offset;
    const data = await this.file.slice(offset, offset + size).arrayBuffer();
    if (!data.byteLength && offset < this.file.size)
      throw new Error("Unreadable file");
    const bytes = new Uint8Array(data);
    hash.update(bytes);
    this.offset += bytes.byteLength;
    return {
      offset,
      data,
      sha256: hash.block(bytes),
      whole: this.offset === this.file.size ? hash.peek() : undefined,
    };
  }
}
