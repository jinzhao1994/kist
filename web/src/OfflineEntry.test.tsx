import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { OfflineEntry } from "./OfflineEntry";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
let pins: Record<string, unknown>;
let available: boolean;
let fail: boolean;
let writes: { path: string; method: string; body: any }[];
beforeEach(() => {
  localStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  pins = {};
  available = false;
  fail = false;
  writes = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, opt?: RequestInit) => {
      const owner = decodeURIComponent(path.split("/").at(-1)!);
      if (opt?.method) {
        const body = JSON.parse(opt.body as string);
        writes.push({ path, method: opt.method, body });
        if (opt.method === "PUT") pins[owner] = body;
        if (opt.method === "DELETE") delete pins[owner];
        if (path.endsWith("/prepare")) {
          if (fail)
            return new Response(
              JSON.stringify({
                code: "offline",
                message: "NAS unavailable",
                class: "internal",
              }),
              { status: 503 },
            );
          available = true;
        }
      }
      const body = path.endsWith("/offline")
        ? pins
        : {
            ready: available,
            available: available ? 2 : 0,
            originals: [{ key: "photo" }, { key: "video" }],
            missing: available ? [] : ["photo", "video"],
            catalog: { complete: true },
          };
      return new Response(JSON.stringify(body), { status: 200 });
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup(fileset = true) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <OfflineEntry
            target={{ id: "entry-1", name: "Travel", fileset }}
            close={() => {}}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("retains the fileset identity before downloading its members", async () => {
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Retain and download" }),
  );
  await waitFor(() => expect(writes).toHaveLength(2));
  expect(writes[0]).toEqual({
    path: "/api/v1/client/offline/fileset%3Aentry-1",
    method: "PUT",
    body: { files: [], filesets: ["entry-1"] },
  });
  expect(writes[1].path).toBe(
    "/api/v1/client/offline/fileset%3Aentry-1/prepare",
  );
  expect(
    await screen.findByText("All required originals are available offline."),
  ).toBeVisible();
});
it("keeps retention after a failed download without claiming readiness", async () => {
  fail = true;
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Retain and download" }),
  );
  expect(await screen.findByText(/NAS unavailable/)).toBeVisible();
  expect(screen.getByText("Offline retention is enabled.")).toBeVisible();
  expect(
    screen.queryByText("All required originals are available offline."),
  ).not.toBeInTheDocument();
  await userEvent.click(
    screen.getByRole("button", { name: "Stop retaining offline" }),
  );
  await waitFor(() => expect(writes.at(-1)?.method).toBe("DELETE"));
  expect(writes.at(-1)?.path).toBe("/api/v1/client/offline/fileset%3Aentry-1");
});
it("does not contact the NAS for an already available local file", async () => {
  available = true;
  setup(false);
  await userEvent.click(
    await screen.findByRole("button", { name: "Retain and download" }),
  );
  expect(
    await screen.findByText("All required originals are available offline."),
  ).toBeVisible();
  expect(writes).toEqual([
    {
      path: "/api/v1/client/offline/file%3Aentry-1",
      method: "PUT",
      body: { files: ["entry-1"], filesets: [] },
    },
  ]);
});
