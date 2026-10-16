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
          boxes={[
            {
              id: "trip",
              name: "Summer trip",
              description: "",
              start_at: "",
              end_at: "",
              created_at: "",
              updated_at: "",
            },
          ]}
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
it("stages conditions until Search and preserves them while sorting", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  const choose = async (field: string, value: string) => {
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: field }));
    await user.type(screen.getByRole("combobox"), value);
    await user.keyboard("{ArrowDown}{Enter}");
  };
  await choose("File type", "Image");
  expect(state().kind).toEqual([]);
  expect(
    screen.getByRole("button", { name: "File type: Image" }),
  ).toBeVisible();
  await choose("Boxes", "Summer");
  await user.click(screen.getByRole("button", { name: "Sort by" }));
  await user.click(screen.getByRole("menuitem", { name: /Descending/ }));
  expect(state().asc).toBe(true);
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(state().kind).toEqual(["image"]);
  expect(state().box).toEqual(["trip"]);
  await user.click(screen.getByRole("button", { name: "Remove: File type" }));
  expect(state().kind).toEqual(["image"]);
  await user.keyboard("{Enter}");
  expect(state().kind).toEqual([]);
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual({ ...emptyFilters, asc: true });
});
it("keeps keywords unlabelled and separates contains from equals", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  await user.type(screen.getByRole("combobox"), "photo,100%{Enter}");
  expect(state().keyword).toEqual(["photo,100%"]);
  expect(state().name).toBe("");
  expect(screen.getByRole("button", { name: "photo,100%" })).toBeVisible();
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Name" }));
  await user.click(screen.getByRole("option", { name: "contains" }));
  await user.type(screen.getByRole("combobox"), "jpg{Enter}");
  expect(state().name_contains).toBe("jpg");
  await user.click(screen.getByRole("button", { name: "Operator: Name" }));
  await user.click(screen.getByRole("option", { name: "=" }));
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "photo.jpg{Enter}");
  expect(state().name_equals).toBe("photo.jpg");
  expect(state().name_contains).toBe("");
  expect(state().keyword).toEqual(["photo,100%"]);
});
it("rejects impossible dates and Escape leaves the previous condition intact", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "From" }));
  await user.type(screen.getByRole("combobox"), "2026-02-30{Enter}");
  expect(state().from).toBe("");
  expect(screen.getByRole("alert")).toBeVisible();
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "2026-10{Enter}");
  expect(state().from).toBe("2026-10");
  await user.click(screen.getByRole("button", { name: "From: 2026-10" }));
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "2025{Escape}");
  expect(state().from).toBe("2026-10");
});
