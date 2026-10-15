export type StreamChunk = {
  offset: number;
  data: ArrayBuffer;
  sha256: string;
  whole?: string;
  hashMs: number;
};

export function uploadStreamWorker(
  signal: AbortSignal,
  measured: (ms: number) => void,
) {
  const worker = new Worker(
    new URL("./upload-stream.worker.ts", import.meta.url),
    { type: "module" },
  );
  let rejectPending: ((error: unknown) => void) | undefined;
  const close = () => {
    worker.terminate();
    signal.removeEventListener("abort", abort);
    rejectPending?.(signal.reason ?? new DOMException("Stopped", "AbortError"));
    rejectPending = undefined;
  };
  const abort = () => close();
  signal.addEventListener("abort", abort, { once: true });
  function request<T>(body: unknown): Promise<T> {
    return new Promise((resolve, reject) => {
      signal.throwIfAborted();
      rejectPending = reject;
      worker.onmessage = (event) => {
        rejectPending = undefined;
        if (event.data.error) reject(new Error(event.data.error));
        else {
          measured(event.data.hashMs);
          resolve(event.data);
        }
      };
      worker.onerror = worker.onmessageerror = () => {
        rejectPending = undefined;
        reject(new Error("Upload worker failed"));
      };
      worker.postMessage(body);
    });
  }
  return {
    open: (file: Blob, offset: number) =>
      request<{ prefix: string }>({ file, offset }),
    next: (size: number) => request<StreamChunk>({ size }),
    close,
  };
}
