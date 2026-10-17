import { ThemeProvider } from "./theme";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { HistoryPage, comparisonFields } from "./History";
import { I18nProvider, storageKey, translator } from "./i18n";
import type { HistoricalFile } from "./api";
const preferences = {
  language: "en",
  formatLocale: "en-US",
  timezone: "UTC",
  direction: "auto",
} as const;
const op = {
  id: 5,
  action: "delete_files",
  ts: "2026-01-02T12:00:00Z",
  origin: "user",
  reverts_id: 2,
  undone_by: null,
  undoable: true,
};
const detail = { ...op, boxes: ["box"], files: ["file"], payload: {} };
const historical: HistoricalFile = {
  op: 4,
  file: {
    file: {
      id: "file",
      name: "old.txt",
      path: "old.txt",
      folder_id: "folder",
      blob_key: "sha256:old",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      captured_at: null,
    },
    tags: ["tag"],
    metadata: [{ key: "file:size", value: "42", type: "int", source: "file" }],
    extracted: [],
  },
  tags: [{ id: "tag", name: "Old tag", count: 1, parent_id: null }],
};
function mount(file?: string, trash = false) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <HistoryPage file={file} trash={trash} boxes={[]} scope={() => {}} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}
beforeEach(() => {
  localStorage.setItem(storageKey, JSON.stringify(preferences));
  sessionStorage.clear();
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  sessionStorage.clear();
});
it("pages deletion history and reuses the restore request key after a lost response", async () => {
  let attempts = 0;
  const keys: string[] = [];
  const fetch = vi.fn(async (path: string, options?: RequestInit) => {
    if (options?.method === "POST") {
      keys.push((options.headers as Record<string, string>)["Idempotency-Key"]);
      if (++attempts === 1) throw new TypeError("lost response");
      return json({ op_id: 6 });
    }
    if (path === "/api/v1/ops/5") return json(detail);
    if (path.includes("before=5"))
      return json({ ops: [{ ...op, id: 2 }], before: 0 });
    return json({ ops: [op], before: 5 });
  });
  vi.stubGlobal("fetch", fetch);
  mount(undefined, true);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Load more" }));
  expect(await screen.findByText("#2")).toBeVisible();
  expect(
    fetch.mock.calls.some(
      ([path]) => path.includes("trash=true") && path.includes("before=5"),
    ),
  ).toBe(true);
  await user.click(screen.getAllByRole("button", { name: /Delete files/ })[0]);
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", {
      name: "Restore",
    }),
  );
  await user.click(
    screen
      .getAllByRole("button", { name: "Restore" })
      .find((button) => button.getAttribute("data-variant") === "filled")!,
  );
  await user.click(await screen.findByRole("button", { name: "Try again" }));
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBe(keys[1]);
});
it("shows the blocking operation and lets the user inspect it", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, options?: RequestInit) => {
      if (options?.method === "POST")
        return json(
          {
            code: "undo_blocked",
            class: "conflict",
            message: "blocked",
            blocking_op: 7,
          },
          409,
        );
      if (path === "/api/v1/ops/7")
        return json({ ...detail, id: 7, action: "tag_files" });
      if (path === "/api/v1/ops/5") return json(detail);
      return json({ ops: [op] });
    }),
  );
  mount();
  const user = userEvent.setup();
  await user.click(
    (await screen.findAllByRole("button", { name: /Delete files/ }))[0],
  );
  await user.click(await screen.findByRole("button", { name: "Restore" }));
  await user.click(
    screen
      .getAllByRole("button", { name: "Restore" })
      .find((button) => button.getAttribute("data-variant") === "filled")!,
  );
  await user.click(
    await screen.findByRole("button", { name: /Blocked by operation #7/ }),
  );
  expect(
    await screen.findByRole("heading", { name: "Operation #7" }),
  ).toBeVisible();
});
it("compares deletion to the historical file without requesting current media or writes", async () => {
  const fetch = vi.fn(async (path: string) => {
    if (path.includes("/history/4/files/file")) return json(historical);
    if (path.includes("/history/5/files/file"))
      return json(
        { code: "file_not_found", class: "not_found", message: "missing" },
        404,
      );
    return json({ ops: [op] });
  });
  vi.stubGlobal("fetch", fetch);
  mount("file");
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "Compare versions" }),
  );
  expect(await screen.findByText("Not present at this point")).toBeVisible();
  expect(screen.getByText("Old tag")).toBeVisible();
  expect(
    screen.getByRole("link", { name: "Download original" }),
  ).toHaveAttribute("href", "/api/v1/files/file/content?at=4&download=true");
  expect(
    fetch.mock.calls.every(
      ([path]) => path.includes("/ops?") || path.includes("/history/"),
    ),
  ).toBe(true);
});
it("does not disguise a missing cutoff as a file deletion", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      path.includes("/history/")
        ? json(
            { code: "op_not_found", class: "not_found", message: "missing op" },
            404,
          )
        : json({ ops: [op] }),
    ),
  );
  mount("file");
  await userEvent
    .setup()
    .click(await screen.findByRole("button", { name: "Compare versions" }));
  expect(await screen.findAllByRole("alert")).toHaveLength(2);
  expect(
    screen.queryByText("Not present at this point"),
  ).not.toBeInTheDocument();
});
it("uses tag paths from each historical state and preserves exact metadata strings", () => {
  const t = translator(preferences);
  const fields = comparisonFields(
    {
      ...historical,
      file: {
        ...historical.file,
        metadata: [
          {
            key: "file:mtime",
            value: "1791586856333712820",
            type: "int",
            source: "file",
          },
        ],
      },
    },
    t,
  );
  expect(fields.Tags).toBe("Old tag");
  expect(fields["file:mtime"]).toContain("1791586856333712820");
});

it("opens restore confirmation from the trash row without writing", async () => {
  const fetch = vi.fn(async (path: string) =>
    json(path === "/api/v1/ops/5" ? detail : { ops: [op], before: 0 }),
  );
  vi.stubGlobal("fetch", fetch);
  mount(undefined, true);
  await userEvent
    .setup()
    .click(await screen.findByRole("button", { name: "Restore" }));
  const dialog = await screen.findByRole("dialog");
  expect(
    await within(dialog).findByText(
      "Restore all items from this deletion? Later changes may prevent restoration.",
    ),
  ).toBeVisible();
  expect(fetch.mock.calls.every(([path]) => !path.endsWith("/undo"))).toBe(
    true,
  );
});

it("shows a deleted file name once and only one restore confirmation action", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      json(
        path === "/api/v1/ops/5"
          ? {
              ...detail,
              payload: {
                name: "deleted.txt",
                files: [{ file_id: "file", name: "deleted.txt" }],
              },
            }
          : { ops: [op], before: 0 },
      ),
    ),
  );
  mount(undefined, true);
  await userEvent
    .setup()
    .click(await screen.findByRole("button", { name: "Restore" }));
  const dialog = within(await screen.findByRole("dialog"));
  expect(await dialog.findAllByText("deleted.txt")).toHaveLength(1);
  expect(dialog.getAllByRole("button", { name: "Restore" })).toHaveLength(1);
});
