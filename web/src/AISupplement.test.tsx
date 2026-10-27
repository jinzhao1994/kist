import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AISupplement } from "./AISupplement";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  Object.defineProperty(document, "fonts", {
    configurable: true,
    value: new EventTarget(),
  });
});
afterEach(() => vi.unstubAllGlobals());
function mount(manual = false, canRun = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">
          <dl className="metadata-list">
            <AISupplement
              fileID="file"
              blobKey="sha256:test"
              kind="ocr"
              manual={manual}
              canRun={canRun}
            />
          </dl>
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}
it("saves a bare name without calling inference, including offline", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  mount(false, false);
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Edit Supplementary information" }),
  );
  await user.type(screen.getByRole("textbox"), "Example Person");
  expect(fetch).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: "Save and re-extract" }),
  ).toBeNull();
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  const [url, init] = (
    fetch.mock.calls as unknown as [string, RequestInit][]
  )[0];
  expect(url).toBe("/api/v1/files/file/metadata/hints/ocr");
  expect(JSON.parse(String(init.body))).toEqual({
    blob_key: "sha256:test",
    before: null,
    value: "Example Person",
  });
});
it("retries saved hints and the same queued request after a lost response", async () => {
  const writes: { url: string; body: string; headers: unknown }[] = [];
  let lost = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      if (!init?.method) return Response.json({ state: "ready" });
      writes.push({ url, body: String(init.body), headers: init.headers });
      if (init.method === "POST" && lost) {
        lost = false;
        throw new TypeError("lost");
      }
      return Response.json(
        init.method === "POST" ? { state: "queued" } : { op_id: 1 },
      );
    }),
  );
  mount();
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Edit Supplementary information" }),
  );
  await user.type(screen.getByRole("textbox"), "Example Person");
  const run = screen.getByRole("button", { name: "Save and re-extract" });
  await waitFor(() => expect(run).toBeEnabled());
  await user.click(run);
  await screen.findByRole("alert");
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(writes).toHaveLength(4));
  expect(writes[0]).toEqual(writes[2]);
  expect(writes[1]).toEqual(writes[3]);
  expect(writes[1].url).toBe("/api/v1/files/file/ocr");
  expect(JSON.parse(writes[1].body)).toMatchObject({
    force: true,
    retry: true,
  });
});
it("never offers to replace human-edited text", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => Response.json({ state: "ready" })),
  );
  mount(true);
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Edit Supplementary information" }),
  );
  await user.type(screen.getByRole("textbox"), "Example Person");
  expect(
    screen.getByRole("button", { name: "Save and re-extract" }),
  ).toBeDisabled();
  expect(screen.getByText(/AI will not replace your text/)).toBeVisible();
  expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
});
