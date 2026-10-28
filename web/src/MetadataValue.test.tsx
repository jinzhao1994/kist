import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MetadataValue } from "./MetadataValue";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

beforeEach(() => {
  Object.defineProperty(document, "fonts", {
    configurable: true,
    value: new EventTarget(),
  });
  localStorage.clear();
  sessionStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function mount(value = "Camera", source = "exif", key = "exif:Make") {
  const props = {
    fileID: "file",
    blobKey: "sha256:test",
    row: { key, value, source, type: "text" },
    label: "Value",
  };
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={new QueryClient()}>
      <I18nProvider>
        <ThemeProvider env="test">{children}</ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>
  );
  return render(<MetadataValue {...props}>{value}</MetadataValue>, { wrapper });
}
it("saves the raw before-state and does not submit cancellation or unchanged values", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit Value" }));
  await user.keyboard("{Enter}");
  expect(fetch).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Edit Value" }));
  await user.clear(screen.getByRole("textbox"));
  await user.type(screen.getByRole("textbox"), "Other{Escape}");
  expect(fetch).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Edit Value" }));
  await user.clear(screen.getByRole("textbox"));
  await user.type(screen.getByRole("textbox"), "Other{Enter}");
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  const [url, init] = (
    fetch.mock.calls as unknown as [string, RequestInit][]
  )[0];
  expect(url).toBe("/api/v1/files/file/metadata");
  expect(JSON.parse(String(init.body))).toMatchObject({
    key: "exif:Make",
    before_value: "Camera",
    before_source: "exif",
    before_type: "text",
    value: "Other",
    blob_key: "sha256:test",
  });
});
it("preserves multiline text and retries the exact operation after a lost response", async () => {
  const fetch = vi.fn(async () => {
    if (fetch.mock.calls.length === 1) throw new TypeError("lost");
    return Response.json({ op_id: 1 });
  });
  vi.stubGlobal("fetch", fetch);
  mount("First\nSecond", "user", "ocr:text");
  expect(screen.getByRole("img", { name: /Edited by user/ })).toBeVisible();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit Value" }));
  const input = screen.getByRole("textbox");
  expect(input).toHaveValue("First\nSecond");
  await user.clear(input);
  await user.type(input, "One{Enter}Two");
  expect(fetch).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByRole("alert");
  expect(input).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(fetch.mock.calls[1]).toEqual(fetch.mock.calls[0]);
  const [, init] = (fetch.mock.calls as unknown as [string, RequestInit][])[0];
  expect(JSON.parse(String(init.body)).value).toBe("One\nTwo");
});

it("requires a second click to delete and lets Escape or cancel dismiss without writing", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Delete Value" });
  await user.click(trigger);
  expect(
    await screen.findByRole("dialog", { name: "Delete “Value”?" }),
  ).toBeVisible();
  expect(fetch).not.toHaveBeenCalled();
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(trigger).toHaveFocus();
  await user.click(trigger);
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(fetch).not.toHaveBeenCalled();
  await user.click(trigger);
  await user.click(screen.getByRole("button", { name: "Confirm deletion" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  const [, init] = (fetch.mock.calls as unknown as [string, RequestInit][])[0];
  expect(JSON.parse(String(init.body))).toEqual({
    key: "exif:Make",
    before_value: "Camera",
    before_source: "exif",
    before_type: "text",
    blob_key: "sha256:test",
    value: "",
    remove: true,
  });
});

it("retries deletion with the same idempotency key after a lost response", async () => {
  const fetch = vi.fn(async () => {
    if (fetch.mock.calls.length === 1) throw new TypeError("lost");
    return Response.json({ op_id: 1 });
  });
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Delete Value" }));
  await user.click(screen.getByRole("button", { name: "Confirm deletion" }));
  await screen.findByRole("alert");
  expect(
    screen.getByRole("button", { name: "Confirm deletion" }),
  ).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(fetch.mock.calls[1]).toEqual(fetch.mock.calls[0]);
});
