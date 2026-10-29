import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { loadSettingsPage } from "./settingsQueries";
import { InstanceSettings } from "./InstanceSettings";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
const settings = {
  mode: "library",
  revision: "test",
  fields: [],
  effective: {},
  overrides: [],
  restart_required: [],
};
const location = {
  automatic: true,
  endpoint: "https://photon.komoot.io/reverse",
  request_interval_seconds: 2,
  pause_on_error: true,
  revision: "test",
};
const ai = {
  model: "gpt-6-luna",
  auto_ocr: false,
  auto_desc: false,
  auto_facts: false,
  auto_orientation: false,
  proxy_url: "",
  revision: "test",
};
const queue = {
  paused: false,
  scanning: false,
  queued: 0,
  failed: 0,
  running: "",
};
it("reveals local settings together, reuses requests and does not wait for remote models", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  let releaseLocation!: (response: Response) => void,
    releaseModels!: (response: Response) => void,
    releaseStorage!: (response: Response) => void;
  const reads: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      const path = url.replace("/api/v1", "");
      reads.push(path);
      if (path === "/geo/settings")
        return new Promise<Response>((r) => {
          releaseLocation = r;
        });
      if (path === "/ai/models")
        return new Promise<Response>((r) => {
          releaseModels = r;
        });
      if (path === "/settings") return Response.json(settings);
      if (path === "/settings/credentials") return Response.json([]);
      if (path === "/settings/storage")
        return new Promise<Response>((r) => {
          releaseStorage = r;
        });
      if (path === "/ai")
        return Response.json({
          supported: true,
          connected: true,
          email: "test@example.test",
          pending: false,
          error: "",
        });
      if (path === "/ai/settings") return Response.json(ai);
      return Response.json(queue);
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <InstanceSettings />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(reads).toContain("/ai/settings"));
  expect(reads).toContain("/geo/settings");
  expect(
    screen.queryByRole("heading", { name: "Capture location" }),
  ).not.toBeInTheDocument();
  await waitFor(() =>
    expect(client.getQueryData(["sync-credentials"])).toEqual([]),
  );
  // The initial catalog check can invalidate a section before its owner mounts.
  await client.invalidateQueries({
    queryKey: ["sync-credentials"],
    refetchType: "none",
  });
  releaseLocation(Response.json(location));
  expect(
    await screen.findByRole("heading", { name: "Capture location" }),
  ).toBeVisible();
  expect(
    screen.getByLabelText("Photon endpoint", { selector: "input" }),
  ).toHaveValue(location.endpoint);
  expect(
    screen.getByLabelText("AI proxy address", { selector: "input" }),
  ).toHaveValue("");
  await screen.findByText("Loading models…");
  for (const path of [
    "/settings/storage",
    "/settings/credentials",
    "/ai",
    "/ai/settings",
    "/geo/settings",
    "/geo",
    "/ai/ocr",
  ])
    expect(reads.filter((p) => p === path)).toHaveLength(1);
  expect(
    view.container.querySelector(".storage-usage-values"),
  ).toHaveTextContent("—");
  releaseStorage(
    Response.json({
      originals_bytes: 10,
      metadata_bytes: 20,
      previews_bytes: 30,
    }),
  );
  releaseModels(Response.json([]));
  view.unmount();
  client.clear();
});
it("isolates optional section failures and respects unsupported AI endpoints", async () => {
  const reads: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      const path = url.replace("/api/v1", "");
      reads.push(path);
      if (path === "/settings") return Response.json(settings);
      if (path === "/settings/credentials")
        return Response.json(
          { code: "internal", message: "Credentials unavailable" },
          { status: 500 },
        );
      if (path === "/ai")
        return Response.json({ supported: false, connected: false });
      if (path === "/geo/settings") return Response.json(location);
      return Response.json(queue);
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  expect(await loadSettingsPage(client)).toEqual(settings);
  expect(client.getQueryState(["sync-credentials"])?.status).toBe("error");
  expect(client.getQueryData(["location-settings"])).toEqual(location);
  expect(reads).not.toContain("/ai/settings");
  expect(reads).not.toContain("/ai/ocr");
  client.clear();
});
