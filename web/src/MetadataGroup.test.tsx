import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { MetadataGroup } from "./MetadataGroup";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
function mount(extractor?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">
          <MetadataGroup title="Test" fileID="file" extractor={extractor}>
            <p>Original result</p>
          </MetadataGroup>
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return client;
}
it("retains original text and the same AI request key after a lost response", async () => {
  const bodies: { force: boolean; retry: boolean; request_id: string }[] = [];
  let state = "ready";
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        bodies.push(JSON.parse(String(init.body)));
        if (bodies.length === 1) throw new TypeError("Connection lost");
        state = "queued";
        return Response.json({ state });
      }
      return Response.json(
        _url.includes("/files/") ? { state } : { paused: false },
      );
    }),
  );
  const client = mount("desc");
  const button = screen.getByRole("button", { name: "Re-extract Test" });
  await waitFor(() => expect(button).toBeEnabled());
  await userEvent.click(button);
  await screen.findByRole("alert");
  expect(screen.getByText("Original result")).toBeVisible();
  await userEvent.click(button);
  await waitFor(() => expect(button).toBeDisabled());
  expect(bodies).toHaveLength(2);
  expect(bodies[0]).toEqual(bodies[1]);
  expect(bodies[0].force).toBe(true);
  expect(bodies[0].retry).toBe(false);
  expect(bodies[0].request_id).toMatch(/^[\da-f-]{36}$/);
  state = "failed";
  await client.invalidateQueries();
  await waitFor(() => expect(button).toBeEnabled());
  expect(screen.getByText("Original result")).toBeVisible();
});
it("uses the regular idempotent extraction endpoint for EXIF", async () => {
  const writes: { url: string; body: string }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      writes.push({ url, body: String(init?.body) });
      return Response.json({ op_id: 1 });
    }),
  );
  mount("exif");
  await userEvent.click(
    screen.getByRole("button", { name: "Re-extract Test" }),
  );
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toEqual({
    url: "/api/v1/files/file/extractions/exif",
    body: '{"force":true}',
  });
});
it("has no action or background request for non-extractable groups", () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
  expect(fetch).not.toHaveBeenCalled();
});
