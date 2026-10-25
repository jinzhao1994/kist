import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider, useTheme } from "./theme";
function ThemeProbe() {
  const { appearance } = useTheme();
  return (
    <span data-testid="active-color" hidden>
      {appearance.primaryColor}
    </span>
  );
}
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <App />
          <ThemeProbe />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "auto",
    }),
  );
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (path: string) =>
        new Response(
          JSON.stringify(
            path.includes("/boxes/search?")
              ? { boxes: [] }
              : { version: "test", timezone: "UTC", projection_dirty: false },
          ),
        ),
    ),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  history.replaceState(null, "", "/");
});
it("keeps committed searches in the list URL and restores empty history entries", async () => {
  history.replaceState(null, "", "/files");
  const view = mount();
  const user = userEvent.setup();
  await user.type(
    await screen.findByRole("combobox", { name: "Search or add a condition…" }),
    "travel{Enter}{Escape}",
  );
  const filteredURL = location.href;
  expect(new URL(filteredURL).searchParams.get("q")).toBe("travel");
  await user.click(screen.getByRole("button", { name: "Boxes" }));
  history.back();
  await waitFor(() => expect(location.href).toBe(filteredURL));
  expect(
    await screen.findByRole("button", {
      name: "travel",
    }),
  ).toBeVisible();
  history.pushState(null, "", "/files");
  window.dispatchEvent(new PopStateEvent("popstate"));
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "travel" }),
    ).not.toBeInTheDocument(),
  );
  view.unmount();
});
it("renders an empty archive and persists independent language, locale and RTL choices", async () => {
  mount();
  const user = userEvent.setup();
  expect(
    await screen.findByText("A place for the things you want to keep."),
  ).toBeVisible();
  const settings = await screen.findByRole("button", {
    name: "Display settings",
  });
  await user.click(settings);
  expect(screen.getByRole("dialog")).toBeVisible();
  await user.click(screen.getByRole("combobox", { name: "Language" }));
  await user.click(await screen.findByRole("option", { name: "中文" }));
  const locale = screen.getByRole("combobox", { name: "地区格式" });
  await user.click(locale);
  await user.type(screen.getByRole("textbox", { name: "搜索选项" }), "de-DE");
  await user.click(screen.getByRole("option", { name: /de-DE/ }));
  await user.click(screen.getByRole("combobox", { name: "阅读方向" }));
  await user.click(screen.getByRole("option", { name: "从右到左" }));
  await user.click(screen.getByRole("button", { name: "关闭" }));
  await waitFor(() => expect(document.documentElement.dir).toBe("rtl"));
  expect(document.documentElement.lang).toBe("zh");
  expect(
    screen.getByRole("heading", { name: "归档盒", level: 1 }),
  ).toBeVisible();
  expect(JSON.parse(localStorage.getItem(storageKey)!).formatLocale).toBe(
    "de-DE",
  );
}, 15_000);
it("closes the dialog with Escape and restores keyboard focus", async () => {
  mount();
  const user = userEvent.setup();
  const settings = await screen.findByRole("button", {
    name: "Display settings",
  });
  await user.click(settings);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await waitFor(() => expect(settings).toHaveFocus());
});

it("opens global tag results from a box and preserves the tag URL on back and reload", async () => {
  const time = "2026-01-01T00:00:00Z";
  const current = {
    total_size: 0,
    id: "current",
    name: "Current box",
    description: "",
    start_at: "",
    end_at: "",
    created_at: time,
    updated_at: time,
  };
  const matching = { ...current, id: "matching", name: "Tagged box" };
  const requests: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      requests.push(path);
      const url = new URL(path, "http://localhost");
      let data: unknown;
      if (url.pathname.endsWith("/boxes/search"))
        data = {
          boxes: url.searchParams.has("tag_all")
            ? [matching]
            : [current, matching],
        };
      else if (url.pathname.endsWith("/folders"))
        data = {
          folders: [
            {
              id: "root",
              parent_id: null,
              name: "",
              total_size: 0,
              created_at: time,
              updated_at: time,
            },
          ],
        };
      else if (url.pathname === "/api/v1/boxes/current") data = current;
      else if (url.pathname === "/api/v1/boxes/matching") data = matching;
      else if (
        url.pathname.includes("/boxes/") &&
        url.pathname.endsWith("/tags")
      )
        data = { tags: [] };
      else if (url.pathname.endsWith("/tags"))
        data = {
          tags: [
            {
              id: "travel",
              name: "Travel",
              parent_id: null,
              count: 3,
              box_count: 1,
              file_count: 2,
            },
          ],
        };
      else if (url.pathname.endsWith("/files"))
        data = { files: [], cursor: "" };
      else
        data = {
          version: "test",
          timezone: "UTC",
          projection_dirty: false,
          max_upload_bytes: 1000000,
          max_upload_files: 100,
        };
      return new Response(JSON.stringify(data));
    }),
  );
  history.replaceState(null, "", "/boxes/current");
  let view = mount();
  const user = userEvent.setup();
  const tree = await screen.findByRole("tree", { name: "Tags" });
  expect(await within(tree).findByTitle("1 box · 2 files")).toHaveTextContent(
    "3",
  );
  await user.click(within(tree).getByText("Travel"));
  expect(location.pathname).toBe("/tags/travel");
  expect(
    await screen.findByRole("heading", { name: "Travel", level: 1 }),
  ).toBeVisible();
  const boxes = await screen.findByRole("region", { name: "Boxes" });
  await within(boxes).findByRole("button", { name: /Tagged box/ });
  expect(within(boxes).queryByText("Current box")).not.toBeInTheDocument();
  await waitFor(() =>
    expect(
      requests.some(
        (path) =>
          path.includes("/files?") &&
          path.includes("tag_all=travel") &&
          !path.includes("box="),
      ),
    ).toBe(true),
  );
  await user.click(within(boxes).getByRole("button", { name: /Tagged box/ }));
  expect(location.pathname).toBe("/boxes/matching");
  history.back();
  await waitFor(() =>
    expect(
      screen.getByRole("heading", { name: "Travel", level: 1 }),
    ).toBeVisible(),
  );
  view.unmount();
  view = mount();
  expect(
    await screen.findByRole("heading", { name: "Travel", level: 1 }),
  ).toBeVisible();
  expect(
    await screen.findByRole("heading", { name: "Files", level: 2 }),
  ).toBeVisible();
  view.unmount();
  history.replaceState(null, "", "/");
});

