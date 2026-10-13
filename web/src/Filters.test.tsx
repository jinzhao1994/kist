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
it("builds typed conditions and edits them without losing other filters or sorting", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  const choose = async (field: string, value: string) => {
    await user.type(
      screen.getByRole("combobox", { name: "Search or add a condition…" }),
      field,
    );
    await user.click(screen.getByRole("option", { name: field }));
    await user.type(screen.getByRole("combobox", { name: field }), value);
    await user.keyboard("{ArrowDown}{Enter}");
  };
  await choose("File type", "Image");
  expect(state().kind).toEqual(["image"]);
  await choose("Boxes", "Summer");
  expect(state().box).toEqual(["trip"]);
  await choose("All of these tags", "Travel");
  await choose("None of these tags", "Work");
  expect(state().tag_all).toEqual(["one"]);
  expect(state().tag_not).toEqual(["two"]);
  await user.click(screen.getByRole("button", { name: "File type: Image" }));
  await user.type(screen.getByRole("combobox", { name: "File type" }), "Video");
  await user.keyboard("{ArrowDown}{Enter}");
  expect(state().kind).toEqual(["image", "video"]);
  await user.click(screen.getByRole("button", { name: "Remove: File type" }));
  expect(state().kind).toEqual([]);
  await user.click(screen.getByRole("button", { name: "Ascending" }));
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual({ ...emptyFilters, asc: true });
});

it("confirms text and valid dates, cancels drafts and keeps existing conditions", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  await user.type(
    screen.getByRole("combobox", { name: "Search or add a condition…" }),
    "photo",
  );
  expect(state().name).toBe("");
  await user.keyboard("{Enter}");
  expect(state().name).toBe("photo");
  await user.click(screen.getByRole("option", { name: "From" }));
  await user.type(
    screen.getByRole("combobox", { name: "From" }),
    "2026-02-30{Enter}",
  );
  expect(state().from).toBe("");
  expect(screen.getByRole("alert")).toBeVisible();
  await user.clear(screen.getByRole("combobox", { name: "From" }));
  await user.type(
    screen.getByRole("combobox", { name: "From" }),
    "2026-10{Enter}",
  );
  expect(state().from).toBe("2026-10");
  await user.click(screen.getByRole("button", { name: "From: 2026-10" }));
  await user.clear(screen.getByRole("combobox", { name: "From" }));
  await user.type(
    screen.getByRole("combobox", { name: "From" }),
    "2025{Escape}",
  );
  expect(state().from).toBe("2026-10");
  expect(state().name).toBe("photo");
  await user.click(screen.getByRole("button", { name: "Remove: Filename" }));
  expect(state().name).toBe("");
});
