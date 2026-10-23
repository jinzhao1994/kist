import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { InstanceSettings } from "./InstanceSettings";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";
let writes: any[];
let stale: boolean;
const initial = () => ({
  revision: "revision-one",
  mode: "client",
  id: "client",
  library_id: "library",
  credential_configured: true,
  connection: { server: "https://remote.example", allow_http: false },
  fields: [
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
      restart: true,
    },
    {
      key: "server.allowed_origins",
      kind: "list",
      value: [],
      default: [],
      explicit: false,
      restart: true,
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
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, options?: RequestInit) => {
      const data = initial();
      if (options?.method === "PUT") {
        const body = JSON.parse(options.body as string);
        writes.push(body);
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
afterEach(() => vi.unstubAllGlobals());
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
it("does not save on open; resetting removes the explicit value using the loaded revision", async () => {
  const user = userEvent.setup();
  mount();
  expect(await screen.findByRole("radio", { name: "Green" })).toBeChecked();
  expect(writes).toEqual([]);
  await user.click(
    screen.getAllByRole("button", { name: "Restore default" })[0],
  );
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
