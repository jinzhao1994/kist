import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <I18nProvider>
          <App />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "auto",
    }),
  );
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (path: string) =>
        new Response(
          JSON.stringify(
            path.endsWith("/boxes")
              ? { boxes: [] }
              : { version: "test", timezone: "UTC", projection_dirty: false },
          ),
        ),
    ),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("renders an empty archive and persists independent language, locale and RTL choices", async () => {
  mount();
  const user = userEvent.setup();
  expect(
    await screen.findByText("A place for the things you want to keep."),
  ).toBeVisible();
  const settings = screen.getByRole("button", { name: "Display settings" });
  await user.click(settings);
  expect(screen.getByRole("dialog")).toBeVisible();
  await user.selectOptions(screen.getByLabelText("Language"), "zh");
  const locale = screen.getByLabelText("Regional format");
  await user.clear(locale);
  await user.type(locale, "de-DE");
  await user.selectOptions(screen.getByLabelText("Reading direction"), "rtl");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(document.documentElement.dir).toBe("rtl"));
  expect(document.documentElement.lang).toBe("zh");
  expect(
    screen.getByRole("heading", { name: "归档盒", level: 1 }),
  ).toBeVisible();
  expect(JSON.parse(localStorage.getItem(storageKey)!).formatLocale).toBe(
    "de-DE",
  );
});
it("closes the dialog with Escape and restores keyboard focus", async () => {
  mount();
  const user = userEvent.setup();
  const settings = screen.getByRole("button", { name: "Display settings" });
  await user.click(settings);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await waitFor(() => expect(settings).toHaveFocus());
});
