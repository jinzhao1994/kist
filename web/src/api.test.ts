import { afterEach, expect, it, vi } from "vitest";
import { APIError, createWrite } from "./api";
afterEach(() => vi.unstubAllGlobals());
it("retains the key and original body across a lost response", async () => {
  const fetch = vi
    .fn()
    .mockRejectedValueOnce(new TypeError("Disconnected"))
    .mockResolvedValueOnce(new Response('{"op_id":7}'));
  vi.stubGlobal("fetch", fetch);
  const body = { name: "First" };
  const write = createWrite("POST", "/boxes", body);
  body.name = "Changed";
  await expect(write.run()).rejects.toThrow("Disconnected");
  await expect(write.run()).resolves.toEqual({ op_id: 7 });
  expect(fetch.mock.calls[0][1].headers["Idempotency-Key"]).toBe(
    fetch.mock.calls[1][1].headers["Idempotency-Key"],
  );
  expect(fetch.mock.calls[1][1].body).toBe('{"name":"First"}');
  expect(createWrite("POST", "/boxes", body).key).not.toBe(write.key);
});
it("distinguishes a committed write from an unknown result", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValueOnce(
        new Response('{"code":"write_committed","op_id":9}', { status: 409 }),
      )
      .mockResolvedValueOnce(
        new Response('{"code":"write_outcome_unknown","message":"unknown"}', {
          status: 409,
        }),
      ),
  );
  await expect(createWrite("POST", "/boxes", {}).run()).resolves.toEqual({
    op_id: 9,
  });
  await expect(createWrite("POST", "/boxes", {}).run()).rejects.toBeInstanceOf(
    APIError,
  );
});
