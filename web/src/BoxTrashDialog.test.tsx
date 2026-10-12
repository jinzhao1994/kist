import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { BoxTrashDialog } from "./BoxTrashDialog";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
import type { Box } from "./api";

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
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
afterEach(() => vi.unstubAllGlobals());
function setup() {
  const done = vi.fn(),
    close = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <BoxTrashDialog
            box={{ id: "box", name: "Travel" } as Box}
            close={close}
            done={done}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { done, close };
}
it("confirms the full scope and submits its revision only after confirmation", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({ name: "Travel", file_count: 12, revision: 42 }),
        { status: 200 },
      ),
    )
    .mockResolvedValue(
      new Response(JSON.stringify({ op_id: 43 }), { status: 200 }),
    );
  vi.stubGlobal("fetch", fetcher);
  const { done } = setup();
  const user = userEvent.setup();
  await screen.findByText(/all 12 files/);
  expect(fetcher).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole("button", { name: "Move to trash" }));
  await waitFor(() => expect(done).toHaveBeenCalledOnce());
  expect(fetcher.mock.calls[1][0]).toBe("/api/v1/boxes/box");
  expect(fetcher.mock.calls[1][1].method).toBe("DELETE");
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ revision: 42 });
});
it("never deletes when the preview fails or the user cancels", async () => {
  const fetcher = vi.fn().mockRejectedValue(new Error("offline"));
  vi.stubGlobal("fetch", fetcher);
  const { close, done } = setup();
  const user = userEvent.setup();
  await screen.findByRole("alert");
  expect(screen.getByRole("button", { name: "Move to trash" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(close).toHaveBeenCalledOnce();
  expect(done).not.toHaveBeenCalled();
  expect(fetcher).toHaveBeenCalledTimes(1);
});
