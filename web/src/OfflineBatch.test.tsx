import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { OfflineBatch } from "./OfflineBatch";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
let calls: { path: string; method?: string; body?: unknown }[];
let mode: "success" | "fail" | "cancel" | "changed";
let available: Set<string>;
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
  calls = [];
  available = new Set();
  mode = "success";
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      calls.push({
        path,
        method: options?.method,
        body: options?.body ? JSON.parse(options.body as string) : undefined,
      });
      const owner = decodeURIComponent(
        path.split("/").at(options?.method === "POST" ? -2 : -1)!,
      );
      if (options?.method === "POST") {
        if (owner === "fileset:pages") {
          if (mode === "cancel")
            await new Promise((_, reject) =>
              options.signal?.addEventListener(
                "abort",
                () => reject(new DOMException("Aborted", "AbortError")),
                { once: true },
              ),
            );
          if (mode === "fail")
            return Response.json(
              {
                code: "internal",
                message: "NAS unreachable",
                class: "internal",
              },
              { status: 503 },
            );
          if (mode === "changed") return Response.json({ ready: false });
        }
        available.add(owner);
      }
      return Response.json({ ready: available.has(owner) });
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <OfflineBatch
            targets={[
              { id: "photo", name: "Photo.jpg", fileset: false },
              { id: "pages", name: "Passport", fileset: true },
            ]}
            close={() => {}}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("retains every logical selection before transferring content", async () => {
  setup();
  await userEvent.click(
    screen.getByRole("button", { name: "Retain and download" }),
  );
  expect(
    await screen.findByText("2 of 2 retained · 2 of 2 prepared"),
  ).toBeVisible();
  expect(calls.slice(0, 2)).toEqual([
    {
      path: "/api/v1/client/offline/file%3Aphoto",
      method: "PUT",
      body: { files: ["photo"], filesets: [] },
    },
    {
      path: "/api/v1/client/offline/fileset%3Apages",
      method: "PUT",
      body: { files: [], filesets: ["pages"] },
    },
  ]);
  expect(calls.filter((c) => c.method === "POST")).toHaveLength(2);
});
it("retries after interruption without redownloading completed selections", async () => {
  mode = "fail";
  setup();
  await userEvent.click(
    screen.getByRole("button", { name: "Retain and download" }),
  );
  expect(await screen.findByText(/NAS unreachable/)).toBeVisible();
  expect(screen.getByText("2 of 2 retained · 1 of 2 prepared")).toBeVisible();
  mode = "success";
  await userEvent.click(
    screen.getByRole("button", { name: "Check and download" }),
  );
  expect(
    await screen.findByText("2 of 2 retained · 2 of 2 prepared"),
  ).toBeVisible();
  expect(
    calls.filter((c) => c.method === "POST" && c.path.includes("file%3Aphoto")),
  ).toHaveLength(1);
  expect(calls.filter((c) => c.method === "DELETE")).toHaveLength(0);
});
it("cancels the active request while preserving all retained selections", async () => {
  mode = "cancel";
  setup();
  await userEvent.click(
    screen.getByRole("button", { name: "Retain and download" }),
  );
  await waitFor(() =>
    expect(calls.at(-1)?.path).toBe(
      "/api/v1/client/offline/fileset%3Apages/prepare",
    ),
  );
  expect(screen.getByText("Close").closest("button")).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(await screen.findByText(/Stopped. Saved selections/)).toBeVisible();
  expect(screen.getByText("2 of 2 retained · 1 of 2 prepared")).toBeVisible();
  expect(calls.filter((c) => c.method === "DELETE")).toHaveLength(0);
});
it("does not count a changed selection as successfully prepared", async () => {
  mode = "changed";
  setup();
  await userEvent.click(
    screen.getByRole("button", { name: "Retain and download" }),
  );
  expect(
    await screen.findByText(/This selection changed during preparation/),
  ).toBeVisible();
  expect(screen.getByText("2 of 2 retained · 1 of 2 prepared")).toBeVisible();
});
