import { BoxGrid } from "./BoxGrid";
import { FolderBreadcrumbs } from "./Folders";
import { ThemeProvider } from "./theme";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { FileBrowser } from "./FileBrowser";
import { emptyFilters } from "./Filters";
import { FileGrid } from "./FileGrid";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile, Box, Folder } from "./api";
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
let intersections: Set<IntersectionObserverCallback>;
function reachEnd() {
  act(() => {
    for (const callback of intersections)
      callback(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver,
      );
  });
}
const ready = () =>
  Response.json({ state: "ready", generation: "v1", result: {} });
function mediaWrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={mediaClient}>{children}</QueryClientProvider>
  );
}
beforeEach(() => {
  intersections = new Set();
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      constructor(private callback: IntersectionObserverCallback) {}
      observe() {
        intersections.add(this.callback);
      }
      disconnect() {
        intersections.delete(this.callback);
      }
    },
  );
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
it("virtualizes a thousand files and preserves thumbnail nodes while details are open", async () => {
  const open = vi.fn();
  const view = render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={files} open={open} />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  await waitFor(() => expect(screen.getAllByRole("img")).toHaveLength(9));
  const thumbnails = screen.getAllByRole("img");
  expect(screen.getAllByRole("button").length).toBeLessThan(25);
  const user = userEvent.setup();
  await user.click(screen.getAllByRole("button")[0]);
  expect(open).toHaveBeenCalledWith(files[0]);
  view.rerender(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={files} open={open} paused />
      </I18nProvider>
    </ThemeProvider>,
  );
  expect(screen.getAllByRole("img")).toEqual(thumbnails);
  view.rerender(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={files} open={open} />
      </I18nProvider>
    </ThemeProvider>,
  );
  await waitFor(() => expect(screen.getAllByRole("img")).toEqual(thumbnails));
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
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={files} open={vi.fn()} />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  await screen.findAllByRole("button");
  const buttons = screen.getAllByRole("button");
  buttons[0].focus();
  const user = userEvent.setup();
  await user.keyboard("{ArrowLeft}");
  await waitFor(() => expect(buttons[1]).toHaveFocus());
  await user.keyboard("{ArrowDown}");
  await waitFor(() => expect(buttons[4]).toHaveFocus());
});

