import { UploadStream } from "./upload-stream";
let stream: UploadStream | undefined;
self.onmessage = async (event: MessageEvent) => {
  const started = performance.now();
  try {
    if (event.data.file) {
      stream = new UploadStream(event.data.file);
      const prefix = await stream.prefix(event.data.offset);
      self.postMessage({ prefix, hashMs: performance.now() - started });
    } else {
      if (!stream) throw new Error("No file");
      const chunk = await stream.next(event.data.size);
      self.postMessage(
        { ...chunk, hashMs: performance.now() - started },
        { transfer: [chunk.data] },
      );
    }
  } catch {
    self.postMessage({ error: "Cannot read original file" });
  }
};
