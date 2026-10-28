import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StructuredFields } from "./StructuredFields";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

const data = JSON.stringify({
  version: 1,
  fields: [
    {
      key: "holder",
      label: "Holder",
      type: "object",
      source: "ai",
      children: [
        {
          key: "name",
          label: "Name",
          type: "text",
          value: "Example Person",
          source: "user",
        },
        {
          key: "secret",
          label: "Removed",
          type: "text",
          value: "Hidden",
          source: "user",
          deleted: true,
        },
      ],
    },
    {
      key: "number",
      label: "Number",
      type: "text",
      value: "001234",
      source: "ai",
    },
    {
      key: "issuer",
      label: "Issuer",
      type: "text",
      uncertain: true,
      source: "ai",
    },
  ],
});
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
function mount(value = data) {
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={new QueryClient()}>
      <I18nProvider>
        <ThemeProvider env="test">{children}</ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>
  );
  const props = {
    fileID: "file",
    blobKey: "sha256:synthetic",
    row: { key: "facts:data", value, source: "facts", type: "text" },
  };
  return { ...render(<StructuredFields {...props} />, { wrapper }), props };
}
it("displays grouped values, uncertainty and leaf ownership without raw JSON or deleted fields", () => {
  mount();
  expect(screen.getByRole("heading", { name: "Holder" })).toBeVisible();
  expect(screen.getByText("Example Person", { selector: "bdi" })).toBeVisible();
  expect(screen.getByText("001234", { selector: "bdi" })).toBeVisible();
  expect(screen.getByText("Unreadable")).toBeVisible();
  expect(screen.getByRole("img", { name: /Edited by user/ })).toBeVisible();
  expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Edit Holder" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Delete Holder" })).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Copy" }),
  ).not.toBeInTheDocument();
});
it("renders a single record as ordinary fields in source order", () => {
  const fields = [
    { key: "title", label: "Title", value: "Synthetic event" },
    { key: "name", label: "Name", value: "Example Person" },
    { key: "date", label: "Date", value: "2025-06" },
  ].map((field) => ({ ...field, type: "text", source: "ai" }));
  const view = mount(JSON.stringify({ version: 1, fields }));
  expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  expect(
    Array.from(
      view.container.querySelectorAll("dt bdi"),
      (node) => node.textContent,
    ),
  ).toEqual(["Title", "Name", "Date"]);
  expect(
    screen.queryByRole("button", { name: "Copy" }),
  ).not.toBeInTheDocument();
});
it("binds a leaf edit to its original whole-document snapshot across background refresh", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  const view = mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit Name" }));
  view.rerender(
    <StructuredFields
      {...view.props}
      row={{ ...view.props.row, value: data.replace("001234", "009999") }}
    />,
  );
  await user.clear(screen.getByRole("textbox"));
  await user.type(screen.getByRole("textbox"), "Correct Person{Enter}");
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  const [, init] = (fetch.mock.calls as unknown as [string, RequestInit][])[0];
  expect(JSON.parse(String(init.body))).toMatchObject({
    key: "facts:data",
    path: "/holder/name",
    before_value: data,
    before_source: "facts",
    value: "Correct Person",
  });
});
it("confirms subtree deletion with its stable path", async () => {
  const fetch = vi.fn(async () => Response.json({ op_id: 1 }));
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Delete Holder" }));
  expect(fetch).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Confirm deletion" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  const [, init] = (fetch.mock.calls as unknown as [string, RequestInit][])[0];
  expect(JSON.parse(String(init.body))).toMatchObject({
    key: "facts:data",
    path: "/holder",
    before_value: data,
    remove: true,
  });
});
it("keeps unsupported document formats out of the editable interface", () => {
  mount('{"version":999,"fields":[]}');
  expect(screen.getByRole("alert")).toBeVisible();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});
