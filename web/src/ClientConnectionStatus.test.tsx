import { afterEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { ClientConnectionStatus } from "./ClientConnectionStatus";

afterEach(() => vi.unstubAllGlobals());
it.each([
  ["connected", "Remote connected"],
  ["unreachable", "Remote unavailable"],
  ["unauthorized", "Invalid credential"],
  ["incompatible", "Remote mismatch"],
  ["unconfigured", "Remote not configured"],
])("shows the probed %s connection state", async (state, label) => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(Response.json({ state }))),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider>
          <ClientConnectionStatus openSettings={() => {}} />
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  expect(await screen.findByText(label)).toBeInTheDocument();
  view.unmount();
  client.clear();
});