it("automatically uses keyset pagination, shows the total and resets when filters change", async () => {
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
  const page = (name = "", paused = false) => (
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FileBrowser
            paused={paused}
            filters={{ ...emptyFilters, name }}
            tags={[]}
            open={vi.fn()}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
  const view = render(page("", true));
  await screen.findByText("1 file loaded");
  expect(intersections.size).toBe(0);
  view.rerender(page());
  await waitFor(() => expect(intersections.size).toBe(1));
  expect(
    screen.queryByRole("button", { name: "Load more" }),
  ).not.toBeInTheDocument();
  reachEnd();
  await waitFor(() => expect(screen.getAllByRole("img")).toHaveLength(2));
  expect(requests[1].searchParams.get("cursor")).toBe("next-page");
  expect(screen.getByText("All 2 files shown")).toBeVisible();
  reachEnd();
  expect(requests).toHaveLength(2);
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

it("keeps loaded files on pagination failure and retries only on request", async () => {
  let attempts = 0;
  let finish: (response: Response) => void = () => {};
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      const url = new URL(path, "http://localhost");
      if (url.pathname.includes("/derived/")) return Promise.resolve(ready());
      if (url.pathname.endsWith("/info"))
        return Promise.resolve(Response.json({ timezone: "UTC" }));
      if (!url.searchParams.get("cursor"))
        return Promise.resolve(
          Response.json({ files: [files[0]], cursor: "next-page" }),
        );
      attempts++;
      return new Promise<Response>((resolve) => {
        finish = resolve;
      });
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FileBrowser filters={emptyFilters} tags={[]} open={vi.fn()} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(intersections.size).toBe(1));
  reachEnd();
  reachEnd();
  expect(attempts).toBe(1);
  expect(await screen.findByText("Loading…")).toBeVisible();
  await act(async () => {
    finish(Response.json({ message: "offline" }, { status: 503 }));
  });
  const retry = await screen.findByRole("button", { name: "Try again" });
  expect(screen.getByRole("button", { name: /photo-0.jpg/ })).toBeVisible();
  reachEnd();
  expect(attempts).toBe(1);
  await userEvent.setup().click(retry);
  expect(attempts).toBe(2);
  expect(await screen.findByText("Loading…")).toBeVisible();
  await act(async () => {
    finish(Response.json({ files: [files[1]] }));
  });
  expect(await screen.findByText("All 2 files shown")).toBeVisible();
});

it("browses folders and files together with compact types and modification dates", async () => {
  localStorage.setItem("kist.display.files.v1", "list");
  const open = vi.fn(),
    chooseFolder = vi.fn(),
    onSort = vi.fn();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid
          files={[
            {
              ...files[0],
              name: "report.txt",
              kind: "other",
              media_type: "text/plain; charset=utf-8",
              captured_at: "2000-01-01T00:00:00Z",
              size: 41,
            },
          ]}
          folders={[
            {
              id: "subfolder",
              parent_id: "root",
              total_size: 2048,
              name: "Documents",
              created_at: files[0].created_at,
              updated_at: files[0].updated_at,
            },
          ]}
          open={open}
          chooseFolder={chooseFolder}
          filters={emptyFilters}
          onSort={onSort}
        />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  const directory = await screen.findByRole("button", { name: /Documents/ });
  const document = await screen.findByRole("button", { name: /report.txt/ });
  expect(
    directory.compareDocumentPosition(document) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(directory).toHaveTextContent("2 kB");
  expect(document).toHaveTextContent("TXT");
  expect(document).not.toHaveTextContent("text/plain");
  expect(document).not.toHaveTextContent("2000");
  expect(document).toHaveTextContent("2026");
  expect(document.querySelector(".media-placeholder, img")).toBeNull();
  const user = userEvent.setup();
  await user.click(directory);
  expect(chooseFolder).toHaveBeenCalledWith("subfolder");
  expect(open).not.toHaveBeenCalled();
  await user.click(document);
  expect(open).toHaveBeenCalledOnce();
  await user.click(screen.getByRole("button", { name: "Name" }));
  expect(onSort).toHaveBeenCalledWith("name");
  await user.click(screen.getByRole("button", { name: "Type" }));
  expect(onSort).toHaveBeenCalledWith("type");
  await user.click(screen.getByRole("button", { name: "Size" }));
  expect(onSort).toHaveBeenCalledWith("size");
});

it("opens file and folder context actions at the pointer without navigating", async () => {
  const open = vi.fn(),
    chooseFolder = vi.fn(),
    createFolder = vi.fn();
  const folder = {
    id: "child",
    name: "Child",
    parent_id: "root",
    total_size: 0,
  } as Folder;
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid
          files={[files[0]]}
          folders={[folder]}
          open={open}
          chooseFolder={chooseFolder}
          createFolder={createFolder}
        />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  const file = await screen.findByRole("button", { name: /photo-0.jpg/ });
  fireEvent.contextMenu(file, { clientX: 111, clientY: 222 });
  expect(open).not.toHaveBeenCalled();
  expect(file).toHaveAttribute("data-active", "true");
  expect(
    await screen.findByRole("menuitem", { name: "Download original" }),
  ).toHaveAttribute("href", "/api/v1/files/0/content?download=true");
  await userEvent.click(screen.getByRole("menuitem", { name: "Open" }));
  expect(open).toHaveBeenCalledWith(files[0]);
  const directory = screen.getByRole("button", { name: "Child" });
  fireEvent.keyDown(directory, { key: "F10", shiftKey: true });
  expect(chooseFolder).not.toHaveBeenCalled();
  expect(directory).toHaveAttribute("data-active", "true");
  await userEvent.click(
    await screen.findByRole("menuitem", { name: "New folder" }),
  );
  expect(createFolder).toHaveBeenCalledWith("child");
});

it("opens the same context menu from the list row action without opening the file", async () => {
  localStorage.setItem("kist.display.files.v1", "list");
  const open = vi.fn();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={[files[0]]} boxID="box" open={open} />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  const more = await screen.findByRole("button", { name: "More actions" });
  expect(more.closest(".file-card")).toBeNull();
  await userEvent.click(more);
  expect(open).not.toHaveBeenCalled();
  expect(more).toHaveAttribute("aria-expanded", "true");
  expect(await screen.findByRole("menuitem", { name: "Rename" })).toBeVisible();
  expect(screen.getByRole("menuitem", { name: "Move to trash" })).toBeVisible();
  expect(
    screen.getByRole("menuitem", { name: "Download original" }),
  ).toHaveAttribute("href", "/api/v1/files/0/content?download=true");
  await userEvent.click(screen.getByRole("menuitem", { name: "Open" }));
  expect(open).toHaveBeenCalledWith(files[0]);
  await waitFor(() => expect(more).toHaveAttribute("aria-expanded", "false"));
});

it("places folders at opposite ends for ascending and descending type order", async () => {
  localStorage.setItem("kist.display.files.v1", "list");
  const folder = {
    id: "child",
    name: "Child",
    parent_id: "root",
    total_size: 0,
  } as Folder;
  const page = (asc: boolean) => (
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid
          files={[files[0]]}
          folders={[folder]}
          filters={{ ...emptyFilters, sort: "type", asc }}
          open={vi.fn()}
        />
      </I18nProvider>
    </ThemeProvider>
  );
  const view = render(page(true), { wrapper: mediaWrapper });
  const directory = await screen.findByRole("button", { name: "Child" });
  const file = screen.getByRole("button", { name: /photo-0.jpg/ });
  expect(
    directory.compareDocumentPosition(file) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  view.rerender(page(false));
  expect(
    file.compareDocumentPosition(directory) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
});

it.each(["list", "medium"])(
  "renames files and folders inline in %s view",
  async (mode) => {
    localStorage.setItem("kist.display.files.v1", mode);
    const open = vi.fn();
    const chooseFolder = vi.fn();
    const fetch = vi.fn(async (path: string, _options?: RequestInit) =>
      Response.json(
        path.includes("/text-preview")
          ? {
              supported: false,
              text: "",
              encoding: "",
              truncated: false,
              reason: "binary",
            }
          : { op_id: 1 },
      ),
    );
    vi.stubGlobal("fetch", fetch);
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileGrid
            files={[{ ...files[0], kind: "other" }]}
            folders={[
              {
                id: "child",
                name: "Child",
                parent_id: "root",
                total_size: 0,
              } as Folder,
            ]}
            boxID="box"
            open={open}
            chooseFolder={chooseFolder}
          />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    const user = userEvent.setup();
    fireEvent.contextMenu(
      await screen.findByRole("button", { name: /photo-0.jpg/ }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    let input = screen.getByRole("textbox", { name: "Rename" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(input.closest("button")).toBeNull();
    expect(input.closest(".file-card")).toHaveAttribute("data-active", "true");
    await user.clear(input);
    await user.type(input, "changed.jpg");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Rename" })).toBeNull();
    expect(
      fetch.mock.calls.filter(([, options]) => options?.method),
    ).toHaveLength(0);
    fireEvent.contextMenu(screen.getByRole("button", { name: "Child" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    input = screen.getByRole("textbox", { name: "Rename" });
    await user.clear(input);
    await user.type(input, "New folder");
    await user.tab();
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Rename" })).toBeNull(),
    );
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/boxes/box/entries/child/name?folder=true",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ before: "Child", name: "New folder" }),
      }),
    );
    expect(open).not.toHaveBeenCalled();
    expect(chooseFolder).not.toHaveBeenCalled();
  },
);

it.each([
  [0, "All 1 folder shown"],
  [1, "All 1 folder and 1 file shown"],
])(
  "includes folders in the completion count with %i files",
  async (count, label) => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ files: files.slice(0, count) })),
      ),
    );
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileBrowser
            filters={emptyFilters}
            folders={[
              {
                id: "child",
                name: "Child",
                parent_id: "root",
                total_size: 0,
              } as Folder,
            ]}
            tags={[]}
            open={vi.fn()}
          />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    expect(await screen.findByText(label)).toBeVisible();
  },
);

it.each([
  [false, false, false],
  [true, false, false],
  [false, true, false],
  [true, true, true],
])(
  "shows scope only for a subfolder search (%s, %s)",
  (searching, searchingSubfolder, visible) => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ files: [] }))),
    );
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileBrowser
            box={{ id: "box" } as Box}
            searching={searching}
            searchingSubfolder={searchingSubfolder}
            filters={emptyFilters}
            tags={[]}
            open={vi.fn()}
          />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    const hint = screen.queryByText(
      "Search scope: current folder and subfolders",
    );
    if (visible) expect(hint).toBeVisible();
    else expect(hint).not.toBeInTheDocument();
  },
);

