import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { MoreActionsMenu } from "./MoreActionsMenu";
import { ObjectLinkMenuItems } from "./ObjectLinkMenuItems";

const path = "/boxes/2026_trip/" + encodeURIComponent("证件 #?%.pdf");
afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.unstubAllGlobals();
  history.replaceState(null, "", "/");
});
it("copies current-origin readable and permanent links, then switches without losing the page or hash", async () => {
  localStorage.setItem(
    "kist.display.v1",
    JSON.stringify({
      language: "zh",
      formatLocale: "zh-CN",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  const user = userEvent.setup();
  const copy = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
  history.replaceState({ kistGallery: true }, "", path + "?page=2#details");
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      Response.json({
        box: "box",
        folder: "parent",
        file: "set",
        kind: "fileset",
        path,
      }),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <MoreActionsMenu>
            <ObjectLinkMenuItems
              target={{
                files: true,
                box: "box",
                folder: "parent",
                file: "set",
                page: "2",
              }}
              switchAddress
            />
          </MoreActionsMenu>
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  expect(screen.queryByRole("menuitem")).toBeNull();
  await user.click(screen.getByRole("button", { name: "更多操作" }));
  await waitFor(() =>
    expect(
      screen.getByRole("menuitem", { name: /^复制链接$/ }),
    ).not.toBeDisabled(),
  );
  await user.click(screen.getByRole("menuitem", { name: /^复制链接$/ }));
  expect(copy).toHaveBeenLastCalledWith(location.origin + path + "?page=2");
  await user.click(screen.getByRole("menuitem", { name: "复制永久链接" }));
  expect(copy).toHaveBeenLastCalledWith(
    location.origin + "/files?file=set&page=2",
  );
  await user.click(screen.getByRole("menuitem", { name: "切换到永久链接" }));
  expect(location.pathname + location.search + location.hash).toBe(
    "/files?file=set&page=2#details",
  );
  expect(history.state).toEqual({
    kistGallery: true,
    kistScope: { box: "box", folder: "parent", tag: undefined },
  });
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
});
