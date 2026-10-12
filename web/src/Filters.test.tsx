import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it } from "vitest";
import { FiltersPanel, emptyFilters } from "./Filters";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

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
  const [value, setValue] = useState(emptyFilters);
  return (
    <ThemeProvider env="test">
      <I18nProvider>
        <FiltersPanel
          value={value}
          onChange={setValue}
          tags={[
            { id: "one", name: "Travel", parent_id: null, count: 0 },
            { id: "two", name: "Work", parent_id: null, count: 0 },
          ]}
        />
        <output data-testid="filters">{JSON.stringify(value)}</output>
      </I18nProvider>
    </ThemeProvider>
  );
}
it("adds, edits and removes independent conditions while preserving sorting on clear", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  expect(
    screen.queryByRole("combobox", { name: "File type" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Add filter" }));
  await user.click(screen.getByRole("button", { name: "File type" }));
  await user.click(screen.getByRole("combobox", { name: "File type" }));
  await user.click(await screen.findByRole("option", { name: "Image" }));
  expect(state().kind).toEqual(["image"]);
  await user.click(screen.getByRole("button", { name: "Close" }));
  await user.click(screen.getByRole("button", { name: "File type: Image" }));
  await user.click(screen.getByRole("combobox", { name: "File type" }));
  await user.click(await screen.findByRole("option", { name: "Video" }));
  expect(state().kind).toEqual(["image", "video"]);
  await user.click(screen.getByRole("button", { name: "Close" }));
  await user.click(screen.getByRole("button", { name: "Remove: File type" }));
  expect(state().kind).toEqual([]);
  const sort = screen.getByRole("combobox", { name: "Sort by" });
  sort.focus();
  await user.keyboard("{ArrowDown}{ArrowDown}{Enter}");
  const selectedSort = state().sort;
  expect(selectedSort).not.toBe("captured");
  await user.click(screen.getByRole("button", { name: "Ascending" }));
  for (const [label, tag] of [
    ["All of these tags", "Travel"],
    ["None of these tags", "Work"],
  ]) {
    await user.click(screen.getByRole("button", { name: "Add filter" }));
    await user.click(screen.getByRole("button", { name: label }));
    await user.click(screen.getByRole("combobox", { name: label }));
    await user.click(await screen.findByRole("option", { name: tag }));
    await user.click(screen.getByRole("button", { name: "Close" }));
  }
  expect(state().tag_all).toEqual(["one"]);
  expect(state().tag_not).toEqual(["two"]);
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual({ ...emptyFilters, sort: selectedSort, asc: true });
});

it("applies partial-precision dates only on confirmation and clears filename search", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  await user.click(screen.getByRole("button", { name: "Add filter" }));
  await user.click(screen.getByRole("button", { name: "From" }));
  await user.type(screen.getByRole("textbox", { name: "From" }), "2026-10");
  expect(state().from).toBe("");
  await user.click(screen.getByRole("button", { name: "Apply" }));
  expect(state().from).toBe("2026-10");
  await user.click(screen.getByRole("button", { name: "From: 2026-10" }));
  await user.clear(screen.getByRole("textbox", { name: "From" }));
  await user.type(screen.getByRole("textbox", { name: "From" }), "2025{Enter}");
  expect(state().from).toBe("2025");
  await user.type(screen.getByRole("searchbox"), "photo");
  expect(state().name).toBe("photo");
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual(emptyFilters);
});
