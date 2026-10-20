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
              total_size: 0,
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
  const values = (field: string) =>
    (state().condition || [])
      .map((raw: string) => JSON.parse(raw))
      .filter((c: { field: string }) => c.field === field)
      .flatMap((c: { values: string[] }) => c.values);
  const choose = async (field: string, value: string) => {
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: field }));
    await user.type(screen.getByRole("combobox"), value);
    await user.keyboard("{ArrowDown}{Enter}");
  };
  await choose("File type", "Image");
  expect(values("kind")).toEqual([]);
  expect(
    screen.getByRole("button", { name: "File type: Image" }),
  ).toBeVisible();
  await choose("Boxes", "Summer");
  await user.click(screen.getByRole("button", { name: "Sort by" }));
  await user.click(screen.getByRole("menuitem", { name: /Descending/ }));
  expect(state().asc).toBe(true);
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(values("kind")).toEqual(["image"]);
  expect(values("box")).toEqual(["trip"]);
  await user.click(
    screen.getByRole("button", { name: "Remove: File type: Image" }),
  );
  expect(values("kind")).toEqual(["image"]);
  await user.keyboard("{Enter}");
  expect(values("kind")).toEqual([]);
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(state()).toEqual({ ...emptyFilters, asc: true });
});
it("keeps keywords unlabelled and separates contains from equals", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  const values = (field: string) =>
    (state().condition || [])
      .map((raw: string) => JSON.parse(raw))
      .filter((c: { field: string }) => c.field === field)
      .flatMap((c: { values: string[] }) => c.values);
  await user.type(screen.getByRole("combobox"), "photo,100%{Enter}");
  expect(values("keyword")).toEqual(["photo,100%"]);
  expect(state().name).toBe("");
  expect(screen.getByRole("button", { name: "photo,100%" })).toBeVisible();
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Name" }));
  await user.click(screen.getByRole("option", { name: "contains" }));
  await user.type(screen.getByRole("combobox"), "jpg{Enter}");
  expect(values("name_contains")).toEqual(["jpg"]);
  await user.click(screen.getByRole("button", { name: "Operator: Name" }));
  await user.click(screen.getByRole("option", { name: "=" }));
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "photo.jpg{Enter}");
  expect(values("name_equals")).toEqual(["photo.jpg"]);
  expect(values("name_contains")).toEqual([]);
  expect(values("keyword")).toEqual(["photo,100%"]);
});
it("rejects impossible dates and Escape leaves the previous condition intact", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  const values = (field: string) =>
    (state().condition || [])
      .map((raw: string) => JSON.parse(raw))
      .filter((c: { field: string }) => c.field === field)
      .flatMap((c: { values: string[] }) => c.values);
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Start date" }));
  await user.type(screen.getByRole("combobox"), "2026-02-30{Enter}");
  expect(values("from")).toEqual([]);
  expect(screen.getByRole("alert")).toBeVisible();
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "2026-10{Enter}");
  expect(values("from")).toEqual(["2026-10"]);
  await user.click(screen.getByRole("button", { name: "Start date: 2026-10" }));
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "2025{Escape}");
  expect(values("from")).toEqual(["2026-10"]);
});

it("appends repeated name and tag clauses and edits only the selected instance", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const clauses = () =>
    JSON.parse(screen.getByTestId("filters").textContent!).condition.map(
      (raw: string) => JSON.parse(raw),
    );
  async function name(value: string, operator = "contains") {
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "Name" }));
    await user.click(screen.getByRole("option", { name: operator }));
    await user.type(screen.getByRole("combobox"), value + "{Enter}");
  }
  await name("123");
  await name("456");
  await name("456");
  expect(clauses()).toEqual([
    { field: "name_contains", values: ["123"] },
    { field: "name_contains", values: ["456"] },
    { field: "name_contains", values: ["456"] },
  ]);
  await user.click(screen.getByRole("button", { name: "Name: 123" }));
  await user.clear(screen.getByRole("combobox"));
  await user.type(screen.getByRole("combobox"), "789{Enter}");
  expect(clauses()[0].values).toEqual(["789"]);
  expect(
    clauses()
      .slice(1)
      .map((c: { values: string[] }) => c.values),
  ).toEqual([["456"], ["456"]]);
  await user.click(screen.getByRole("button", { name: "Remove: Name: 789" }));
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(clauses()).toHaveLength(2);
  for (const tag of ["Travel", "Work"]) {
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "Tags" }));
    expect(
      screen.queryByRole("option", { name: "All of these tags" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Any of these tags" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("option", { name: tag }));
  }
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(clauses().slice(2)).toEqual([
    { field: "tag_all", values: ["one"] },
    { field: "tag_all", values: ["two"] },
  ]);
  expect(
    screen
      .getAllByRole("button", { name: "Operator: Tags" })
      .map((el) => el.textContent),
  ).toEqual(["=", "="]);
  await user.click(
    screen.getByRole("button", { name: "Remove: Tags: Travel" }),
  );
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(clauses().at(-1)).toEqual({ field: "tag_all", values: ["two"] });
  await name("A", "=");
  await name("B", "=");
  expect(clauses().slice(-2)).toEqual([
    { field: "name_equals", values: ["A"] },
    { field: "name_equals", values: ["B"] },
  ]);
});

it("keeps segmented prefixes while editing and offers the value directly", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Name" }));
  await user.click(screen.getByRole("option", { name: "contains" }));
  expect(screen.getByText("Name")).toHaveClass("filter-token-segment");
  expect(screen.getByText("contains")).toHaveClass("filter-token-segment");
  await user.type(screen.getByRole("combobox"), "456");
  await user.click(screen.getByRole("option", { name: "456" }));
  await user.click(screen.getByRole("button", { name: "Name: 456" }));
  expect(screen.getByRole("combobox")).toHaveValue("456");
  expect(screen.getByText("Name")).toHaveClass("filter-token-segment");
  expect(screen.getByText("contains")).toHaveClass("filter-token-segment");
  expect(screen.getByRole("option", { name: "456" })).toBeVisible();
});

it("edits member scope alongside conditions and submits the existing API flag", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const state = () => JSON.parse(screen.getByTestId("filters").textContent!);
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Files within sets" }));
  await user.click(screen.getByRole("option", { name: "Include" }));
  expect(
    screen.getByRole("button", {
      name: "Files within sets: Include",
    }),
  ).toBeVisible();
  expect(state().include_members).toBe(false);
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(state().include_members).toBe(true);
  expect(state().condition).toEqual([]);
  await user.click(
    screen.getByRole("button", {
      name: "Files within sets: Include",
    }),
  );
  await user.click(screen.getByRole("option", { name: "Exclude" }));
  expect(state().include_members).toBe(true);
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(state().include_members).toBe(false);
  await user.click(screen.getByRole("combobox"));
  await user.click(screen.getByRole("option", { name: "Files within sets" }));
  await user.click(screen.getByRole("option", { name: "Include" }));
  await user.click(screen.getByRole("button", { name: "Search" }));
  await user.click(
    screen.getByRole("button", {
      name: "Remove: Files within sets: Include",
    }),
  );
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(state().include_members).toBe(false);
});
