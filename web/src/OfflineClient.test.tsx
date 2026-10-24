import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { OfflineClient } from "./OfflineClient";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
import type { ClientStatus } from "./offline-client";
let state: ClientStatus;
let writes: { path: string; body: any }[];
let failSync: boolean;
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
  state = {
    settings: {
      version: 1,
      id: "client",
      library_id: "nas",
      timezone: "UTC",
      cache_max_age: "720h",
      cache_max_bytes: 10485760,
    },
    catalog: {
      complete: false,
      confirmed: { library_id: "nas", op_id: 0, sum: "" },
      downloaded: 0,
    },
    connection: { server: "https://nas.example", allow_http: false },
    pending: [],
  };
  writes = [];
  failSync = false;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      let body: unknown;
      if (options?.method) {
        writes.push({ path, body: JSON.parse(options.body as string) });
        if (path.endsWith("/sync")) {
          if (failSync)
            return new Response(
              JSON.stringify({
                code: "internal",
                message: "Connection interrupted",
                class: "internal",
              }),
              { status: 503 },
            );
          body = {
            result: { complete: true, accepted: 0, remaining: 0 },
            status: state,
          };
        } else if (path.endsWith("/resolve")) {
          state = { ...state, pending: [], blocked_id: undefined };
          body = state;
        } else body = state;
      } else if (path.endsWith("/offline")) body = {};
      else if (path.includes("/history")) body = { entries: [] };
      else if (path.endsWith("/connection-status"))
        body = {
          state: "connected",
          catalog_complete: state.catalog.complete,
          pending_download: 0,
          pending_upload: state.pending?.length ?? 0,
          manual_required: (state.pending?.length ?? 0) > 0,
        };
      else if (path.endsWith("/status")) body = state;
      else if (path.endsWith("/protected-storage"))
        body = { originals_bytes: 100, pending_journal_bytes: 20 };
      else
        body = {
          cached_bytes: 10,
          partial_bytes: 0,
          reserved_bytes: 0,
          pinned_bytes: 0,
          corrupt_bytes: 0,
          limit_bytes: 10485760,
          over_limit: false,
        };
      return new Response(JSON.stringify(body), { status: 200 });
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <OfflineClient openSettings={() => {}} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("shows incomplete catalog without automatically syncing", async () => {
  setup();
  expect(
    await screen.findByText(/catalog has not finished syncing/),
  ).toBeVisible();
  expect(writes).toEqual([]);
  await userEvent.click(screen.getByRole("button", { name: "Sync now" }));
  expect(await screen.findByText("Sync complete")).toBeVisible();
  expect(writes).toEqual([{ path: "/api/v1/client/sync", body: {} }]);
});
it("retains a pending change after a failed sync", async () => {
  state.pending = [
    {
      id: "pending-1",
      action: "create_box",
      sequence: 1,
      occurred_at: "2026-10-21T00:00:00Z",
      awaiting_receipt: true,
      awaiting_root: false,
    },
  ];
  failSync = true;
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Sync now" }),
  );
  expect(await screen.findByText(/Connection interrupted/)).toBeVisible();
  expect(screen.getByText("pending-1")).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Discard local change" }),
  ).not.toBeInTheDocument();
});
it("requires an explicit conflict decision before discarding", async () => {
  state.pending = [
    {
      id: "pending-1",
      action: "update_box",
      sequence: 1,
      occurred_at: "2026-10-21T00:00:00Z",
      awaiting_receipt: false,
      awaiting_root: false,
      conflict: "Name changed on NAS",
    },
  ];
  state.blocked_id = "pending-1";
  setup();
  const discard = await screen.findByRole("button", {
    name: "Discard local change",
  });
  expect(discard).toBeDisabled();
  await userEvent.click(
    screen.getByRole("checkbox", { name: /discard this local change/ }),
  );
  await userEvent.click(discard);
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].path).toBe("/api/v1/client/pending/pending-1/resolve");
  expect(writes[0].body.reason).toBeTruthy();
  expect(
    await screen.findByText("Decision saved. Sync again to continue."),
  ).toBeVisible();
});

it("shows a compact synchronized state without zero counts or redundant catalog text", async () => {
  state.catalog.complete = true;
  setup();
  expect(await screen.findByText("Up to date")).toBeVisible();
  expect(
    screen.queryByText("0 operations to download"),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByText("No local changes to submit."),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", {
      name: "Get remote updates and submit local changes.",
    }),
  ).toBeVisible();
});

it("keeps relay custody distinct from final confirmation and delegates conflict decisions", async () => {
  state.catalog.complete = true;
  state.pending = [
    {
      id: "held",
      sequence: 1,
      action: "create_box",
      occurred_at: "2026-10-24T00:00:00Z",
      awaiting_receipt: false,
      awaiting_root: true,
      conflict: "Remote conflict",
    },
  ];
  state.blocked_id = "held";
  setup();
  expect(
    await screen.findByText(
      "Held by the remote; awaiting root confirmation. Resolve changes at the remote.",
    ),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Discard local change" }),
  ).not.toBeInTheDocument();
  expect(screen.queryByText("1 operations to upload")).not.toBeInTheDocument();
});