it("exposes canonical file links and history from the keyboard menu", async () => {
  const user = userEvent.setup();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid files={files.slice(0, 1)} open={vi.fn()} />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  fireEvent.keyDown(screen.getByRole("button", { name: /photo-0/ }), {
    key: "F10",
    shiftKey: true,
  });
  expect(
    await screen.findByRole("menuitem", { name: "Open in new tab" }),
  ).toHaveAttribute("href", "/files?file=0");
  expect(screen.getByRole("menuitem", { name: "History" })).toHaveAttribute(
    "href",
    "/history?file=0",
  );
  await user.click(screen.getByRole("menuitem", { name: "Copy link" }));
  expect(await navigator.clipboard.readText()).toBe(
    `${window.location.origin}/files?file=0`,
  );
});

it("uploads to the selected directory without opening it", async () => {
  const upload = vi.fn(),
    open = vi.fn();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileGrid
          files={[]}
          folders={[
            {
              id: "child",
              name: "Child",
              parent_id: "root",
              total_size: 0,
            } as Folder,
          ]}
          chooseFolder={open}
          uploadToFolder={upload}
          boxID="box"
          open={vi.fn()}
        />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  fireEvent.contextMenu(screen.getByRole("button", { name: "Child" }));
  fireEvent.click(
    await screen.findByRole("menuitem", { name: "Upload to this folder" }),
  );
  expect(upload).toHaveBeenCalledWith("child");
  expect(open).not.toHaveBeenCalled();
});

