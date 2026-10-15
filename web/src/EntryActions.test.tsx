import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";
import { EntryActions } from "./EntryActions";

beforeEach(() => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  sessionStorage.clear();
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function mount(action: "rename" | "trash", folder = false) {
  const close = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <EntryActions
            entry={{ id: "entry", box: "box", name: "old.txt", folder, action }}
            close={close}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return close;
}
it("renames using the original name as a concurrency precondition", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  const close = mount("rename");
  fireEvent.change(screen.getByLabelText("Name"), {
    target: { value: "new.txt" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(close).toHaveBeenCalled());
  expect(fetch).toHaveBeenCalledWith(
    "/api/v1/boxes/box/entries/entry/name",
    expect.objectContaining({
      method: "PUT",
      body: JSON.stringify({ before: "old.txt", name: "new.txt" }),
    }),
  );
});
it("requires confirmation with subtree counts and the preview revision", async () => {
  const fetch = vi.fn(async (_url: unknown, init?: RequestInit) =>
    Response.json(
      init?.method === "DELETE"
        ? { op_id: 2 }
        : { name: "folder", file_count: 12, folder_count: 3, revision: 7 },
    ),
  );
  vi.stubGlobal("fetch", fetch);
  const close = mount("trash", true);
  await screen.findByText("Includes 12 files and 3 subfolders.");
  expect(fetch.mock.calls.every(([, init]) => init?.method !== "DELETE")).toBe(
    true,
  );
  fireEvent.click(screen.getByRole("button", { name: "Move to trash" }));
  await waitFor(() => expect(close).toHaveBeenCalled());
  expect(fetch).toHaveBeenCalledWith(
    "/api/v1/boxes/box/entries/entry?folder=true",
    expect.objectContaining({
      method: "DELETE",
      body: JSON.stringify({ revision: 7 }),
    }),
  );
});
it("keeps the dialog open when a concurrent rename is rejected", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => Response.json({ code: "stale_before" }, { status: 409 })),
  );
  const close = mount("rename");
  fireEvent.change(screen.getByLabelText("Name"), {
    target: { value: "new.txt" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByRole("alert");
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Try again" })).toBeEnabled();
});
