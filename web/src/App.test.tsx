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
});
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
