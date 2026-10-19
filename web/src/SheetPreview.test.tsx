import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import SheetPreview, { columnName, selectedText } from "./SheetPreview";
import type { ArchiveFile } from "./api";
const data = {
  supported: true,
  sheets: ["Expenses", "Notes"],
  sheet: 0,
  row: 0,
  column: 0,
  cells: [
    [{ text: "项目" }, { text: "金额" }],
    [{ text: "交通" }, { text: "1,280.00" }],
  ],
  widths: [24, 18],
  merges: [],
  more_rows: true,
  more_columns: true,
  truncated: false,
};
function mount(detail = true, active = true) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <SheetPreview
            file={
              {
                id: "sheet",
                name: "travel.xlsx",
                blob_key: "blob",
              } as ArchiveFile
            }
            detail={detail}
            active={active}
            paused={false}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
afterEach(() => vi.unstubAllGlobals());
it("names columns and copies rectangular values with TSV escaping", () => {
  expect([0, 25, 26, 701].map(columnName)).toEqual(["A", "Z", "AA", "ZZ"]);
  expect(
    selectedText(
      [[{ text: "a\tb" }, { text: 'say "hi"' }], [{ text: "中文" }]],
      { start: { row: 1, col: 1 }, end: { row: 0, col: 0 } },
    ),
  ).toBe('"a\tb"\t"say ""hi"""\n中文\t');
});
it("reads pages and worksheets without requesting original files", async () => {
  const fetch = vi.fn(async (_url: string) => Response.json(data));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(await screen.findByText("交通")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Next rows" }));
  await waitFor(() =>
    expect(fetch.mock.calls.at(-1)?.[0]).toContain("row=100"),
  );
  await userEvent.click(screen.getByRole("button", { name: "Next columns" }));
  await waitFor(() =>
    expect(fetch.mock.calls.at(-1)?.[0]).toContain("column=32"),
  );
  await userEvent.selectOptions(screen.getByLabelText("Worksheet"), "1");
  await waitFor(() =>
    expect(fetch.mock.calls.at(-1)?.[0]).toContain("sheet=1&row=0&column=0"),
  );
});
it("selects cells, searches and copies with keyboard clipboard", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string) => Response.json(data)),
  );
  mount();
  const cell = await screen.findByText("交通");
  fireEvent.pointerDown(cell, { button: 0 });
  const region = screen.getByRole("region", { name: "Table cells" });
  fireEvent.keyDown(region, { key: "a", ctrlKey: true });
  const setData = vi.fn();
  fireEvent.copy(region, { clipboardData: { setData } });
  expect(setData).toHaveBeenCalledWith(
    "text/plain",
    "项目\t金额\n交通\t1,280.00",
  );
  await userEvent.type(screen.getByLabelText("Search this page"), "交通");
  expect(cell).toHaveClass("match");
});
it("does not fetch inactive thumbnails", () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  mount(false, false);
  expect(fetch).not.toHaveBeenCalled();
});