it("opens an upload window for the selected child folder", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      Response.json({
        files: [],
        max_upload_files: 10,
        max_upload_bytes: 10000,
      }),
    ),
  );
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FileBrowser
          box={{ id: "box" } as Box}
          uploadFolder="root"
          folders={[
            {
              id: "child",
              name: "Child",
              parent_id: "root",
              total_size: 0,
            } as Folder,
          ]}
          filters={emptyFilters}
          tags={[]}
          open={vi.fn()}
        />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  fireEvent.contextMenu(await screen.findByRole("button", { name: "Child" }));
  fireEvent.click(
    await screen.findByRole("menuitem", { name: "Upload to this folder" }),
  );
  expect(
    await screen.findByRole("dialog", {
      name: "Upload to this folder · Child",
    }),
  ).toBeVisible();
});
it("links breadcrumb menus to their own folder with keyboard access", async () => {
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FolderBreadcrumbs
          box="box"
          folders={
            [
              { id: "root", name: "", parent_id: null, total_size: 0 },
              { id: "child", name: "Child", parent_id: "root", total_size: 0 },
            ] as Folder[]
          }
          current="child"
          choose={vi.fn()}
        />
      </I18nProvider>
    </ThemeProvider>,
  );
  fireEvent.keyDown(screen.getByRole("button", { name: "Child" }), {
    key: "F10",
    shiftKey: true,
  });
  expect(
    await screen.findByRole("menuitem", { name: "Open in new tab" }),
  ).toHaveAttribute("href", "/boxes/box?folder=child");
});

it("distinguishes the archive root from the current box root", async () => {
  const goRoot = vi.fn(),
    choose = vi.fn();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FolderBreadcrumbs
          box="box"
          rootLabel="Trip"
          goRoot={goRoot}
          folders={
            [
              { id: "root", name: "", parent_id: null, total_size: 0 },
            ] as Folder[]
          }
          current="root"
          choose={choose}
        />
      </I18nProvider>
    </ThemeProvider>,
  );
  const root = screen.getByRole("link", { name: "Root" });
  expect(root).toHaveAttribute("href", "/");
  fireEvent.click(root);
  expect(goRoot).toHaveBeenCalledOnce();
  const box = screen.getByRole("button", { name: "Box" });
  expect(box).toHaveAttribute("aria-current", "page");
  expect(box).toHaveAttribute("title", "Return to box root · Trip");
  fireEvent.click(box);
  expect(choose).toHaveBeenCalledWith("root");
  fireEvent.keyDown(root, { key: "F10", shiftKey: true });
  expect(
    await screen.findByRole("menuitem", { name: "Open in new tab" }),
  ).toHaveAttribute("href", "/");
});

it.each([
  { scoped: false, start: "2026", source: "Trip · 2026" },
  { scoped: false, start: "2026-10", source: "Trip · 2026-10" },
  { scoped: false, start: "2026-10-20", source: "Trip · 2026-10-20" },
  {
    scoped: false,
    start: "2026-10-20T08:30",
    source: "Trip · 2026-10-20 08:30",
  },
  { scoped: false, start: "", source: "Trip" },
  { scoped: true, start: "2026", source: "" },
])(
  "renders card source and size without repeating the filename: %j",
  async ({ scoped, start, source }) => {
    const box: Box = {
      id: "box",
      name: "Trip",
      start_at: start,
      total_size: 2048,
      created_at: files[0].created_at,
      updated_at: files[0].updated_at,
    };
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileGrid
            files={[
              {
                ...files[0],
                box_id: box.id,
                size: 2048,
                path: "sub/photo-0.jpg",
              },
            ]}
            boxes={[box]}
            boxID={scoped ? box.id : undefined}
            open={() => {}}
          />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    const card = await screen.findByRole("button", { name: /photo-0.jpg/ });
    expect(card.querySelector(".file-card-size")).toHaveTextContent("2 kB");
    if (scoped) {
      expect(card.querySelector(".file-card-meta")).toBeNull();
      expect(
        card.querySelector(".file-name-cell .file-card-size"),
      ).toBeInTheDocument();
    } else {
      expect(card.querySelector(".file-card-source")).toHaveTextContent(source);
      await userEvent.hover(card.querySelector(".file-card-source")!);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(source);
      expect(card.querySelector(".file-card-meta")).not.toHaveTextContent(
        "photo-0.jpg",
      );
    }
  },
);

