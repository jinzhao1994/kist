import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <App />
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
    screen.getByRole("combobox", { name: "Search or add a condition…" }),
    "travel{Enter}{Escape}",
  );
  const filteredURL = location.href;
  expect(new URL(filteredURL).searchParams.get("filter")).toContain("travel");
  await user.click(screen.getByRole("button", { name: "Boxes" }));
  history.back();
  await waitFor(() => expect(location.href).toBe(filteredURL));
  expect(
    await screen.findByRole("button", {
      name: "Filename: travel",
    }),
  ).toBeVisible();
  history.pushState(null, "", "/files");
  window.dispatchEvent(new PopStateEvent("popstate"));
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "Filename: travel" }),
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
  const settings = screen.getByRole("button", { name: "Display settings" });
  await user.click(settings);
  expect(screen.getByRole("dialog")).toBeVisible();
  await user.click(screen.getByRole("combobox", { name: "Language" }));
  await user.click(await screen.findByRole("option", { name: "中文" }));
  const locale = screen.getByRole("combobox", { name: "地区格式" });
  await user.click(locale);
  await user.clear(locale);
  await user.type(locale, "de-DE", { skipClick: true });
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
  const settings = screen.getByRole("button", { name: "Display settings" });
  await user.click(settings);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await waitFor(() => expect(settings).toHaveFocus());
});

it("opens global tag results from a box and preserves the tag URL on back and reload", async () => {
  const time = "2026-01-01T00:00:00Z";
  const current = {
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
  await within(boxes).findByRole("button", { name: "Tagged box" });
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
  await user.click(within(boxes).getByRole("button", { name: "Tagged box" }));
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
  await user.click(screen.getByRole("button", { name: /All files/ }));
  expect(screen.getAllByRole("region", { name: "Filters" })).toHaveLength(1);
  await user.type(
    screen.getByRole("combobox", { name: "Search or add a condition…" }),
    "Boxes",
  );
  await user.click(screen.getByRole("option", { name: "Boxes" }));
  await user.click(await screen.findByRole("option", { name: "Current box" }));
  await waitFor(() =>
    expect(
      requests.some(
        (path) => path.includes("/files?") && path.includes("box=current"),
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
    expect(requests.some((path) => path.includes("text=Archive"))).toBe(true),
  );
  await screen.findByRole("heading", { name: "Archive" });
  await user.click(screen.getByRole("button", { name: "Display mode" }));
  await user.click(screen.getByRole("menuitem", { name: "List" }));
  expect(
    screen.queryByRole("button", { name: "Sort by" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /^Activity date:/ }));
  await waitFor(() =>
    expect(new URL(location.href).searchParams.get("box_filter")).toContain(
      '"sort":"activity"',
    ),
  );
  const descending = await screen.findByRole("button", {
    name: "Activity date: Descending",
  });
  await user.click(descending);
  await screen.findByRole("button", { name: "Activity date: Ascending" });
  expect(
    JSON.parse(new URL(location.href).searchParams.get("box_filter")!).asc,
  ).toBe(true);
  expect(new URL(location.href).searchParams.get("box_filter")).toContain(
    "Archive",
  );
  await user.click(screen.getByRole("button", { name: "Display mode" }));
  await user.click(screen.getByRole("menuitem", { name: "Small icons" }));
  expect(screen.getByRole("button", { name: "Sort by" })).toBeVisible();
  expect(new URL(location.href).searchParams.get("box_filter")).toContain(
    '"sort":"activity"',
  );
});
