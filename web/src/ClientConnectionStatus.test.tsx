import { afterEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { ClientConnectionStatus } from "./ClientConnectionStatus";

afterEach(() => vi.unstubAllGlobals());
it.each([
  ["connected", "Remote connected"],
  ["unreachable", "Remote unavailable"],
  ["unauthorized", "Invalid credential"],
  ["incompatible", "Remote mismatch"],
  ["unconfigured", "Remote not configured"],
])("shows the probed %s connection state", async (state, label) => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        Response.json({
          state,
          pending_download: state === "connected" ? 3 : undefined,
          pending_upload: 2,
        }),
      ),
    ),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider>
          <ClientConnectionStatus openSync={() => {}} />
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  expect(await screen.findByText(label)).toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent(
    `${state === "connected" ? 3 : "—"} operations to download`,
  );
  expect(screen.getByRole("status")).toHaveTextContent(
    "2 operations to upload",
  );
  view.unmount();
  client.clear();
});

it.each([
  [{ syncing: true, pending_download: 3 }, "Synchronizing…"],
  [
    { syncing: true, progress: { direction: "download", count: 6 } },
    "Downloading 6 operations…",
  ],
  [
    { syncing: true, progress: { direction: "upload", count: 2 } },
    "Uploading 2 operations…",
  ],
  [
    { manual_required: true, pending_download: 3, pending_upload: 2 },
    "Manual merge required",
  ],
  [{}, "Up to date"],
])(
  "shows synchronization progress and hides zero counts",
  async (overrides, label) => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        Response.json({
          state: "connected",
          catalog_complete: true,
          catalog_op_id: 10,
          pending_download: 0,
          pending_upload: 0,
          ...overrides,
        }),
      ),
    );
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const view = render(
      <QueryClientProvider client={client}>
        <I18nProvider>
          <ThemeProvider>
            <ClientConnectionStatus openSync={() => {}} />
          </ThemeProvider>
        </I18nProvider>
      </QueryClientProvider>,
    );
    expect(await screen.findByText(label)).toBeInTheDocument();
    expect(
      screen.queryByText("0 operations to upload"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("0 operations to download"),
    ).not.toBeInTheDocument();
    view.unmount();
    client.clear();
  },
);

it.each([
  [6, 0, false, false, "connected", true],
  [0, 2, false, false, "connected", true],
  [6, 2, false, false, "connected", false],
  [0, 0, false, false, "connected", false],
  [6, 0, true, false, "connected", false],
  [6, 0, false, true, "connected", false],
  [6, 0, false, false, "unreachable", false],
])(
  "wakes only eligible one-way synchronization (%s/%s)",
  async (download, upload, syncing, manual, state, wake) => {
    const fetcher = vi.fn(async (url: string) =>
      Response.json(
        url.endsWith("/sync/automatic")
          ? {}
          : {
              state,
              pending_download: download,
              pending_upload: upload,
              syncing,
              manual_required: manual,
            },
      ),
    );
    vi.stubGlobal("fetch", fetcher);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const view = render(
      <QueryClientProvider client={client}>
        <I18nProvider>
          <ThemeProvider>
            <ClientConnectionStatus openSync={() => {}} />
          </ThemeProvider>
        </I18nProvider>
      </QueryClientProvider>,
    );
    await screen.findByText(
      syncing
        ? "Synchronizing…"
        : state === "unreachable"
          ? "Remote unavailable"
          : manual
            ? "Manual merge required"
            : "Remote connected",
    );
    expect(
      fetcher.mock.calls.some(([url]) => url.endsWith("/sync/automatic")),
    ).toBe(wake);
    if (wake)
      expect(fetcher).toHaveBeenCalledWith(
        "/api/v1/client/sync/automatic",
        expect.objectContaining({ method: "POST" }),
      );
    view.unmount();
    client.clear();
  },
);
