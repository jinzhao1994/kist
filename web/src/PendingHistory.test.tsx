import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { HistoryPage } from "./History";
import { I18nProvider } from "./i18n";
import { ThemeProvider } from "./theme";

afterEach(() => vi.unstubAllGlobals());
it("shows pending deletion in scoped trash and restores it using its local identity", async () => {
  let restored = false;
  const entry = {
    id: "local-delete",
    action: "delete_entry_contents",
    state: "pending",
    occurred_at: "2026-10-25T00:00:00Z",
    origin: "user",
    summary: { name: "offline.txt", count: 1 },
  };
  const fetcher = vi.fn(async (raw: string, init?: RequestInit) => {
    const url = new URL(raw, "http://localhost");
    if (init?.method === "POST") {
      expect(url.pathname).toBe("/api/v1/client/history/local-delete/undo");
      restored = true;
      return Response.json({ local_id: "local-restore" });
    }
    if (url.pathname === "/api/v1/client/pending-history") {
      expect(url.searchParams.get("box")).toBe("box");
      expect(url.searchParams.get("trash")).toBe("true");
      return Response.json({ entries: restored ? [] : [entry] });
    }
    if (url.pathname === "/api/v1/client/history/local-delete")
      return Response.json({
        ...entry,
        proposal: {},
        undone_by: restored ? { local_id: "local-restore" } : undefined,
      });
    if (url.pathname === "/api/v1/ops") return Response.json({ ops: [] });
    throw new Error(`Unexpected request: ${raw}`);
  });
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <HistoryPage client trash box="box" boxes={[]} scope={() => {}} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  expect(await screen.findByText(/offline.txt/)).toBeVisible();
  expect(
    screen.queryByText("No deleted items in this scope"),
  ).not.toBeInTheDocument();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Restore" }));
  const dialog = await screen.findByRole("dialog");
  await user.click(
    await within(dialog).findByRole("button", { name: "Restore" }),
  );
  expect(within(dialog).getByText(/Restore all items/)).toBeVisible();
  await user.click(within(dialog).getByRole("button", { name: "Restore" }));
  await waitFor(() => expect(restored).toBe(true));
  await waitFor(() =>
    expect(
      within(dialog).queryByRole("button", { name: "Restore" }),
    ).not.toBeInTheDocument(),
  );
  expect(
    await screen.findByText("No deleted items in this scope"),
  ).toBeVisible();
  view.unmount();
  client.clear();
});
