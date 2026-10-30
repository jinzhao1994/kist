import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { InstanceSettings } from "./InstanceSettings";
import { hasSettingChange } from "./SettingsFields";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
vi.mock("@mantine/hooks", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@mantine/hooks")>()),
  useElementSize: () => ({ ref: () => {}, height: 50, width: 800 }),
}));
let writes: any[];
let stale: boolean;
let invalidFields: { field: string; code: string; message: string }[];
const initial = () => ({
  revision: "revision-one",
  mode: "client",
  id: "client",
  library_id: "library",
  credential_configured: true,
  connection: { server: "https://remote.example", allow_http: false },
  fields: [
    {
      key: "client.cache_max_age",
      kind: "text",
      value: "180d",
      default: "180d",
      explicit: false,
      restart: false,
    },
    {
      key: "appearance.primary_color",
      kind: "select",
      value: "green",
      default: "blue",
      explicit: true,
      restart: false,
    },
    {
      key: "server.max_upload_bytes",
      kind: "bytes",
      value: 32 * 2 ** 30,
      default: 32 * 2 ** 30,
      explicit: false,
      restart: false,
    },
    {
      key: "server.allowed_origins",
      kind: "list",
      value: [],
      default: [],
      explicit: false,
      restart: false,
    },
  ],
  effective: {},
  overrides: [],
  restart_required: [],
});
beforeEach(() => {
  Object.defineProperty(document, "fonts", {
    configurable: true,
    value: { addEventListener: vi.fn(), removeEventListener: vi.fn() },
  });
  localStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      timezone: "UTC",
      formatLocale: "en-US",
      direction: "ltr",
    }),
  );
  writes = [];
  stale = false;
  invalidFields = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, options?: RequestInit) => {
      if (_path.endsWith("/settings/credentials")) return new Response("[]");
      if (_path.endsWith("/settings/storage"))
        return new Response(
          JSON.stringify({
            originals_bytes: 1024,
            metadata_bytes: 512,
            previews_bytes: 256,
          }),
        );
      const data = initial();
      if (options?.method === "PUT") {
        const body = JSON.parse(options.body as string);
        writes.push(body);
        if (invalidFields.length)
          return Response.json(
            {
              code: "invalid_request",
              class: "invalid",
              message: "Invalid settings",
              fields: invalidFields,
            },
            { status: 400 },
          );
        if (stale)
          return new Response(
            JSON.stringify({
              message: "Settings changed; reload before saving.",
              code: "conflict",
              class: "conflict",
            }),
            { status: 409 },
          );
        data.revision = "revision-two";
        for (const field of data.fields)
          if (Object.hasOwn(body.changes, field.key)) {
            field.value = body.changes[field.key] ?? field.default;
            field.explicit = body.changes[field.key] !== null;
          }
      }
      return new Response(JSON.stringify(data), { status: 200 });
    }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
});
function mount() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <InstanceSettings />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
it("shows server field errors only after save and keeps them when the floating message is dismissed", async () => {
  const user = userEvent.setup();
  mount();
  const input = await screen.findByRole("textbox", {
    name: "Maximum cache age",
  });
  await user.clear(input);
  await user.type(input, "30da21");
  expect(input).not.toHaveAttribute("aria-invalid", "true");
  expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  invalidFields = [
    {
      field: "client.cache_max_age",
      code: "duration",
      message: "Invalid duration",
    },
  ];
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toHaveClass("floating-feedback");
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Settings were not saved. Check the values you entered.",
  );
  expect(input).toHaveAttribute("aria-invalid", "true");
  expect(input.closest(".setting-field")).toHaveTextContent(
    "Enter a positive duration",
  );
  expect(input).toHaveValue("30da21");
  await user.click(
    within(screen.getByRole("alert")).getByRole("button", { name: "Close" }),
  );
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(input).toHaveAttribute("aria-invalid", "true");
  await user.clear(input);
  await user.type(input, "30d");
  expect(input).not.toHaveAttribute("aria-invalid", "true");
  invalidFields = [];
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes).toHaveLength(2));
});
it("restores a fragment after settings load without scrolling again on edits", async () => {
  window.history.replaceState(null, "", "/settings#settings-access");
  const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
  mount();
  await waitFor(() => expect(scroll).toHaveBeenCalled());
  expect(scroll.mock.instances.at(-1)).toBe(
    document.getElementById("settings-access"),
  );
  scroll.mockClear();
  await userEvent.setup().click(screen.getByRole("radio", { name: "Orange" }));
  expect(scroll).not.toHaveBeenCalled();
  window.location.hash = "settings-service";
  await waitFor(() => expect(scroll).toHaveBeenCalled());
  expect(scroll.mock.instances.at(-1)).toBe(
    document.getElementById("settings-service"),
  );
});
it("does not save on open; resetting removes the explicit value using the loaded revision", async () => {
  const user = userEvent.setup();
  mount();
  expect(await screen.findByRole("radio", { name: "Green" })).toBeChecked();
  expect(writes).toEqual([]);
  const colorField = screen
    .getByRole("radio", { name: "Green" })
    .closest<HTMLElement>(".setting-field")!;
  const reset = within(colorField).getByRole("button", {
    name: "Restore default",
  });
  expect(reset.closest(".setting-field-heading")).toHaveTextContent(
    "Primary color",
  );
  expect(document.querySelectorAll(".setting-field-footer")).toHaveLength(0);
  await user.click(reset);
  expect(reset).not.toBeInTheDocument();
  expect(screen.getByRole("radio", { name: "Blue" })).toBeChecked();
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(writes).toEqual([
      {
        revision: "revision-one",
        changes: { "appearance.primary_color": null },
      },
    ]),
  );
});
it("keeps color choices keyboard accessible without saving the draft", async () => {
  const user = userEvent.setup();
  mount();
  const green = await screen.findByRole("radio", { name: "Green" });
  expect(green).toHaveAttribute("tabindex", "0");
  await user.click(green);
  await user.keyboard("{ArrowRight}");
  const yellow = screen.getByRole("radio", { name: "Yellow" });
  expect(yellow).toBeChecked();
  expect(yellow).toHaveFocus();
  expect(yellow).toHaveAttribute("tabindex", "0");
  expect(green).toHaveAttribute("tabindex", "-1");
  expect(writes).toEqual([]);
});
it("keeps edits on conflict and does not retry the write", async () => {
  stale = true;
  const user = userEvent.setup();
  mount();
  await user.click(await screen.findByRole("radio", { name: "Orange" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Settings changed",
  );
  expect(screen.getByRole("radio", { name: "Orange" })).toBeChecked();
  expect(writes).toHaveLength(1);
});
it("allows multi-line origins and keeps stored credentials out of inputs", async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByRole("heading", { name: "Connection" });
  expect(screen.getByLabelText(/^Sync credential/)).toHaveValue("");
  expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Service" })).toBeVisible();
  const origins = screen.getByRole("textbox", { name: "Allowed access URLs" });
  await user.type(origins, "https://one.example{Enter}https://two.example");
  expect(origins).toHaveValue("https://one.example\nhttps://two.example");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].changes).toEqual({
    "server.allowed_origins": ["https://one.example", "https://two.example"],
  });
});

