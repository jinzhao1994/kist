import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { OfflineRetention } from "./OfflineRetention";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
let selections: Record<string, { files: string[]; filesets: string[] }>;
let requests: { path: string; method?: string }[];
let fail: boolean;
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
  selections = {
    "Travel / documents": { files: ["one", "two"], filesets: ["pages"] },
  };
  requests = [];
  fail = false;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      requests.push({ path, method: options?.method });
      if (options?.method === "DELETE")
        delete selections[decodeURIComponent(path.split("/").at(-1)!)];
      if (fail && path.includes("/offline/"))
        return new Response(
          JSON.stringify({
            code: "internal",
            message: "Unavailable member",
            class: "internal",
          }),
          { status: 500 },
        );
      return new Response(
        JSON.stringify(
          path.endsWith("/offline")
            ? selections
            : path.includes("/files/")
              ? { file: { name: "Passport.png" } }
              : {
                  ready: false,
                  available: 1,
                  originals: [{ key: "one" }, { key: "two" }],
                  previews_available: 0,
                  previews: [],
                },
        ),
        { status: 200 },
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <OfflineRetention
            active
            busy={false}
            run={async (command) => {
              await command(new AbortController().signal);
              await client.invalidateQueries({ queryKey: ["client"] });
            }}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("lists named selections without hashing their content until requested", async () => {
  setup();
  expect(await screen.findByText("Travel / documents")).toBeVisible();
  expect(screen.getByText("2 files · 1 file sets")).toBeVisible();
  expect(requests).toEqual([
    { path: "/api/v1/client/offline", method: undefined },
  ]);
  await userEvent.click(
    screen.getByRole("button", { name: "Check availability" }),
  );
  expect(await screen.findByText(/1 of 2 originals available/)).toBeVisible();
  expect(requests.at(-1)?.path).toBe(
    "/api/v1/client/offline/Travel%20%2F%20documents",
  );
});
it("prepares an existing selection without overwriting its members", async () => {
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Check and download" }),
  );
  await waitFor(() =>
    expect(requests.filter((r) => r.method)).toEqual([
      {
        path: "/api/v1/client/offline/Travel%20%2F%20documents/prepare",
        method: "POST",
      },
    ]),
  );
  expect(await screen.findByText(/1 of 2 originals available/)).toBeVisible();
});
it("can remove retention even when one of its targets no longer resolves", async () => {
  fail = true;
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Check availability" }),
  );
  expect(await screen.findByText(/Unavailable member/)).toBeVisible();
  fail = false;
  await userEvent.click(
    screen.getByRole("button", { name: "Stop retaining offline" }),
  );
  expect(await screen.findByText(/No offline selections yet/)).toBeVisible();
  expect(requests.filter((r) => r.method)).toEqual([
    {
      path: "/api/v1/client/offline/Travel%20%2F%20documents",
      method: "DELETE",
    },
  ]);
});
it("uses the current name for a file retained from its menu", async () => {
  selections = { "file:one": { files: ["one"], filesets: [] } };
  setup();
  expect(await screen.findByText("Passport.png")).toBeVisible();
  expect(requests.some((r) => r.path.includes("/offline/"))).toBe(false);
});

it("refreshes an in-flight availability check after preparation", async () => {
  const originalFetch = globalThis.fetch;
  let prepared = false;
  let started = false;
  let release!: () => void;
  const delayed = new Promise<void>((resolve) => {
    release = resolve;
  });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      if (path.endsWith("/prepare")) prepared = true;
      const response = await originalFetch(path, options);
      if (!options?.method && path.includes("/offline/")) {
        if (!prepared) {
          started = true;
          await delayed;
        } else {
          return Response.json({
            ready: true,
            available: 2,
            originals: [{ key: "one" }, { key: "two" }],
            previews: [],
            previews_available: 0,
          });
        }
      }
      return response;
    }),
  );
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Check availability" }),
  );
  await waitFor(() => expect(started).toBe(true));
  await userEvent.click(
    screen.getByRole("button", { name: "Check and download" }),
  );
  expect(
    await screen.findByText(
      "All required originals and previews are available offline.",
    ),
  ).toBeVisible();
  release();
  await waitFor(() =>
    expect(
      screen.queryByText(/1 of 2 originals available/),
    ).not.toBeInTheDocument(),
  );
});
