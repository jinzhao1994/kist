import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  render,
  screen,
  within,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { FilesetEditor, zeroID, type Fileset } from "./Filesets";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});
it("reorders pages with pointer and keyboard, and saves removals without changing the primary or deleting files", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  const time = "2026-01-01T00:00:00Z";
  const folder = {
    id: "set",
    name: "Documents",
    total_size: 0,
    parent_id: "root",
    created_at: time,
    updated_at: time,
  };
  const set: Fileset = {
    folder_id: "set",
    kind: "document_pages",
    layout: "directory",
    primary: "first",
    members: ["first", "second"].map((id) => ({
      file_id: id,
      folder_id: zeroID,
      role: "page",
      language: "",
      default: false,
    })),
  };
  const fetcher = vi.fn(async (input: string, init?: RequestInit) => {
    if (init?.method === "PUT") return Response.json({ op_id: 1 });
    if (input.includes("/folders")) return Response.json({ folders: [folder] });
    return Response.json({
      files: ["first", "second", "extra"].map((id) => ({
        id,
        name: `${id}.jpg`,
        folder_id: "set",
        box_id: "box",
        created_at: time,
        updated_at: time,
      })),
    });
  });
  vi.stubGlobal("fetch", fetcher);
  const close = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <FilesetEditor folder={folder} box="box" sets={[set]} close={close} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  const first = await screen.findByRole("button", {
    name: "Reorder first.jpg",
  });
  const list = screen.getByRole("list", { name: "Pages" });
  const order = () =>
    within(list)
      .getAllByRole("listitem")
      .map((row) => row.textContent);
  const initial = order();
  expect(screen.queryByLabelText("Role")).not.toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Primary file" })).toBeVisible();
  first.setPointerCapture = vi.fn();
  within(list)
    .getAllByRole("listitem")
    .forEach((row, index) =>
      vi.spyOn(row, "getBoundingClientRect").mockReturnValue({
        top: index * 50,
        bottom: (index + 1) * 50,
      } as DOMRect),
    );
  fireEvent(first, new MouseEvent("pointerdown", { bubbles: true, button: 0 }));
  fireEvent(
    first,
    new MouseEvent("pointermove", { bubbles: true, clientY: 75 }),
  );
  fireEvent(first, new MouseEvent("pointerup", { bubbles: true }));
  expect(order()[0]).toContain("second.jpg");
  fireEvent.keyDown(first, { key: "ArrowUp" });
  expect(order()).toEqual(initial);
  fireEvent.keyDown(first, { key: "ArrowDown" });
  expect(order()[0]).toContain("second.jpg");
  fireEvent.click(
    screen.getByRole("button", { name: "Remove second.jpg from pages" }),
  );
  expect(within(list).getAllByRole("listitem")).toHaveLength(1);
  fireEvent.click(screen.getByRole("combobox", { name: "Add page" }));
  fireEvent.click(await screen.findByRole("option", { name: "extra.jpg" }));
  expect(within(list).getAllByRole("listitem")).toHaveLength(2);
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(close).toHaveBeenCalled());
  const write = fetcher.mock.calls.find(([, init]) => init?.method === "PUT")!;
  const payload = JSON.parse(write[1]!.body as string);
  expect(payload.after.primary).toBe("first");
  expect(
    payload.after.members.map((m: { file_id: string }) => m.file_id),
  ).toEqual(["first", "extra"]);
  expect(payload.before).toEqual(set);
  expect(fetcher.mock.calls.some(([, init]) => init?.method === "DELETE")).toBe(
    false,
  );
});
