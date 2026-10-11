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
it("selects and clears kinds, supports keyboard sorting, and keeps tag filters independent", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  await user.click(screen.getByRole("combobox", { name: "File type" }));
  await user.click(await screen.findByRole("option", { name: "Image" }));
  expect(state().kind).toEqual(["image"]);
  await user.click(screen.getByRole("combobox", { name: "File type" }));
  await user.click(await screen.findByRole("option", { name: "All types" }));
  expect(state().kind).toEqual([]);
  const sort = screen.getByRole("combobox", { name: "Sort by" });
  sort.focus();
  await user.keyboard("{ArrowDown}{ArrowDown}{Enter}");
  expect(state().sort).not.toBe("captured");
  await user.click(screen.getByRole("combobox", { name: "All of these tags" }));
  await user.click(await screen.findByRole("option", { name: "Travel" }));
  await user.keyboard("{Escape}");
  expect(state().tag_all).toEqual(["one"]);
  expect(state().tag_not).toEqual([]);
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual(emptyFilters);
});
