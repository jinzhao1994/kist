import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ClientConflictEditor } from "./ClientConflictEditor";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
let bodies: any[];
let fail: boolean;
let supported: boolean;
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
  bodies = [];
  fail = false;
  supported = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, options?: RequestInit) => {
      if (options?.method === "POST") {
        bodies.push(JSON.parse(options.body as string));
        if (fail) {
          fail = false;
          throw new TypeError("lost response");
        }
        return new Response(JSON.stringify({}), { status: 200 });
      }
      return new Response(
        JSON.stringify({
          summary: {},
          fields: supported
            ? [
                {
                  key: "name",
                  path: ["name", "after"],
                  before: "Original title",
                  current: "NAS title",
                  proposed: "Laptop title",
                },
              ]
            : [],
          replacement: supported
            ? {
                version: 1,
                id: "stable-replacement",
                occurred_at: "2026-10-21T10:00:00Z",
                action: "update_box",
                origin: "user",
                payload: {
                  box_id: "box",
                  name: { before: "NAS title", after: "Laptop title" },
                },
              }
            : null,
        }),
        { status: 200 },
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup() {
  const close = vi.fn(),
    locked = vi.fn();
  const rendered = render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <ClientConflictEditor
            id="conflict"
            action="update_box"
            close={close}
            onLocked={locked}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { close, locked, ...rendered };
}
it("shows the compared values and keeps the exact replacement when retrying a lost response", async () => {
  fail = true;
  const { close, locked, unmount } = setup();
  const input = await screen.findByLabelText("Value to submit");
  expect(screen.getByText(/Before your change · Original title/)).toBeVisible();
  expect(screen.getByText(/Current value · NAS title/)).toBeVisible();
  await userEvent.clear(input);
  await userEvent.type(input, "Reconciled title");
  await userEvent.click(
    screen.getByRole("button", { name: "Save revised change" }),
  );
  await screen.findByRole("alert");
  expect(input).toBeDisabled();
  expect(close).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(bodies).toHaveLength(2);
  expect(bodies[1]).toEqual(bodies[0]);
  expect(bodies[0].replacement.id).toBe("stable-replacement");
  expect(bodies[0].replacement.payload.name).toEqual({
    before: "NAS title",
    after: "Reconciled title",
  });
  unmount();
  expect(locked).toHaveBeenLastCalledWith(false);
});
it("does not offer an unsafe generic overwrite for unsupported changes", async () => {
  supported = false;
  setup();
  expect(
    await screen.findByText(/This change requires its own domain editor/),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Save revised change" }),
  ).not.toBeInTheDocument();
  expect(bodies).toEqual([]);
});
