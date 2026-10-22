import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { LocalHistory } from "./LocalHistory";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
const entry = {
  id: "local-1",
  action: "create_box",
  state: "pending",
  occurred_at: "2026-10-21T00:00:00Z",
  origin: "user",
};
function setup(active = true) {
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
  const locked = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <LocalHistory active={active} onLocked={locked} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return locked;
}
afterEach(() => vi.unstubAllGlobals());
it.each(["pending", "accepted"])(
  "can undo a %s undo operation after explicit confirmation",
  async (state) => {
    const posts: string[] = [];
    const undone = {
      ...entry,
      id: "local-undo",
      action: "undo",
      state,
      undo: { local_id: "local-original" },
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (path: string, init?: RequestInit) => {
        if (init?.method === "POST") {
          posts.push(path);
          return new Response(JSON.stringify({ local_id: "local-redo" }));
        }
        return new Response(
          JSON.stringify(
            path.includes("/local-undo")
              ? { ...undone, proposal: { undo: undone.undo } }
              : { entries: [undone] },
          ),
        );
      }),
    );
    setup();
    await userEvent.click(await screen.findByRole("button", { name: "Undo" }));
    await screen.findByRole("button", { name: "Back to local operations" });
    const button = await screen.findByRole("button", { name: "Undo" });
    expect(posts).toHaveLength(0);
    await userEvent.click(button);
    expect(posts).toHaveLength(0);
    await userEvent.click(button);
    await waitFor(() => expect(posts).toHaveLength(1));
    expect(posts[0]).toBe("/api/v1/client/history/local-undo/undo");
  },
);
it("shows object summaries in the list without fetching each operation", async () => {
  const fetch = vi.fn(
    async () =>
      new Response(
        JSON.stringify({
          entries: [
            {
              ...entry,
              summary: { name: "Travel notes", count: 1, current_names: false },
            },
            {
              ...entry,
              id: "local-2",
              summary: { name: "Receipts", count: 1, current_names: true },
            },
          ],
        }),
      ),
  );
  vi.stubGlobal("fetch", fetch);
  setup();
  expect(
    await screen.findByRole("button", { name: /Travel notes/ }),
  ).toBeVisible();
  expect(screen.getByRole("button", { name: /Receipts/ })).toBeVisible();
  expect(fetch).toHaveBeenCalledTimes(1);
});
it("loads only when opened and never posts automatically", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  setup(false);
  expect(fetch).not.toHaveBeenCalled();
});
it("keeps the local undo key across a lost response and refreshes after retry", async () => {
  const keys: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        keys.push((init.headers as Record<string, string>)["Idempotency-Key"]);
        if (keys.length === 1) throw new TypeError("Disconnected");
        return new Response(JSON.stringify({ local_id: "undo-1" }));
      }
      return new Response(
        JSON.stringify(
          path.includes("local-1")
            ? {
                ...entry,
                summary: {
                  name: "Travel notes",
                  count: 1,
                  current_names: false,
                },
                proposal: {},
              }
            : { entries: [entry] },
        ),
      );
    }),
  );
  const locked = setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Create box" }),
  );
  expect(await screen.findByText(/Travel notes/)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Undo" }));
  await userEvent.click(screen.getByRole("button", { name: "Undo" }));
  await userEvent.click(
    await screen.findByRole("button", { name: "Try again" }),
  );
  await waitFor(() => expect(keys).toHaveLength(2));
  expect(keys[0]).toBe(keys[1]);
  await waitFor(() => expect(locked).toHaveBeenLastCalledWith(false));
});
it("follows a replacement and never offers undo for an uncertain submission", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (path: string) =>
        new Response(
          JSON.stringify(
            path.includes("local-2")
              ? {
                  ...entry,
                  id: "local-2",
                  state: "awaiting_receipt",
                  proposal: {},
                }
              : path.includes("local-1")
                ? {
                    ...entry,
                    state: "replaced",
                    replaced_by: "local-2",
                    proposal: {},
                  }
                : { entries: [{ ...entry, state: "replaced" }] },
          ),
        ),
    ),
  );
  setup();
  await userEvent.click(
    await screen.findByRole("button", { name: "Create box" }),
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "View replacement" }),
  );
  expect(await screen.findByText(/Awaiting confirmation/)).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Undo" }),
  ).not.toBeInTheDocument();
});
