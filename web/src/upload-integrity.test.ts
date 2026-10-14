import { afterEach, expect, it, vi } from "vitest";
import { calculateIntegrity } from "./upload-integrity";

afterEach(() => vi.unstubAllGlobals());
function mockWorker() {
  class Worker {
    static instance: Worker;
    terminate = vi.fn();
    postMessage = vi.fn();
    onmessage?: (event: { data: unknown }) => void;
    onerror?: () => void;
    constructor() {
      Worker.instance = this;
    }
  }
  vi.stubGlobal("Worker", Worker);
  return Worker;
}
it("forwards progress and terminates after hashing", async () => {
  const Worker = mockWorker();
  const progress = vi.fn();
  const promise = calculateIntegrity(
    [],
    progress,
    new AbortController().signal,
  );
  Worker.instance.onmessage?.({ data: { progress: 0.5 } });
  expect(progress).toHaveBeenCalledWith(0.5);
  Worker.instance.onmessage?.({ data: { results: [] } });
  await expect(promise).resolves.toEqual([]);
  expect(Worker.instance.terminate).toHaveBeenCalledOnce();
});
it("terminates computation on cancellation", async () => {
  const Worker = mockWorker();
  const controller = new AbortController();
  const promise = calculateIntegrity([], () => {}, controller.signal);
  controller.abort();
  await expect(promise).rejects.toMatchObject({ name: "AbortError" });
  expect(Worker.instance.terminate).toHaveBeenCalledOnce();
});
it("does not start a worker when already cancelled", async () => {
  const Worker = mockWorker();
  const controller = new AbortController();
  controller.abort();
  await expect(
    calculateIntegrity([], () => {}, controller.signal),
  ).rejects.toMatchObject({ name: "AbortError" });
  expect(Worker.instance).toBeUndefined();
});
it("rejects worker failures instead of uploading unverified files", async () => {
  const Worker = mockWorker();
  const promise = calculateIntegrity(
    [],
    () => {},
    new AbortController().signal,
  );
  Worker.instance.onerror?.();
  await expect(promise).rejects.toThrow("worker failed");
  expect(Worker.instance.terminate).toHaveBeenCalledOnce();
});
