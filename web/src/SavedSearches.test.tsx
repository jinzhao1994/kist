import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";
import {
  SavedSearches,
  SaveSearchButton,
  savedSearchRoute,
  searchDefinition,
  searchReferencesExist,
  type SavedSearch,
} from "./SavedSearches";
import { useNavigation } from "./navigation";
import type { Box, Tag } from "./api";

const definition = searchDefinition(
  "files",
  {
    name: "contract",
    sort: "name",
    asc: true,
    tag_not: ["tag-1"],
    kind: ["document"],
    from: "2024",
  },
  "box-1",
);
const tags = [{ id: "tag-1", name: "Private", count: 0 }] as Tag[];
const boxes = [{ id: "box-1", name: "Archive" }] as Box[];
function wrap(child: React.ReactNode) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>{child}</I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

it("saves committed conditions to the library and loads them after remount", async () => {
  let searches: SavedSearch[] = [];
  const requests: Array<{ method: string; body: unknown }> = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        const body = JSON.parse(init.body as string);
        requests.push({ method: init.method, body });
        searches = [{ id: "search-1", ...body }];
        return new Response('{"op_id":1}');
      }
      return new Response(
        JSON.stringify(
          url.endsWith("/tags")
            ? { tags }
            : url.endsWith("/boxes")
              ? { boxes }
              : { searches },
        ),
      );
    }),
  );
  const user = userEvent.setup();
  const open = vi.fn();
  const view = wrap(
    <>
      <SaveSearchButton definition={definition} />
      <SavedSearches open={open} current={definition} />
    </>,
  );
  await user.click(screen.getByRole("button", { name: "Save this search" }));
  await user.type(screen.getByRole("textbox", { name: "Name" }), "Contracts");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(
    await screen.findByRole("button", { name: "Contracts" }),
  ).toBeVisible();
  expect(requests).toEqual([
    { method: "POST", body: { name: "Contracts", definition } },
  ]);
  view.unmount();
  wrap(<SavedSearches open={open} />);
  await user.click(await screen.findByRole("button", { name: "Contracts" }));
  await waitFor(() =>
    expect(open).toHaveBeenCalledWith(savedSearchRoute(definition)),
  );
  expect(definition.filters.tag_not).toEqual(["tag-1"]);
});
it("refuses missing exclusion references without opening a broader search", async () => {
  const open = vi.fn();
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (url: string) =>
        new Response(
          JSON.stringify(
            url.endsWith("/tags")
              ? { tags: [] }
              : url.endsWith("/boxes")
                ? { boxes }
                : { searches: [{ id: "s", name: "Contracts", definition }] },
          ),
        ),
    ),
  );
  wrap(<SavedSearches open={open} />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Contracts" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "missing tag, box or folder",
  );
  expect(open).not.toHaveBeenCalled();
});
it("keeps box filters distinct and restores them from the URL", async () => {
  const d = searchDefinition("boxes", {
    name: "Trips",
    sort: "activity",
    asc: true,
    from: "2025",
    tag_all: ["tag-1"],
  });
  function Harness() {
    const nav = useNavigation();
    return (
      <>
        <button onClick={() => nav.go(savedSearchRoute(d))}>Open</button>
        <output>{JSON.stringify(nav.route)}</output>
      </>
    );
  }
  const view = render(<Harness />);
  await userEvent.click(screen.getByText("Open"));
  expect(window.location.search).toContain("box_filter=");
  expect(screen.getByRole("status")).toHaveTextContent('"sort":"activity"');
  view.unmount();
  render(<Harness />);
  expect(screen.getByRole("status")).toHaveTextContent('"name":"Trips"');
  expect(screen.getByRole("status")).toHaveTextContent('"asc":true');
  expect(screen.getByRole("status")).not.toHaveTextContent('"filters":');
});
it("validates scope IDs and does not save cursor or UI state", () => {
  expect(searchReferencesExist(definition, tags, boxes)).toBe(true);
  expect(
    searchReferencesExist({ ...definition, box_id: "gone" }, tags, boxes),
  ).toBe(false);
  expect(
    searchReferencesExist(
      { ...definition, folder_id: "gone" },
      tags,
      boxes,
      [],
    ),
  ).toBe(false);
  expect(
    searchReferencesExist({ ...definition, version: 2 }, tags, boxes),
  ).toBe(false);
  expect(
    searchDefinition("files", {
      ...definition.filters,
      sort: "name",
      cursor: "next",
    } as never).filters,
  ).not.toHaveProperty("cursor");
});

it("renames without changing conditions and confirms removal", async () => {
  let searches: SavedSearch[] = [{ id: "s", name: "Contracts", definition }];
  const writes: Array<{ method: string; body?: unknown }> = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === "PUT") {
        const body = JSON.parse(init.body as string);
        writes.push({ method: "PUT", body });
        searches = [{ id: "s", ...body }];
        return new Response('{"op_id":2}');
      }
      if (init?.method === "DELETE") {
        writes.push({ method: "DELETE" });
        searches = [];
        return new Response('{"op_id":3}');
      }
      return new Response(JSON.stringify({ searches }));
    }),
  );
  wrap(
    <SavedSearches
      open={vi.fn()}
      current={searchDefinition("boxes", { sort: "activity" })}
    />,
  );
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", {
      name: "Contracts · Edit saved search",
    }),
  );
  await user.click(screen.getByRole("menuitem", { name: "Rename" }));
  const input = screen.getByRole("textbox", { name: "Name" });
  await user.clear(input);
  await user.type(input, "Legal");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await user.click(
    await screen.findByRole("button", { name: "Legal · Edit saved search" }),
  );
  expect(writes).toEqual([
    {
      method: "PUT",
      body: {
        name: "Legal",
        definition,
        before: { name: "Contracts", definition },
      },
    },
  ]);
  await user.click(
    screen.getByRole("menuitem", { name: "Remove saved search" }),
  );
  expect(writes).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Remove" }));
  await waitFor(() => expect(writes).toHaveLength(2));
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "Legal" }),
    ).not.toBeInTheDocument(),
  );
});

it("opens favorite context actions without applying the search and reuses its rename editor", async () => {
  const open = vi.fn();
  const item = { id: "s", name: "Contracts", definition };
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => Response.json({ searches: [item] })),
  );
  wrap(<SavedSearches open={open} current={definition} />);
  const link = await screen.findByRole("button", { name: "Contracts" });
  fireEvent.contextMenu(link, { clientX: 123, clientY: 234 });
  expect(open).not.toHaveBeenCalled();
  expect(link.closest("li")).toHaveAttribute("data-active", "true");
  await userEvent.click(
    await screen.findByRole("menuitem", { name: "Rename" }),
  );
  expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(
    "Contracts",
  );
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  fireEvent.keyDown(link, { key: "F10", shiftKey: true });
  expect(await screen.findByRole("menuitem", { name: "Rename" })).toBeVisible();
});