it("replaces the box search bar when switching to all files and applies a box condition", async () => {
  const current = {
    total_size: 0,
    id: "current",
    name: "Current box",
    description: "",
    start_at: "",
    end_at: "",
    created_at: "",
    updated_at: "",
  };
  const requests: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      requests.push(path);
      const pathname = new URL(path, "http://localhost").pathname;
      const data = pathname.endsWith("/folders")
        ? {
            folders: [
              {
                id: "root",
                parent_id: null,
                name: "",
                total_size: 0,
                created_at: "",
                updated_at: "",
              },
            ],
          }
        : pathname === "/api/v1/boxes/current"
          ? current
          : pathname === "/api/v1/boxes"
            ? { boxes: [current] }
            : pathname.endsWith("/tags")
              ? { tags: [] }
              : pathname.endsWith("/files")
                ? { files: [], cursor: "" }
                : {
                    version: "test",
                    timezone: "UTC",
                    max_upload_bytes: 1000000,
                    max_upload_files: 100,
                  };
      return new Response(JSON.stringify(data));
    }),
  );
  history.replaceState(null, "", "/boxes/current");
  const view = mount();
  const user = userEvent.setup();
  await screen.findByRole("heading", { name: "Current box", level: 1 });
  expect(await screen.findByText("No files or folders yet")).toBeVisible();
  expect(screen.getByRole("button", { name: "Display mode" })).toBeVisible();
  expect(screen.getByRole("button", { name: "Upload files" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: /All files/ }));
  expect(screen.getAllByRole("region", { name: "Filters" })).toHaveLength(1);
  await user.type(
    screen.getByRole("combobox", { name: "Search or add a condition…" }),
    "Boxes",
  );
  await user.click(screen.getByRole("option", { name: "Boxes" }));
  await user.click(await screen.findByRole("option", { name: "Current box" }));
  await user.click(screen.getByRole("button", { name: "Search" }));
  await waitFor(() =>
    expect(
      requests.some(
        (path) =>
          path.includes("/files?") &&
          new URL(path, "http://localhost").searchParams
            .getAll("condition")
            .some((raw) => {
              const c = JSON.parse(raw);
              return c.field === "box" && c.values.includes("current");
            }),
      ),
    ).toBe(true),
  );
  view.unmount();
  history.replaceState(null, "", "/");
});

it("sorts boxes from list headings and keeps the search when switching views", async () => {
  const requests: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      requests.push(path);
      return new Response(
        JSON.stringify(
          path.includes("/boxes/search?")
            ? {
                boxes: [
                  {
                    id: "box-layout",
                    name: "Archive",
                    total_size: 0,
                    description: "A short description",
                    start_at: "2025",
                    end_at: "2026",
                    created_at: "2026-01-01T00:00:00Z",
                    updated_at: "2026-01-01T00:00:00Z",
                  },
                ],
              }
            : { version: "test", timezone: "UTC", projection_dirty: false },
        ),
      );
    }),
  );
  mount();
  const user = userEvent.setup();
  await screen.findByRole("heading", { name: "Archive" });
  await user.type(
    screen.getByRole("combobox", { name: "Search or add a condition…" }),
    "Archive{Enter}{Escape}",
  );
  // Wait for the debounced search to replace the initial query before using
  // its headings: a refetch can otherwise detach the button mid-click.
  await waitFor(() =>
    expect(
      requests.some((path) =>
        new URL(path, "http://localhost").searchParams
          .getAll("condition")
          .some((raw) => {
            const c = JSON.parse(raw);
            return c.field === "keyword" && c.values.includes("Archive");
          }),
      ),
    ).toBe(true),
  );
  await screen.findByRole("heading", { name: "Archive" });
  await user.click(screen.getByRole("button", { name: "Display mode" }));
  await user.click(screen.getByRole("menuitem", { name: "List" }));
  expect(
    screen.queryByRole("button", { name: "Sort by" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /^Activity date:/ }));
  await waitFor(() =>
    expect(new URL(location.href).searchParams.get("b.sort")).toBe("activity"),
  );
  const descending = await screen.findByRole("button", {
    name: "Activity date: Descending",
  });
  await user.click(descending);
  await screen.findByRole("button", { name: "Activity date: Ascending" });
  expect(new URL(location.href).searchParams.get("b.asc")).toBe("1");
  expect(new URL(location.href).searchParams.get("b.q")).toBe("Archive");
  await user.click(screen.getByRole("button", { name: "Display mode" }));
  await user.click(screen.getByRole("menuitem", { name: "Small icons" }));
  expect(screen.getByRole("button", { name: "Sort by" })).toBeVisible();
  expect(new URL(location.href).searchParams.get("b.sort")).toBe("activity");
});

