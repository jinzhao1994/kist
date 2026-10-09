import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { FileBrowser } from "./FileBrowser";
import { emptyFilters } from "./Filters";
import { FileGrid } from "./FileGrid";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile } from "./api";
const files: ArchiveFile[] = Array.from({ length: 1000 }, (_, index) => ({
  id: String(index),
  name: `photo-${index}.jpg`,
  folder_id: "folder",
  blob_key: "blob",
  captured_at: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  media_type: "image/jpeg",
  kind: "image",
}));
let mediaClient: QueryClient;
const ready = () =>
  Response.json({ state: "ready", generation: "v1", result: {} });
function mediaWrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={mediaClient}>{children}</QueryClientProvider>
  );
}
beforeEach(() => {
  mediaClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ready()),
  );
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(900);
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(600);
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(600);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(private callback: ResizeObserverCallback) {}
      observe(target: Element) {
        this.callback(
          [
            {
              target,
              contentRect: { width: 900, height: 600 },
            } as ResizeObserverEntry,
          ],
          this as unknown as ResizeObserver,
        );
      }
      unobserve() {}
      disconnect() {}
    },
  );
  HTMLElement.prototype.scrollTo = function (
    options?: ScrollToOptions | number,
  ) {
    this.scrollTop = typeof options === "number" ? options : options?.top || 0;
    this.dispatchEvent(new Event("scroll"));
  };
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("virtualizes a thousand files and unloads every grid image while details are open", async () => {
  const open = vi.fn();
  const view = render(
    <I18nProvider>
      <FileGrid files={files} open={open} />
    </I18nProvider>,
    { wrapper: mediaWrapper },
  );
  await waitFor(() => expect(screen.getAllByRole("img")).toHaveLength(8));
  expect(screen.getAllByRole("button").length).toBeLessThan(25);
  const user = userEvent.setup();
  await user.click(screen.getAllByRole("button")[0]);
  expect(open).toHaveBeenCalledWith(files[0]);
  view.rerender(
    <I18nProvider>
      <FileGrid files={files} open={open} paused />
    </I18nProvider>,
  );
  expect(screen.queryAllByRole("img")).toHaveLength(0);
  view.rerender(
    <I18nProvider>
      <FileGrid files={files} open={open} />
    </I18nProvider>,
  );
  await waitFor(() => expect(screen.getAllByRole("img")).toHaveLength(8));
});
it("moves keyboard focus by row and respects RTL horizontal navigation", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "rtl",
    }),
  );
  render(
    <I18nProvider>
      <FileGrid files={files} open={vi.fn()} />
    </I18nProvider>,
    { wrapper: mediaWrapper },
  );
  await screen.findAllByRole("button");
  const buttons = screen.getAllByRole("button");
  buttons[0].focus();
  const user = userEvent.setup();
  await user.keyboard("{ArrowLeft}");
  await waitFor(() => expect(buttons[1]).toHaveFocus());
  await user.keyboard("{ArrowDown}");
  await waitFor(() => expect(buttons[5]).toHaveFocus());
});

it("uses keyset pagination and resets it when filters change", async () => {
  const requests: URL[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const url = new URL(path, "http://localhost");
      if (url.pathname.includes("/derived/")) return ready();
      if (url.pathname.endsWith("/info"))
        return new Response(
          JSON.stringify({
            version: "test",
            max_upload_files: 10,
            max_upload_bytes: 10000,
          }),
        );
      requests.push(url);
      return new Response(
        JSON.stringify(
          url.searchParams.get("name")
            ? { files: [] }
            : url.searchParams.get("cursor")
              ? { files: [files[1]] }
              : { files: [files[0]], cursor: "next-page" },
        ),
      );
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const page = (name = "") => (
    <QueryClientProvider client={client}>
      <I18nProvider>
        <FileBrowser
          filters={{ ...emptyFilters, name }}
          tags={[]}
          open={vi.fn()}
        />
      </I18nProvider>
    </QueryClientProvider>
  );
  const view = render(page());
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Load more" }));
  await waitFor(() => expect(screen.getAllByRole("img")).toHaveLength(2));
  expect(requests[1].searchParams.get("cursor")).toBe("next-page");
  expect(
    screen.queryByRole("button", { name: "Load more" }),
  ).not.toBeInTheDocument();
  view.rerender(page("missing"));
  await waitFor(() =>
    expect(requests.at(-1)?.searchParams.get("name")).toBe("missing"),
  );
  expect(requests.at(-1)?.searchParams.has("cursor")).toBe(false);
  await waitFor(() => expect(screen.queryAllByRole("img")).toHaveLength(0));
});
