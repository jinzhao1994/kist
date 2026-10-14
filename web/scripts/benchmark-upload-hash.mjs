// Measures the production incremental hasher in Node, not browser memory.
import { openAsBlob, statSync } from "node:fs";
import {
  Worker,
  isMainThread,
  parentPort,
  workerData,
} from "node:worker_threads";
import { hashFile } from "../src/upload-hash.ts";
if (isMainThread) {
  if (!process.argv[2])
    throw new Error(
      "Usage: node web/scripts/benchmark-upload-hash.mjs /absolute/path/to/test-file",
    );
  const worker = new Worker(new URL(import.meta.url), {
    workerData: process.argv[2],
  });
  let beats = 0,
    maxGap = 0,
    previous = performance.now();
  const timer = setInterval(() => {
    const now = performance.now();
    maxGap = Math.max(maxGap, now - previous);
    previous = now;
    beats++;
  }, 10);
  worker.on("message", (result) =>
    console.log(
      JSON.stringify({
        ...result,
        mainThreadBeats: beats,
        maxMainThreadGapMs: maxGap,
      }),
    ),
  );
  worker.on("exit", (code) => {
    clearInterval(timer);
    process.exitCode = code;
  });
  worker.on("error", (error) => {
    console.error(error);
    process.exitCode = 1;
  });
} else {
  let peak = { rss: 0, heapUsed: 0, arrayBuffers: 0, external: 0 };
  const sample = () => {
    const m = process.memoryUsage();
    for (const k of Object.keys(peak)) peak[k] = Math.max(peak[k], m[k]);
  };
  sample();
  const before = process.memoryUsage();
  const start = performance.now();
  const source = await openAsBlob(workerData);
  if (source.size !== statSync(workerData).size)
    throw new Error(
      "Node file-backed Blob reports an incorrect size; this run is invalid",
    );
  const result = await hashFile(source, sample);
  sample();
  parentPort.postMessage({
    node: process.version,
    file: workerData,
    ...result,
    elapsedMs: performance.now() - start,
    before,
    peak,
  });
}