it("opens a folder's combination in its parent list and returns there after reload", async () => {
  const time = "2026-01-01T00:00:00Z";
  const current = {
    id: "box",
    name: "Archive",
    total_size: 0,
    description: "",
    start_at: "",
    end_at: "",
    created_at: time,
    updated_at: time,
  };
  const root = {
    id: "root",
    parent_id: null,
    name: "",
    total_size: 0,
    created_at: time,
    updated_at: time,
  };
  const folder = { ...root, id: "set", parent_id: "root", name: "Documents" };
  const set = {
    folder_id: "set",
    kind: "document_pages",
    layout: "directory",
    primary: "00000000-0000-0000-0000-000000000000",
    members: [],
  };
  const entry = {
    id: "set",
    fileset_id: "set",
    name: "Documents",
    folder_id: "root",
    box_id: "box",
    size: 0,
    created_at: time,
    updated_at: time,
  };
  const requests: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      requests.push(input);
      const url = new URL(input, "http://localhost");
      const path = url.pathname;
      let data: unknown;
      if (path === "/api/v1/filesets") data = { filesets: [set] };
      else if (path === "/api/v1/filesets/set")
        data = { fileset: set, folder, box_id: "box" };
      else if (path.endsWith("/folders")) data = { folders: [root, folder] };
      else if (path === "/api/v1/boxes/box") data = current;
      else if (path.endsWith("/boxes/search") || path.endsWith("/boxes"))
        data = { boxes: [current] };
      else if (path.endsWith("/tags")) data = { tags: [] };
      else if (path.endsWith("/entries"))
        data = {
          files:
            url.searchParams.get("folder") === "root"
              ? [
                  entry,
                  {
                    ...entry,
                    id: "other",
                    fileset_id: undefined,
                    name: "other.txt",
                  },
                ]
              : [],
        };
      else if (path.endsWith("/files")) data = { files: [] };
      else
        data = {
          version: "test",
          timezone: "UTC",
          projection_dirty: false,
          max_upload_bytes: 1000000,
          max_upload_files: 100,
        };
      return Response.json(data);
    }),
  );
  history.replaceState(null, "", "/boxes/box?folder=set&q=member-only");
  let view = mount();
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "View file set" }),
  );
  await screen.findByRole("region", { name: "Gallery" });
  expect(new URL(location.href).searchParams.get("folder")).toBe("root");
  expect(new URL(location.href).searchParams.get("file")).toBe("set");
  expect(new URL(location.href).searchParams.has("q")).toBe(false);
  const navigation = screen.getByRole("navigation", { name: "Browse files" });
  await waitFor(() => expect(navigation).toHaveTextContent("1 / 2"));
  expect(
    within(navigation).getByRole("button", { name: "Next file" }),
  ).toBeEnabled();
  view.unmount();
  view = mount();
  await screen.findByRole("region", { name: "Gallery" });
  await user.click(screen.getByRole("button", { name: "Back to results" }));
  await waitFor(() =>
    expect(
      screen.queryByRole("region", { name: "Gallery" }),
    ).not.toBeInTheDocument(),
  );
  expect(new URL(location.href).searchParams.get("folder")).toBe("root");
  expect(new URL(location.href).searchParams.has("file")).toBe(false);
  expect(
    requests.some(
      (path) =>
        path.includes("/entries?") &&
        new URL(path, "http://localhost").searchParams.get("folder") === "root",
    ),
  ).toBe(true);
  view.unmount();
});

it("waits for the instance color before showing the application shell", async () => {
  const originalFetch = globalThis.fetch;
  let resolve!: (response: Response) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, options?: RequestInit) =>
      path.endsWith("/info")
        ? new Promise<Response>((done) => {
            resolve = done;
          })
        : originalFetch(path, options),
    ),
  );
  const view = mount();
  expect(screen.getByRole("status")).toHaveTextContent("Loading");
  expect(view.container.querySelector(".app-shell")).toBeNull();
  resolve(
    Response.json({ version: "test", timezone: "UTC", primary_color: "pink" }),
  );
  expect(
    await screen.findByRole("button", { name: "Display settings" }),
  ).toBeVisible();
  expect(screen.getByTestId("active-color")).toHaveTextContent("pink");
  view.unmount();
});