it("displays exact capacity units and submits the user's size string", async () => {
  const user = userEvent.setup();
  mount();
  const input = await screen.findByRole("textbox", {
    name: "Maximum upload size",
  });
  expect(input).toHaveValue("32 GiB");
  await user.clear(input);
  await user.type(input, "32G");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].changes).toEqual({ "server.max_upload_bytes": "32G" });
});

it("shows an empty credential row without a table and can open and cancel creation", async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByText("No access credentials yet");
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Create credential" }));
  expect(
    screen.getByRole("textbox", { name: "Note (optional)" }),
  ).toBeVisible();
  expect(
    screen.queryByText("No access credentials yet"),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.getByText("No access credentials yet")).toBeVisible();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});

it("shows inherited state without a reset until a default field is explicitly edited", async () => {
  const user = userEvent.setup();
  mount();
  const input = await screen.findByRole("textbox", {
    name: "Maximum upload size",
  });
  const field = input.closest<HTMLElement>(".setting-field")!;
  expect(
    within(field).queryByRole("button", { name: "Restore default" }),
  ).not.toBeInTheDocument();
  expect(within(field).getByText("Following defaults")).toBeVisible();
  await user.clear(input);
  await user.type(input, "16 GiB");
  const reset = within(field).getByRole("button", { name: "Restore default" });
  expect(reset.closest(".setting-field-heading")).toBeInTheDocument();
  expect(reset).toBeEnabled();
  expect(within(field).getByText("Explicitly set")).toBeVisible();
  await user.clear(input);
  await user.type(input, "32 GiB");
  expect(input).toHaveValue("32 GiB");
  expect(reset).toBeEnabled();
  expect(within(field).getByText("Explicitly set")).toBeVisible();
  await user.click(reset);
  expect(input).toHaveValue("32 GiB");
  expect(
    within(field).queryByRole("button", { name: "Restore default" }),
  ).not.toBeInTheDocument();
  expect(within(field).getByText("Following defaults")).toBeVisible();
  expect(within(field).queryByText("Explicitly set")).not.toBeInTheDocument();
  expect(writes).toEqual([]);
});

it("compares saved meaning without conflating explicit and inherited defaults", () => {
  const bytes = { kind: "bytes", value: 32 * 2 ** 30, explicit: true };
  expect(hasSettingChange(bytes, "32 GiB")).toBe(false);
  expect(hasSettingChange(bytes, "34,359,738,368 B")).toBe(false);
  expect(hasSettingChange(bytes, "32 GB")).toBe(true);
  expect(hasSettingChange({ ...bytes, explicit: false }, "32 GiB")).toBe(true);
  expect(hasSettingChange({ ...bytes, explicit: false }, null)).toBe(false);
  expect(hasSettingChange(bytes, null)).toBe(true);
  expect(
    hasSettingChange(
      { kind: "list", value: ["https://example.test"], explicit: true },
      [" https://example.test ", ""],
    ),
  ).toBe(false);
});
