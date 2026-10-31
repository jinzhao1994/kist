import { Table } from "@mantine/core";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AIProxyRow } from "./AIProxyRow";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

beforeEach(() =>
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  ),
);
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
function view(saved = true, revision = "one") {
  return (
    <ThemeProvider env="test">
      <I18nProvider>
        <Table>
          <Table.Tbody>
            <Table.Tr>
              <AIProxyRow
                number={1}
                name="Synthetic"
                type="HTTP"
                address="http://127.0.0.1:9"
                revision={revision}
                saved={saved}
                disabled={false}
              >
                {null}
              </AIProxyRow>
            </Table.Tr>
          </Table.Tbody>
        </Table>
      </I18nProvider>
    </ThemeProvider>
  );
}
const exit = {
  ip: "192.0.2.1",
  country: "Example",
  region: "Region",
  city: "City",
  elapsed_ms: 120,
  error: "",
};

it("tests only on demand and shows the saved proxy's exit details", async () => {
  let finish!: (value: Response) => void;
  const fetch = vi.fn(
    () =>
      new Promise<Response>((resolve) => {
        finish = resolve;
      }),
  );
  vi.stubGlobal("fetch", fetch);
  render(view());
  expect(fetch).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Test" }));
  expect(screen.getByRole("button", { name: "Testing…" })).toBeDisabled();
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch).toHaveBeenCalledWith(
    "/api/v1/ai/proxies/1/test",
    expect.objectContaining({ method: "POST", body: "{}" }),
  );
  finish(Response.json(exit));
  await waitFor(() =>
    expect(screen.getByRole("status")).toHaveTextContent(
      "Exit IP: 192.0.2.1 · Example · Region · City · 120 ms",
    ),
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Test" })).toBeEnabled(),
  );
});

it("keeps unsaved proxies untestable with a save-first hint", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  render(view(false));
  const button = screen.getByRole("button", { name: "Test" });
  expect(button).toBeDisabled();
  await userEvent.hover(button.parentElement!);
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "Save this proxy before testing.",
  );
  expect(fetch).not.toHaveBeenCalled();
});

it("reports a diagnostic failure next to the proxy and allows retry", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(Response.json({ ...exit, error: "timeout" }))
    .mockResolvedValueOnce(Response.json(exit));
  vi.stubGlobal("fetch", fetch);
  render(view());
  await userEvent.click(screen.getByRole("button", { name: "Test" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Test timed out after 15 seconds",
  );
  expect(screen.queryByText(/Exit IP:/)).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Test" }));
  expect(await screen.findByRole("status")).toHaveTextContent("192.0.2.1");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("cancels a test and discards its result when its configuration changes", async () => {
  let finish!: (value: Response) => void;
  const fetch = vi.fn(
    (_url: string, _init: RequestInit) =>
      new Promise<Response>((resolve) => {
        finish = resolve;
      }),
  );
  vi.stubGlobal("fetch", fetch);
  const { rerender } = render(view());
  await userEvent.click(screen.getByRole("button", { name: "Test" }));
  const signal = fetch.mock.calls[0][1].signal!;
  rerender(view(false));
  expect(signal.aborted).toBe(true);
  finish(Response.json(exit));
  await waitFor(() =>
    expect(screen.queryByRole("status")).not.toBeInTheDocument(),
  );
  expect(screen.queryByText(/192.0.2.1/)).not.toBeInTheDocument();
});
