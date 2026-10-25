import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import SheetPreview, { columnName, selectedText } from "./SheetPreview";
import type { ArchiveFile } from "./api";
vi.mock("./UniverSheet", async () => {
  const { useImperativeHandle } = await import("react");
  return {
    default: ({
      data,
      onSelection,
      ref,
    }: {
      data: import("./sheetModel").SheetData;
      onSelection: (s: import("./sheetModel").Selection | null) => void;
      ref: import("react").Ref<import("./UniverSheet").SheetHandle>;
    }) => {
      useImperativeHandle(ref, () => ({
        select(row: number, col: number) {
          onSelection({ start: { row, col }, end: { row, col } });
        },
      }));
      return (
        <div>
          {data.cells?.flatMap((row, r) =>
            row?.map((cell, c) => (
              <button
                key={`${r}:${c}`}
                onClick={() =>
                  onSelection({
                    start: { row: r, col: c },
                    end: { row: r, col: c },
                  })
                }
              >
                {cell.text}
              </button>
            )),
          )}
        </div>
      );
    },
  };
});
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
it("connects native selections and search to the copy toolbar", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => Response.json(data)),
  );
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
  mount();
  await screen.findByText("交通");
  await user.type(screen.getByLabelText("Search this page"), "交通");
  await user.click(screen.getByRole("button", { name: "Next match" }));
  expect(screen.getByRole("status")).toHaveTextContent("A2–A2 · 交通");
  await user.click(screen.getByRole("button", { name: "Copy selection" }));
  expect(write).toHaveBeenCalledWith("交通");
});
it("does not fetch inactive thumbnails", () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  mount(false, false);
  expect(fetch).not.toHaveBeenCalled();
});

it("reuses a thumbnail until its content or filename changes", async () => {
  const fetcher = vi.fn(async (_url: string) => Response.json(data));
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const file = {
    id: "csv",
    name: "prices.csv",
    blob_key: "sha256:first",
  } as ArchiveFile;
  const view = (current: ArchiveFile, visible = true) => (
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          {visible && (
            <SheetPreview file={current} detail={false} active paused={false} />
          )}
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
  const { rerender, unmount } = render(view(file));
  await screen.findByText("交通");
  expect(decodeURIComponent(String(fetcher.mock.calls[0]?.[0]))).toContain(
    "v=sheet-v1:sha256:first:prices.csv",
  );
  rerender(view(file, false));
  rerender(view(file));
  await screen.findByText("交通");
  expect(fetcher).toHaveBeenCalledTimes(1);
  rerender(view({ ...file, name: "prices.tsv" }));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  rerender(view({ ...file, blob_key: "sha256:second" }));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
  unmount();
  client.clear();
});