it.each(["medium", "list"])(
  "assigns file tags from the %s action menu",
  async (mode) => {
    localStorage.setItem("kist.display.files.v1", mode);
    let assigned: string[] = [];
    const taggedFile = {
      ...files[0],
      display_tags: [{ id: "existing", name: "Existing" }],
    };
    const fetch = vi.fn(async (path: string, options?: RequestInit) => {
      if (path === "/api/v1/tags/new/files" && options?.method === "PUT") {
        assigned = ["new"];
        return Response.json({ op_id: 1 });
      }
      if (path === "/api/v1/tags")
        return Response.json({
          tags: [{ id: "new", name: "Travel", parent_id: null, count: 0 }],
        });
      if (path === "/api/v1/files/0")
        return Response.json({
          file: taggedFile,
          tags: assigned,
          metadata: [],
        });
      return ready();
    });
    vi.stubGlobal("fetch", fetch);
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileGrid files={[taggedFile]} open={() => {}} />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    const card = await screen.findByRole("button", { name: /photo-0.jpg/ });
    expect(card).toHaveTextContent("Existing");
    if (mode === "list")
      await userEvent.click(
        screen.getByRole("button", { name: "More actions" }),
      );
    else fireEvent.contextMenu(card);
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "Add tag" }),
    );
    const picker = await screen.findByRole("combobox", {
      name: "Select a tag",
    });
    await userEvent.click(picker);
    await userEvent.click(
      await screen.findByRole("option", { name: /^Travel$/ }),
    );
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/tags/new/files",
        expect.objectContaining({ method: "PUT", body: '{"ids":["0"]}' }),
      ),
    );
    expect(await screen.findByRole("dialog")).toHaveTextContent("Travel");
  },
);

it.each(["medium", "list"])(
  "sets a cover from the %s file menu",
  async (mode) => {
    localStorage.setItem("kist.display.files.v1", mode);
    const fetch = vi.fn(async () => ready());
    vi.stubGlobal("fetch", fetch);
    render(
      <ThemeProvider env="test">
        <I18nProvider>
          <FileGrid
            files={[{ ...files[0], box_id: "owner" }]}
            open={() => {}}
          />
        </I18nProvider>
      </ThemeProvider>,
      { wrapper: mediaWrapper },
    );
    const card = await screen.findByRole("button", { name: /photo-0.jpg/ });
    if (mode === "list")
      await userEvent.click(
        screen.getByRole("button", { name: "More actions" }),
      );
    else fireEvent.contextMenu(card);
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "Set as box cover" }),
    );
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/boxes/owner/cover",
        expect.objectContaining({ method: "PUT", body: '{"file_id":"0"}' }),
      ),
    );
  },
);

it("falls back for a trashed cover and removes only the reference", async () => {
  const box: Box = {
    id: "owner",
    name: "Trip",
    total_size: 1,
    created_at: files[0].created_at,
    updated_at: files[0].updated_at,
    cover_file_id: "0",
  };
  let visible = false;
  const fetch = vi.fn(async (path: string, options?: RequestInit) => {
    if (path === "/api/v1/boxes/owner/cover" && options?.method !== "PUT")
      return Response.json(visible ? { file: files[0] } : {});
    return ready();
  });
  vi.stubGlobal("fetch", fetch);
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <BoxGrid boxes={[box]} open={() => {}} />
      </I18nProvider>
    </ThemeProvider>,
    { wrapper: mediaWrapper },
  );
  const card = await screen.findByRole("button", { name: /Trip/ });
  await waitFor(() =>
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/boxes/owner/cover",
      expect.anything(),
    ),
  );
  expect(card.querySelector(".box-icon")).toBeInTheDocument();
  visible = true;
  await act(() => mediaClient.invalidateQueries());
  await waitFor(() =>
    expect(card.querySelector(".box-cover img")).toBeInTheDocument(),
  );
  fireEvent.contextMenu(card);
  await userEvent.click(
    await screen.findByRole("menuitem", { name: "Remove cover" }),
  );
  await waitFor(() =>
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/boxes/owner/cover",
      expect.objectContaining({ method: "PUT", body: '{"file_id":""}' }),
    ),
  );
});
