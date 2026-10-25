import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { MoreActionsMenu } from "./MoreActionsMenu";
import { OfflineMenuItem } from "./OfflineMenuItem";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("cancels retention of the whole combination and then offers preparation without deleting originals", async () => {
  localStorage.setItem(
    "kist.display.v1",
    JSON.stringify({
      language: "zh",
      formatLocale: "zh-CN",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  let retained = true;
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    if (input === "/api/v1/client/offline")
      return Response.json(
        retained ? { "fileset:set": { files: [], filesets: ["set"] } } : {},
      );
    if (
      input === "/api/v1/client/offline/fileset%3Aset" &&
      init?.method === "DELETE"
    ) {
      retained = false;
      return Response.json({});
    }
    throw new Error("Unexpected request: " + input);
  });
  vi.stubGlobal("fetch", fetch);
  const prepare = vi.fn();
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <MoreActionsMenu>
            <OfflineMenuItem
              target={{ id: "set", name: "Pages", fileset: true }}
              prepare={prepare}
            />
          </MoreActionsMenu>
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "更多操作" }));
  await user.click(
    await screen.findByRole("menuitem", { name: "取消组合离线保留" }),
  );
  await waitFor(() => expect(retained).toBe(false));
  expect(fetch).toHaveBeenCalledWith(
    "/api/v1/client/offline/fileset%3Aset",
    expect.objectContaining({ method: "DELETE", body: "{}" }),
  );
  await user.click(
    await screen.findByRole("menuitem", { name: "保留整个组合离线" }),
  );
  expect(prepare).toHaveBeenCalledOnce();
});
