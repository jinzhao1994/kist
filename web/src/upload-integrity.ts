export type FileIntegrity = { sha256: string; size: number };

export function calculateIntegrity(
  files: readonly Blob[],
  progress: (fraction: number) => void,
  signal: AbortSignal,
): Promise<FileIntegrity[]> {
  return new Promise((resolve, reject) => {
    signal.throwIfAborted();
    const worker = new Worker(
      new URL("./upload-hash.worker.ts", import.meta.url),
      { type: "module" },
    );
    const cleanup = () => {
      worker.terminate();
      signal.removeEventListener("abort", abort);
    };
    const abort = () => {
      cleanup();
      reject(signal.reason);
    };
    signal.addEventListener("abort", abort, { once: true });
    worker.onmessage = (event) => {
      if (event.data.results) {
        cleanup();
        resolve(event.data.results);
      } else if (event.data.error) {
        cleanup();
        reject(new Error(event.data.error));
      } else if (typeof event.data.progress === "number")
        progress(event.data.progress);
    };
    worker.onerror = () => {
      cleanup();
      reject(new Error("File verification worker failed"));
    };
    worker.onmessageerror = () => {
      cleanup();
      reject(new Error("File verification response failed"));
    };
    try {
      worker.postMessage(files);
    } catch (error) {
      cleanup();
      reject(error);
    }
  });
}
