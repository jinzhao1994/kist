import { aiServicesFixture } from "./aiServices.fixture";
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
const ai = aiServicesFixture();
const queue = {
  paused: false,
  scanning: false,
  queued: 0,
  failed: 0,
  running: "",
};
// Multi-section user flows retain a bounded budget on slower CI workers.
it("reveals local settings together, reuses requests and does not wait for remote models", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  let releaseLocation!: (response: Response) => void,
    releaseModels!: (response: Response) => void,
    releaseStorage!: (response: Response) => void;
  let locationRequests = 0;
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
      if (path === "/geo/settings") {
        if (locationRequests++ > 0) return Response.json(location);
        return new Promise<Response>((r) => {
          releaseLocation = r;
        });
      }
      if (path === "/ai/accounts/1/models")
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
      if (path === "/ai/services") return Response.json(ai);
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
  await waitFor(() => expect(reads).toContain("/ai/services"));
  expect(reads).toContain("/geo/settings");
  expect(
    screen.queryByRole("heading", { name: "GPS", level: 2 }),
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
    await screen.findByRole("heading", { name: "GPS", level: 2 }),
  ).toBeVisible();
  expect(
    screen.getByLabelText("Photon endpoint", { selector: "input" }),
  ).toHaveValue(location.endpoint);
  expect(reads).not.toContain("/ai/accounts/1/models");
  for (const path of [
    "/settings/storage",
    "/settings/credentials",
    "/ai",
    "/ai/services",
    "/geo/settings",
    "/geo",
    "/metadata",
    "/ai/ocr",
  ])
    expect(reads.filter((p) => p === path)).toHaveLength(1);
  await userEvent.click(
    within(
      screen.getByRole("region", { name: "Text recognition (OCR)" }),
    ).getByRole("combobox", {
      name: "Model",
    }),
  );
  await screen.findByText("Loading models…");
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
  const aiSubsections = [
    "settings-ai-proxies",
    "settings-ai-accounts",
    "settings-ai-orientation",
    "settings-ai-ocr",
    "settings-ai-transcription",
    "settings-ai-desc",
    "settings-ai-facts",
  ];
  expect(
    [...view.container.querySelectorAll(".section-index-subsection")].map(
      (link) => link.getAttribute("href"),
    ),
  ).toEqual(aiSubsections.map((id) => `#${id}`));
  for (const id of aiSubsections)
    expect(document.getElementById(id)).toHaveAccessibleName();
  expect(
    screen.queryByRole("heading", { name: "AI features" }),
  ).not.toBeInTheDocument();
  expect(sectionIDs.slice(-4)).toEqual([
    "settings-location",
    "settings-maps",
    "settings-ai",
    "settings-background-tasks",
  ]);
  expect(
    indexLinks
      .filter((href) => !aiSubsections.some((id) => href === `#${id}`))
      .slice(-4),
  ).toEqual(sectionIDs.slice(-4).map((id) => `#${id}`));
  const background = screen.getByRole("region", { name: "Background tasks" });
  expect(within(background).getAllByRole("rowheader")).toHaveLength(7);
  for (const id of ["settings-ai", "settings-location"])
    expect(view.container.querySelector(`#${id} table.task-table`)).toBeNull();
  const ocr = within(background)
    .getByRole("rowheader", { name: /^Text recognition \(OCR\)/ })
    .closest("tr")!;
  const geo = within(background)
    .getByRole("rowheader", { name: /^GPS/ })
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
  await user.click(screen.getByRole("button", { name: "Add proxy" }));
  await user.click(screen.getByRole("textbox", { name: "Name" }));
  await user.paste("Test proxy");
  expect(ocrPause).toBeDisabled();
  expect(geoPause).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
  await waitFor(() => expect(ocrPause).toBeEnabled());
  await user.type(
    screen.getByRole("textbox", { name: "Request interval (seconds)" }),
    "0",
  );
  expect(geoPause).toBeDisabled();
  expect(ocrPause).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
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
}, 15_000);
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
  expect(reads).not.toContain("/ai/services");
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
      if (path === "/geo")
        return Response.json({
          ...queue,
          paused: true,
          scanning: true,
          queued: 1,
          error: "context canceled",
        });
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
  const row = (await screen.findByRole("rowheader", { name: /^GPS/ })).closest(
    "tr",
  )!;
  await waitFor(() =>
    expect(within(row).getByRole("status")).toHaveTextContent(/^Paused$/),
  );
  expect(within(row).queryByText("Scan pending")).not.toBeInTheDocument();
  expect(within(row).queryByRole("alert")).not.toBeInTheDocument();
  expect(within(row).getByRole("button", { name: "Retry" })).toBeDisabled();
  expect(within(row).getByRole("button", { name: "Resume" })).toBeEnabled();
  const user = userEvent.setup();
  await user.click(within(row).getByRole("button", { name: "Resume" }));
  const fill = within(row).getByRole("button", { name: "Fill missing" });
  await waitFor(() => expect(fill).toBeEnabled());
  await user.click(fill);
  await waitFor(() => expect(fill).toBeEnabled());
  expect(within(row).queryByText(/Backfill requested/)).not.toBeInTheDocument();
  expect(within(row).getByRole("status")).toHaveTextContent(/^Idle$/);
  const retry = within(row).getByRole("button", { name: "Retry" });
  await waitFor(() => expect(retry).toBeEnabled());
  await user.click(retry);
  const pause = within(row).getByRole("button", { name: "Pause" });
  await waitFor(() => expect(pause).toBeEnabled());
  await user.click(pause);
  expect(writes).toEqual([
    { path: "/geo/pause", body: { paused: false } },
    { path: "/geo/backfill", body: { retry: false } },
    { path: "/geo/backfill", body: { retry: true } },
    { path: "/geo/pause", body: { paused: true } },
  ]);
  expect(screen.getAllByRole("rowheader")).toHaveLength(2);
  view.unmount();
  client.clear();
});
