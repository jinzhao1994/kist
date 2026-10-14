import { bytesToHex } from "@noble/hashes/utils.js";
import { sha256 } from "@noble/hashes/sha2.js";

export const HASH_CHUNK_BYTES = 4 * 1024 * 1024;

// Runs in a worker; at most one chunk is materialized at a time.
export async function hashFile(
  file: Blob,
  progress: (bytes: number) => void,
): Promise<{ sha256: string; size: number }> {
  const hash = sha256.create();
  let bytes = 0;
  while (bytes < file.size) {
    const chunk = new Uint8Array(
      await file.slice(bytes, bytes + HASH_CHUNK_BYTES).arrayBuffer(),
    );
    if (!chunk.length) throw new Error("File became unreadable");
    hash.update(chunk);
    bytes += chunk.length;
    progress(bytes);
  }
  return { sha256: bytesToHex(hash.digest()), size: bytes };
}
