import { hashFile } from "./upload-hash";

self.onmessage = async (event: MessageEvent<File[]>) => {
  try {
    const results = [];
    let completed = 0;
    const total = event.data.reduce((sum, file) => sum + file.size, 0);
    for (const file of event.data) {
      results.push(
        await hashFile(file, (bytes) => {
          self.postMessage({
            progress: total ? (completed + bytes) / total : 1,
          });
        }),
      );
      completed += file.size;
    }
    self.postMessage({ results });
  } catch {
    self.postMessage({ error: "Cannot read original files for verification" });
  }
};
