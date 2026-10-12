import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
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
