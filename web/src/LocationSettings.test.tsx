import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { LocationSettings } from "./LocationSettings";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("edits endpoint policy together without starting a lookup", async () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  let saved = {
    automatic: true,
    endpoint: "https://photon.komoot.io/reverse",
    request_interval_seconds: 2,
    pause_on_error: true,
    revision: "one",
  };
  const writes: (typeof saved)[] = [];
  const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith("/geo/settings")) {
      if (init?.method === "PUT") {
        saved = JSON.parse(String(init.body));
        writes.push(saved);
      }
      return Response.json(saved);
    }
    if (url.endsWith("/geo") && !init?.method)
      return Response.json({
        paused: false,
        queued: 0,
        failed: 0,
        running: "",
        scanning: false,
      });
    throw new Error("Unexpected request");
  });
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">
          <LocationSettings />
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  expect(screen.getByRole("heading", { name: "GPS" })).toBeVisible();
  const input = await screen.findByRole("textbox", {
    name: "Request interval (seconds)",
  });
  const pause = screen.getByRole("checkbox", {
    name: "Pause background queries on failure",
  });
  expect(input).toHaveValue("2");
  expect(pause).toBeChecked();
  const user = userEvent.setup();
  await user.clear(input);
  expect(
    screen.getByRole("button", { name: "Save GPS settings" }),
  ).toBeDisabled();
  await user.type(input, "5");
  await user.click(pause);
  expect(writes).toHaveLength(0);
  await user.click(screen.getByRole("button", { name: "Save GPS settings" }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({
    request_interval_seconds: 5,
    pause_on_error: false,
  });
  const labels = [...view.container.querySelectorAll("label")].map(
    (el) => el.textContent,
  );
  expect(labels.indexOf("Photon endpoint")).toBeLessThan(
    labels.indexOf("Request interval (seconds)"),
  );
  expect(labels.indexOf("Request interval (seconds)")).toBeLessThan(
    labels.indexOf("Pause background queries on failure"),
  );
  view.unmount();
  client.clear();
});
