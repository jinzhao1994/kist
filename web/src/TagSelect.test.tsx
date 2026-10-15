import { useState } from "react";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TagManager, TagTree } from "./Tags";
import { TagSelect } from "./TagSelect";
import { flattenTags, tagOption } from "./TagPath";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
import type { Tag } from "./api";
const tags: Tag[] = [
  {
    id: "a",
    name: "Education",
    parent_id: null,
    count: 0,
    children: [
      {
        id: "b",
        name: "School",
        parent_id: "a",
        count: 0,
        children: [{ id: "c", name: "Reports", parent_id: "b", count: 0 }],
      },
    ],
  },
];
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
});
function Harness() {
  const [value, setValue] = useState("c");
  return (
    <>
      <label htmlFor="tag">Tag</label>
      <TagSelect
        id="tag"
        value={value}
        onValueChange={setValue}
        options={flattenTags(tags).map(tagOption)}
      />
      <output>{value}</output>
    </>
  );
}
function wrap(child: React.ReactNode) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider env="test">
        <I18nProvider>{child}</I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("renders ancestor paths separately without indentation and supports keyboard search", async () => {
  wrap(<Harness />);
  const user = userEvent.setup();
  const selected = screen.getByRole("combobox", { name: "Tag" });
  expect(selected).toHaveTextContent("Education / School / Reports");
  expect(within(selected).getByText("Education / School /")).toHaveClass(
    "tag-path-prefix",
  );
  await user.click(selected);
  const search = screen.getByPlaceholderText("Search tags");
  await user.type(search, "Education / School / Rep");
  expect(screen.getAllByRole("option")).toHaveLength(1);
  await user.keyboard("{ArrowDown}{Enter}");
  expect(screen.queryByRole("option")).not.toBeInTheDocument();
  expect(selected).toHaveFocus();
  await user.click(selected);
  await user.click(screen.getByRole("option", { name: "Education / School" }));
  expect(selected).toHaveTextContent("Education / School");
  expect(screen.getByRole("status")).toHaveTextContent("b");
});
it("excludes the selected tag and descendants from parent choices", async () => {
  wrap(<TagManager tags={tags} close={() => {}} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Select a tag" }));
  await user.click(screen.getByRole("option", { name: "Education / School" }));
  await user.click(screen.getByRole("combobox", { name: "Parent tag" }));
  expect(
    screen.queryByRole("option", { name: "Education / School" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("option", {
      name: "Education / School / Reports",
    }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("option", { name: "Education" })).toBeInTheDocument();
});

it("keeps tree branches independently collapsed and separates expansion from filtering", async () => {
  const choose = vi.fn();
  function RefreshingTree() {
    const [items, setItems] = useState(tags);
    return (
      <>
        <button onClick={() => setItems(structuredClone(tags))}>Refresh</button>
        <TagTree tags={items} choose={choose} />
      </>
    );
  }
  wrap(<RefreshingTree />);
  const user = userEvent.setup();
  expect(screen.queryByText("School")).not.toBeInTheDocument();
  await user.click(screen.getByText("Education"));
  expect(choose).toHaveBeenLastCalledWith("a");
  expect(screen.queryByText("School")).not.toBeInTheDocument();
  choose.mockClear();
  await user.click(screen.getByRole("button", { name: "Expand Education" }));
  expect(choose).not.toHaveBeenCalled();
  expect(screen.getByText("School")).toBeInTheDocument();
  expect(screen.queryByText("Reports")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Expand School" }));
  await user.click(screen.getByRole("button", { name: "Refresh" }));
  expect(screen.getByText("Reports")).toBeInTheDocument();
  await user.click(screen.getByText("Reports"));
  expect(choose).toHaveBeenLastCalledWith("c");
  await user.click(screen.getByRole("button", { name: "Collapse Education" }));
  expect(screen.queryByText("School")).not.toBeInTheDocument();
  const root = screen.getByRole("treeitem");
  root.focus();
  await user.keyboard("{ArrowRight}{ArrowRight}{Enter}");
  expect(choose).toHaveBeenLastCalledWith("b");
  await user.keyboard("{ArrowLeft}");
  expect(screen.queryByText("Reports")).not.toBeInTheDocument();
});

afterEach(() => {
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

it("retains the parent for consecutive new tags and resets it on reopening", async () => {
  const requests: { name: string; parent_id?: string }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, init: RequestInit) => {
      requests.push(JSON.parse(String(init.body)));
      return new Response(JSON.stringify({ op_id: requests.length }));
    }),
  );
  function Manager() {
    const [open, setOpen] = useState(true);
    return (
      <>
        <button onClick={() => setOpen(true)}>Open manager</button>
        {open && <TagManager tags={tags} close={() => setOpen(false)} />}
      </>
    );
  }
  wrap(<Manager />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Parent tag" }));
  await user.click(screen.getByRole("option", { name: "Education / School" }));
  for (const name of ["Math", "Science"]) {
    const input = screen.getByRole("textbox", { name: "Name" });
    await user.type(input, name);
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(input).toHaveValue(""));
    expect(input).toHaveFocus();
    expect(screen.getByRole("dialog")).toBeVisible();
    expect(
      screen.getByRole("combobox", { name: "Parent tag" }),
    ).toHaveTextContent("Education / School");
  }
  expect(requests).toEqual([
    { name: "Math", parent_id: "b" },
    { name: "Science", parent_id: "b" },
  ]);
  await user.click(screen.getByRole("button", { name: "Close" }));
  await user.click(screen.getByRole("button", { name: "Open manager" }));
  expect(
    screen.getByRole("combobox", { name: "Parent tag" }),
  ).not.toHaveTextContent("Education");
  await user.type(screen.getByRole("textbox", { name: "Name" }), "Travel");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(requests).toHaveLength(3));
  expect(requests[2]).toEqual({ name: "Travel" });
});

it("renames through the context menu without filtering and cancels with Escape", async () => {
  const choose = vi.fn();
  const fetcher = vi.fn(
    async (_url: RequestInfo | URL, _init?: RequestInit) =>
      new Response("{}", { status: 200 }),
  );
  vi.stubGlobal("fetch", fetcher);
  wrap(<TagTree tags={tags} choose={choose} />);
  const user = userEvent.setup();
  fireEvent.contextMenu(screen.getByText("Education"));
  await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
  const input = await screen.findByRole("textbox", { name: "Rename" });
  await user.clear(input);
  await user.type(input, "Learning{Enter}");
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
  expect(String(fetcher.mock.calls[0]?.[0])).toContain("/tags/a/name");
  expect(JSON.parse(fetcher.mock.calls[0]?.[1]?.body as string)).toEqual({
    name: "Learning",
  });
  await waitFor(() =>
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument(),
  );
  await user.click(
    screen.getByRole("button", { name: "Actions for Education" }),
  );
  await user.click(screen.getByRole("menuitem", { name: "Rename" }));
  await user.type(
    screen.getByRole("textbox", { name: "Rename" }),
    "discard{Escape}",
  );
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(choose).not.toHaveBeenCalled();
});
it("confirms inline rename on outside click without submitting empty or unchanged names", async () => {
  const fetcher = vi.fn(async () => new Response("{}", { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  wrap(
    <>
      <TagTree tags={tags} choose={vi.fn()} />
      <button>Outside</button>
    </>,
  );
  const user = userEvent.setup();
  const outside = screen.getByRole("button", { name: "Outside" });
  const edit = async () => {
    fireEvent.contextMenu(screen.getByText("Education"));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    return screen.getByRole("textbox", { name: "Rename" });
  };
  await edit();
  await user.click(outside);
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(fetcher).not.toHaveBeenCalled();

  const input = await edit();
  await user.clear(input);
  await user.click(outside);
  expect(input).toBeInTheDocument();
  expect(fetcher).not.toHaveBeenCalled();

  await user.type(input, "Learning");
  await user.click(outside);
  await waitFor(() =>
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument(),
  );
  expect(fetcher).toHaveBeenCalledTimes(1);
});

it("creates children under the clicked tag and excludes descendants from parent choices", async () => {
  const fetcher = vi.fn(
    async (_url: RequestInfo | URL, _init?: RequestInit) =>
      new Response("{}", { status: 200 }),
  );
  vi.stubGlobal("fetch", fetcher);
  wrap(<TagTree tags={tags} choose={vi.fn()} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Expand Education" }));
  await user.click(screen.getByRole("button", { name: "Actions for School" }));
  await user.click(screen.getByRole("menuitem", { name: "Add child tag" }));
  await user.type(screen.getByRole("textbox", { name: "Name" }), "Notes");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
  expect(JSON.parse(fetcher.mock.calls[0]?.[1]?.body as string)).toEqual({
    name: "Notes",
    parent_id: "b",
  });
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  await user.click(screen.getByRole("button", { name: "Actions for School" }));
  await user.click(screen.getByRole("menuitem", { name: "Change parent tag" }));
  await user.click(screen.getByRole("combobox", { name: "Parent tag" }));
  expect(
    screen.getAllByRole("option").map((option) => option.textContent),
  ).toEqual(["No parent", "Education"]);
});
it("requires confirmation before deleting through the keyboard context menu", async () => {
  const fetcher = vi.fn(
    async (_url: RequestInfo | URL, _init?: RequestInit) =>
      new Response("{}", { status: 200 }),
  );
  vi.stubGlobal("fetch", fetcher);
  const choose = vi.fn();
  wrap(<TagTree tags={tags} choose={choose} />);
  const user = userEvent.setup();
  screen.getByRole("treeitem").focus();
  await user.keyboard("{Shift>}{F10}{/Shift}");
  await user.click(screen.getByRole("menuitem", { name: "Remove" }));
  expect(fetcher).not.toHaveBeenCalled();
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "Remove" }),
  );
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
  expect(fetcher.mock.calls[0]?.[1]?.method).toBe("DELETE");
  expect(choose).not.toHaveBeenCalled();
});
