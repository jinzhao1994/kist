import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const path = url.replace("/api/v1", "");
      if (init?.method === "POST") {
        writes.push(path);
        return Response.json({ ...queue, paused: true });
      }
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
    screen.queryByRole("heading", { name: "Location", level: 2 }),
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
    await screen.findByRole("heading", { name: "Location", level: 2 }),
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
  releaseModels(Response.json([]));
  const indexLinks = [
    ...view.container.querySelectorAll(".section-index a"),
  ].map((a) => a.getAttribute("href"));
  // Section order and the directory must agree, with background tasks last.
  const sectionIDs = [
    ...view.container.querySelectorAll(".flat-page-section"),
  ].map((el) => el.id);
  expect(sectionIDs.slice(-3)).toEqual([
    "settings-location",
    "settings-maps",
    "settings-background-tasks",
  ]);
  expect(indexLinks.slice(-3)).toEqual(
    sectionIDs.slice(-3).map((id) => `#${id}`),
  );
  const background = screen.getByRole("region", { name: "Background tasks" });
  expect(within(background).getAllByRole("rowheader")).toHaveLength(5);
  for (const id of ["settings-ai", "settings-location"])
    expect(view.container.querySelector(`#${id} table.task-table`)).toBeNull();
  const ocr = within(background)
    .getByRole("rowheader", { name: /^OCR/ })
    .closest("tr")!;
  const geo = within(background)
    .getByRole("rowheader", { name: /^Location/ })
    .closest("tr")!;
  const ocrPause = within(ocr).getByRole("button", {
    name: "Pause",
  });
  const geoPause = within(geo).getByRole("button", {
    name: "Pause",
  });
  expect(ocrPause).toBeEnabled();
  expect(geoPause).toBeEnabled();
  const user = userEvent.setup();
  await user.type(
    screen.getByLabelText("AI proxy address", { selector: "input" }),
    "http://localhost:7890",
  );
  expect(ocrPause).toBeDisabled();
  expect(geoPause).toBeEnabled();
  await user.click(screen.getByRole("button", { name: "Reload AI settings" }));
  await waitFor(() => expect(ocrPause).toBeEnabled());
  await user.type(
    screen.getByRole("textbox", { name: "Request interval (seconds)" }),
    "0",
  );
  expect(geoPause).toBeDisabled();
  expect(ocrPause).toBeEnabled();
  await user.click(
    screen.getByRole("button", { name: "Reload location settings" }),
  );
  await waitFor(() => expect(geoPause).toBeEnabled());
  await user.click(ocrPause);
  expect(
    await within(ocr).findByRole("button", { name: "Resume" }),
  ).toBeEnabled();
  expect(geoPause).toHaveTextContent("Pause");
  expect(writes).toEqual(["/ai/ocr/pause"]);
  releaseStorage(
    Response.json({
      originals_bytes: 10,
      metadata_bytes: 20,
      previews_bytes: 30,
    }),
  );
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

it("keeps task commands scoped to their row and distinguishes backfill from retry", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  const writes: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const path = url.replace("/api/v1", "");
      if (init?.method === "POST") {
        writes.push({ path, body: JSON.parse(String(init.body)) });
        return Response.json({ ...queue, failed: 2 });
      }
      if (path === "/ai")
        return Response.json({ supported: false, connected: false });
      if (path === "/geo/settings") return Response.json(location);
      if (path === "/geo") return Response.json({ ...queue, failed: 2 });
      throw new Error(`Unexpected request: ${path}`);
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const { BackgroundTasks } = await import("./BackgroundTasks");
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <BackgroundTasks />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  const row = (
    await screen.findByRole("rowheader", { name: /^Location/ })
  ).closest("tr")!;
  const user = userEvent.setup();
  const fill = within(row).getByRole("button", { name: "Fill missing" });
  await waitFor(() => expect(fill).toBeEnabled());
  await user.click(fill);
  const retry = within(row).getByRole("button", { name: "Retry" });
  await waitFor(() => expect(retry).toBeEnabled());
  await user.click(retry);
  const pause = within(row).getByRole("button", { name: "Pause" });
  await waitFor(() => expect(pause).toBeEnabled());
  await user.click(pause);
  expect(writes).toEqual([
    { path: "/geo/backfill", body: { retry: false } },
    { path: "/geo/backfill", body: { retry: true } },
    { path: "/geo/pause", body: { paused: true } },
  ]);
  expect(screen.getAllByRole("rowheader")).toHaveLength(1);
  view.unmount();
  client.clear();
});
