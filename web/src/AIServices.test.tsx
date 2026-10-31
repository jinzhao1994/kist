import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AIConnection } from "./AIServices";
import { aiServicesFixture } from "./aiServices.fixture";
import { SettingsErrorsContext } from "./SettingsFields";
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
  vi.restoreAllMocks();
  localStorage.clear();
});
function mount(content: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>{content}</I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { client, ...view };
}
async function choose(region: HTMLElement, name: string, option: string) {
  await userEvent.click(within(region).getByRole("combobox", { name }));
  await userEvent.click(screen.getByRole("option", { name: option }));
}
async function fill(input: HTMLElement, value: string) {
  const user = userEvent.setup();
  await user.click(input);
  await user.paste(value);
}

it("rejects blank names and invalid addresses before adding or completing a proxy without a request", async () => {
  const fetch = reads(aiServicesFixture({ proxies: [] }));
  const { client, unmount } = mount(<AIConnection />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Add proxy" }),
  );
  const editor = screen.getByRole("region", { name: "Add proxy" });
  await userEvent.click(
    within(editor).getByRole("button", { name: "Add proxy" }),
  );
  expect(within(editor).getByText("This value is required.")).toBeVisible();
  expect(
    screen.queryByRole("table", { name: "Proxies" }),
  ).not.toBeInTheDocument();
  await fill(within(editor).getByRole("textbox", { name: "Name" }), "   ");
  await userEvent.click(
    within(editor).getByRole("button", { name: "Add proxy" }),
  );
  expect(within(editor).getByText("This value is required.")).toBeVisible();
  const name = within(editor).getByRole("textbox", { name: "Name" });
  await userEvent.clear(name);
  await fill(name, "123");
  const address = within(editor).getByRole("textbox", {
    name: "Proxy address",
  });
  await fill(address, "456");
  await userEvent.click(
    within(editor).getByRole("button", { name: "Add proxy" }),
  );
  expect(address).toHaveAttribute("aria-invalid", "true");
  expect(
    within(editor).getByText("Use a valid HTTP(S) or SOCKS proxy URL."),
  ).toBeVisible();
  expect(
    screen.queryByRole("table", { name: "Proxies" }),
  ).not.toBeInTheDocument();
  await userEvent.clear(address);
  await fill(address, "http://127.0.0.1:7890");
  await userEvent.click(
    within(editor).getByRole("button", { name: "Add proxy" }),
  );
  const table = screen.getByRole("table", { name: "Proxies" });
  expect(within(table).getByText("Pending save")).toBeVisible();
  expect(within(table).getByText("http://127.0.0.1:7890/")).toBeVisible();
  await userEvent.click(within(table).getByRole("button", { name: "Edit" }));
  const editing = screen.getByRole("region", { name: "Edit proxy" });
  const editedAddress = within(editing).getByRole("textbox", {
    name: "Proxy address",
  });
  await userEvent.clear(editedAddress);
  await fill(editedAddress, "456");
  await userEvent.click(within(editing).getByRole("button", { name: "Done" }));
  expect(editedAddress).toHaveAttribute("aria-invalid", "true");
  await userEvent.click(
    within(editing).getByRole("button", { name: "Cancel" }),
  );
  expect(within(table).getByText("http://127.0.0.1:7890/")).toBeVisible();
  expect(
    fetch.mock.calls.every(
      ([, init]) => !(init as RequestInit | undefined)?.method,
    ),
  ).toBe(true);
  unmount();
  client.clear();
});

it("keeps creation above the table without a placeholder row and can cancel it", async () => {
  reads();
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const table = await screen.findByRole("table", { name: "Accounts" });
  const before = within(table).getAllByRole("row").length;
  await userEvent.click(screen.getByRole("button", { name: "Add account" }));
  const form = screen.getByRole("region", { name: "Add account" });
  expect(within(form).getByRole("textbox", { name: "Name" })).toHaveFocus();
  expect(form.closest("table")).toBeNull();
  expect(
    form.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(within(table).getAllByRole("row")).toHaveLength(before);
  await fill(
    within(form).getByRole("textbox", { name: "Name" }),
    "Unsaved account",
  );
  await userEvent.click(within(form).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(
    screen.queryByRole("region", { name: "Add account" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Add account" })).toHaveFocus();
  unmount();
  client.clear();
});

it("edits one table row and cancels newly entered secrets without retaining hidden fields", async () => {
  reads(
    aiServicesFixture({
      proxies: [
        {
          number: 1,
          id: "Home proxy",
          type: "http",
          url: "http://proxy.example:7890",
          host: "",
          port: 0,
          server_name: "",
          secret_configured: true,
        },
      ],
    }),
  );
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const accounts = await screen.findByRole("table", { name: "Accounts" });
  await userEvent.click(within(accounts).getByRole("button", { name: "Edit" }));
  const account = screen.getByRole("region", { name: "Edit AI account" });
  expect(account.closest("tr")?.closest("table")).toBe(accounts);
  expect(
    within(accounts).queryByRole("rowheader", { name: "Test account" }),
  ).not.toBeInTheDocument();
  await fill(
    within(account).getByLabelText("API Key", { selector: "input" }),
    "temporary-secret",
  );
  await userEvent.click(
    within(account).getByRole("button", { name: "Cancel" }),
  );
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(
    within(accounts).getByRole("rowheader", { name: "Test account" }),
  ).toBeVisible();
  const proxies = screen.getByRole("table", { name: "Proxies" });
  await userEvent.click(within(proxies).getByRole("button", { name: "Edit" }));
  const proxy = screen.getByRole("region", { name: "Edit proxy" });
  expect(proxy.closest("tr")?.closest("table")).toBe(proxies);
  await choose(proxy, "Type", "Trojan");
  await fill(
    within(proxy).getByLabelText("Password", { selector: "input" }),
    "temporary-password",
  );
  await userEvent.click(within(proxy).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(within(proxies).getByRole("cell", { name: "HTTP" })).toBeVisible();
  unmount();
  client.clear();
});
function reads(view = aiServicesFixture()) {
  const fetch = vi.fn(async (path: string, _init?: RequestInit) =>
    Response.json(
      path.endsWith("/services")
        ? view
        : path.includes("/models")
          ? []
          : { supported: true },
    ),
  );
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

it("tests a saved account explicitly and displays only the catalog summary", async () => {
  const models = Array.from(
    { length: 130 },
    (_, index) => `synthetic-model-${index}`,
  );
  let finish!: (value: Response) => void;
  const fetch = vi.fn((path: string, init?: RequestInit) =>
    path.endsWith("/accounts/1/test")
      ? new Promise<Response>((resolve) => {
          finish = resolve;
        })
      : Promise.resolve(
          Response.json(
            path.endsWith("/services")
              ? aiServicesFixture()
              : { supported: true },
          ),
        ),
  );
  vi.stubGlobal("fetch", fetch);
  const { client, unmount } = mount(<AIConnection />);
  const row = (
    await screen.findByRole("rowheader", { name: "Test account" })
  ).closest("tr")!;
  expect(
    within(row)
      .getAllByRole("button")
      .map((button) => button.textContent),
  ).toEqual(["Test", "Edit", "Delete"]);
  const test = within(row).getByRole("button", { name: "Test" });
  expect(test).toBeEnabled();
  expect(
    fetch.mock.calls.filter(([, init]) => init?.method === "POST"),
  ).toHaveLength(0);
  await userEvent.click(test);
  expect(within(row).getByRole("button", { name: "Testing…" })).toBeDisabled();
  expect(fetch).toHaveBeenCalledWith(
    "/api/v1/ai/accounts/1/test",
    expect.objectContaining({ method: "POST", body: "{}" }),
  );
  finish(
    Response.json({
      models,
      elapsed_ms: 120,
      error: "",
    }),
  );
  await waitFor(() =>
    expect(within(row).getByRole("status")).toHaveTextContent(
      "Test passed · 130 models · 120 ms",
    ),
  );
  expect(within(row).queryByText(/synthetic-model-/)).not.toBeInTheDocument();
  expect(within(row).getByText("Configured")).toBeVisible();
  await userEvent.click(within(row).getByRole("button", { name: "Edit" }));
  const editor = screen.getByRole("region", { name: "Edit AI account" });
  await fill(within(editor).getByRole("textbox", { name: "Name" }), "Changed");
  await userEvent.click(within(editor).getByRole("button", { name: "Done" }));
  expect(screen.getByRole("button", { name: "Test" })).toBeDisabled();
  expect(within(row).queryByRole("status")).not.toBeInTheDocument();
  unmount();
  client.clear();
});

it("reports account test permission failures without disconnecting the account", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      Response.json(
        path.endsWith("/accounts/1/test")
          ? { models: null, elapsed_ms: 0, error: "permission" }
          : path.endsWith("/services")
            ? aiServicesFixture()
            : { supported: true },
      ),
    ),
  );
  const { client, unmount } = mount(<AIConnection />);
  const row = (
    await screen.findByRole("rowheader", { name: "Test account" })
  ).closest("tr")!;
  await userEvent.click(within(row).getByRole("button", { name: "Test" }));
  expect(await within(row).findByRole("alert")).toHaveTextContent(
    "Model list access denied",
  );
  expect(within(row).getByRole("button", { name: "Test" })).toBeEnabled();
  expect(within(row).getByText("Configured")).toBeVisible();
  expect(
    within(row).queryByRole("button", { name: "Disconnect" }),
  ).not.toBeInTheDocument();
  unmount();
  client.clear();
});

// Multi-section user flows retain a bounded budget on slower CI workers.
it("edits multiple same-type accounts and distinct proxies in one unsaved draft", async () => {
  const fixture = aiServicesFixture({ proxies: [], accounts: [] });
  for (const feature of Object.values(fixture.features)) {
    feature.account = 0;
    feature.model = "";
  }
  const fetch = reads(fixture);
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  expect(await screen.findByText("No proxies")).toBeInTheDocument();
  expect(
    screen.queryByRole("table", { name: "Proxies" }),
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Add proxy" }));
  let editor = screen.getByRole("region", { name: "Add proxy" });
  expect(within(editor).getByRole("textbox", { name: "Name" })).toHaveAttribute(
    "aria-required",
    "true",
  );
  expect(
    within(editor).getByRole("textbox", { name: "Proxy address" }),
  ).toHaveAttribute("type", "text");
  expect(editor.querySelector('input[type="password"]')).toBeNull();
  await fill(within(editor).getByRole("textbox", { name: "Name" }), "Home");
  await fill(
    within(editor).getByLabelText("Proxy address", { selector: "input" }),
    "http://localhost:7890",
  );
  await userEvent.click(screen.getByRole("button", { name: "Add proxy" }));
  await userEvent.click(screen.getByRole("button", { name: "Add proxy" }));
  editor = screen.getByRole("region", { name: "Add proxy" });
  await choose(editor, "Type", "Trojan");
  await fill(within(editor).getByRole("textbox", { name: "Name" }), "Travel");
  await fill(
    within(editor).getByRole("textbox", { name: "Server address" }),
    "proxy.example",
  );
  await fill(
    within(editor).getByLabelText("Password", { selector: "input" }),
    "synthetic-password",
  );
  await userEvent.click(screen.getByRole("button", { name: "Add proxy" }));
  for (const [name, proxy] of [
    ["Primary", "Home"],
    ["Backup", "Travel"],
  ]) {
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    editor = screen.getByRole("region", { name: "Add account" });
    await fill(within(editor).getByRole("textbox", { name: "Name" }), name);
    await choose(editor, "Type", "OpenAI API");
    await choose(editor, "Proxy", proxy);
    await fill(
      within(editor).getByLabelText("API Key", { selector: "input" }),
      `synthetic-${name}`,
    );
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
  }
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.accounts).toHaveLength(2),
  );
  const draft = changed.mock.lastCall![0].value;
  expect(
    draft.accounts.map((account: { type: string }) => account.type),
  ).toEqual(["openai", "openai"]);
  expect(
    draft.accounts.map((account: { proxy: number }) => account.proxy),
  ).toEqual(draft.proxies.map((proxy: { number: number }) => proxy.number));
  expect(draft.proxies[0]).toMatchObject({ type: "http", port: 0 });
  expect(draft.proxies[1]).toMatchObject({ type: "trojan", port: 443 });
  expect(
    fetch.mock.calls.every(
      ([, init]) => !(init as RequestInit | undefined)?.method,
    ),
  ).toBe(true);
  unmount();
  client.clear();
}, 15_000);

it("loads a selected account catalog on focus and preserves independent feature parameters", async () => {
  const fixture = aiServicesFixture();
  fixture.accounts!.push({
    number: 2,
    id: "Second",
    type: "openai",
    proxy: 0,
    connected: true,
    email: "",
  });
  let release!: (value: Response) => void;
  const fetch = vi.fn(async (path: string) =>
    path.includes("/models")
      ? new Promise<Response>((resolve) => {
          release = resolve;
        })
      : Response.json(
          path.endsWith("/services") ? fixture : { supported: true },
        ),
  );
  vi.stubGlobal("fetch", fetch);
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const ocr = await screen.findByRole("region", {
    name: "Text recognition (OCR)",
  });
  expect(fetch.mock.calls.some(([path]) => path.includes("/models"))).toBe(
    false,
  );
  await choose(ocr, "AI account", "Second");
  const model = within(ocr).getByRole("combobox", { name: "Model" });
  await userEvent.click(model);
  expect(await screen.findByText("Loading models…")).toBeInTheDocument();
  expect(model).toBeEnabled();
  expect(
    screen.queryByRole("textbox", { name: "Search options" }),
  ).not.toBeInTheDocument();
  expect(model).toHaveAttribute("aria-expanded", "false");
  release(
    Response.json([
      { id: "second-model", name: "Second model" },
      { id: "other-model", name: "Other model" },
    ]),
  );
  const search = await screen.findByRole("textbox", { name: "Search options" });
  expect(search).toHaveValue("");
  await waitFor(() => expect(search).toHaveFocus());
  expect(
    await screen.findByRole("option", { name: "other-model · Other model" }),
  ).toBeVisible();
  await userEvent.click(
    await screen.findByRole("option", { name: "second-model · Second model" }),
  );
  await choose(ocr, "Reasoning effort", "high");
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.features.ocr.model).toBe(
      "second-model",
    ),
  );
  const draft = changed.mock.lastCall![0].value;
  expect(draft.features.ocr).toMatchObject({
    account: 2,
    model: "second-model",
    reasoning_effort: "high",
  });
  expect(draft.features.desc).toEqual(fixture.features.desc);
  expect(
    fetch.mock.calls
      .filter(([path]) => path.includes("/models"))
      .map(([path]) => path),
  ).toEqual(["/api/v1/ai/accounts/2/models"]);
  const audio = screen.getByRole("region", {
    name: "Speech recognition (STT)",
  });
  expect(
    within(audio).queryByRole("combobox", { name: "Reasoning effort" }),
  ).not.toBeInTheDocument();
  unmount();
  client.clear();
});

it("keeps secrets hidden and clears the dirty state when a secret edit is undone", async () => {
  vi.stubGlobal("CSS", { ...globalThis.CSS, supports: vi.fn(() => true) });
  const fixture = aiServicesFixture();
  fixture.proxies = [
    {
      number: 1,
      id: "Private proxy",
      type: "http",
      url: "http://proxy.example:7890",
      host: "",
      port: 0,
      server_name: "",
      secret_configured: true,
    },
  ];
  reads(fixture);
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const row = (
    await screen.findByRole("rowheader", { name: "Test account" })
  ).closest("tr")!;
  await userEvent.click(within(row).getByRole("button", { name: "Edit" }));
  const editor = screen.getByRole("region", { name: "Edit AI account" });
  const key = within(editor).getByLabelText("API Key", { selector: "input" });
  expect(
    within(editor).getByLabelText("API Key", { selector: "input" }),
  ).toHaveAttribute("placeholder", "Configured; leave empty to keep");
  expect(key).toHaveAttribute("type", "text");
  expect(key).toHaveAttribute("autocomplete", "off");
  await fill(key, "temporary");
  await waitFor(() => expect(changed.mock.lastCall?.[0]).not.toBeNull());
  await userEvent.clear(key);
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  await choose(editor, "Type", "ChatGPT");
  await choose(editor, "Type", "OpenAI API");
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(
    within(editor).getByLabelText("API Key", { selector: "input" }),
  ).toHaveAttribute("placeholder", "Configured; leave empty to keep");
  await userEvent.click(within(editor).getByRole("button", { name: "Done" }));
  expect(within(row).getByText("Configured")).toBeVisible();
  expect(
    within(row).queryByRole("button", { name: "Disconnect" }),
  ).not.toBeInTheDocument();
  const proxyRow = screen
    .getByRole("rowheader", { name: "Private proxy" })
    .closest("tr")!;
  await userEvent.click(within(proxyRow).getByRole("button", { name: "Edit" }));
  const proxy = screen.getByRole("region", { name: "Edit proxy" });
  const clear = within(proxy).getByRole("checkbox", {
    name: "Remove saved proxy authentication",
  });
  await userEvent.click(clear);
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.proxies[0].clear_secret).toBe(
      true,
    ),
  );
  await userEvent.click(clear);
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  await choose(proxy, "Type", "Trojan");
  expect(
    within(proxy).queryByRole("checkbox", {
      name: "Remove saved proxy authentication",
    }),
  ).not.toBeInTheDocument();
  await choose(proxy, "Type", "HTTP");
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(
    within(proxy).getByRole("checkbox", {
      name: "Remove saved proxy authentication",
    }),
  ).not.toBeChecked();
  unmount();
  client.clear();
});

it("opens field errors in the corresponding editor and lets the server own validation", async () => {
  reads();
  const clear = vi.fn();
  const { client, unmount } = mount(
    <SettingsErrorsContext
      value={{
        errors: {
          "ai_services.accounts.0.id": {
            field: "ai_services.accounts.0.id",
            code: "unique_id",
            message: "Duplicate ID",
          },
        },
        clear,
      }}
    >
      <AIConnection onDraftChange={vi.fn()} />
    </SettingsErrorsContext>,
  );
  const editor = await screen.findByRole("region", { name: "Edit AI account" });
  expect(within(editor).getByText("Enter a unique name.")).toBeInTheDocument();
  await fill(within(editor).getByRole("textbox", { name: "Name" }), " updated");
  expect(clear).toHaveBeenCalledWith("ai_services.accounts.0.id");
  unmount();
  client.clear();
});

it.each([false, true])(
  "opens ChatGPT during the click gesture and authenticates the selected account (configured=%s)",
  async (configured) => {
    let fixture = aiServicesFixture({
      accounts: [
        {
          number: 7,
          id: "ChatGPT home",
          type: "chatgpt",
          proxy: 0,
          connected: configured,
          email: "",
        },
      ],
    });
    const order: string[] = [];
    const replace = vi.fn();
    const login = {
      opener: {},
      closed: false,
      location: { replace },
      close: vi.fn(),
    };
    vi.spyOn(window, "open").mockImplementation(() => {
      order.push("open");
      return login as unknown as Window;
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(async (path: string, init?: RequestInit) => {
        if (init?.method === "POST") {
          order.push(path);
          return Response.json({
            id: "synthetic-login",
            url: "https://auth.openai.com/synthetic",
          });
        }
        if (path.endsWith("/sign-ins/synthetic-login"))
          return Response.json({
            id: "synthetic-login",
            pending: true,
            connected: false,
            email: "",
            error: "",
          });
        return Response.json(
          path.endsWith("/services") ? fixture : { supported: true },
        );
      }),
    );
    const { client, unmount } = mount(<AIConnection onDraftChange={vi.fn()} />);
    const signIn = await screen.findByRole("button", {
      name: configured ? "Sign in again" : "Sign in",
    });
    const row = signIn.closest("tr")!;
    expect(
      within(row).getByText(configured ? "Configured" : "Not configured"),
    ).toBeVisible();
    const test = within(row).getByRole("button", { name: "Test" });
    if (configured) expect(test).toBeEnabled();
    else expect(test).toBeDisabled();
    expect(
      within(row)
        .getAllByRole("button")
        .slice(-3)
        .map((button) => button.textContent),
    ).toEqual(["Test", "Edit", "Delete"]);
    if (configured) {
      const disconnect = within(row).getByRole("button", {
        name: "Disconnect",
      });
      expect(
        disconnect.compareDocumentPosition(test) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
    expect(
      signIn.compareDocumentPosition(test) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    await userEvent.click(signIn);
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith("https://auth.openai.com/synthetic"),
    );
    expect(order).toEqual(["open", "/api/v1/ai/sign-ins"]);
    expect(login.opener).toBeNull();
    expect(
      await screen.findByRole("link", { name: "Open ChatGPT sign-in" }),
    ).toHaveAttribute("target", "_blank");
    unmount();
    client.clear();
  },
);

it("signs in a new account inside the creation form and leaves only a reference for unified save", async () => {
  const fixture = aiServicesFixture({ accounts: [], proxies: [] });
  const requests: { path: string; init?: RequestInit }[] = [];
  const changed = vi.fn();
  vi.spyOn(window, "open").mockReturnValue(null);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      requests.push({ path, init });
      if (path.endsWith("/sign-ins") && init?.method === "POST")
        return Response.json({
          id: "new-draft",
          url: "https://auth.openai.com/synthetic",
          pending: true,
          connected: false,
          email: "",
          error: "",
        });
      if (path.endsWith("/sign-ins/new-draft"))
        return Response.json({
          id: "new-draft",
          pending: false,
          connected: true,
          email: "synthetic@example.test",
          error: "",
        });
      return Response.json(
        path.endsWith("/services") ? fixture : { supported: true },
      );
    }),
  );
  const { client, unmount, rerender } = mount(
    <AIConnection onDraftChange={changed} />,
  );
  await userEvent.click(
    await screen.findByRole("button", { name: "Add account" }),
  );
  const form = screen.getByRole("region", { name: "Add account" });
  await userEvent.click(
    within(form).getByRole("button", { name: "Sign in and add" }),
  );
  expect(within(form).getByText("This value is required.")).toBeVisible();
  expect(requests.filter((r) => r.init?.method === "POST")).toHaveLength(0);
  await fill(
    within(form).getByRole("textbox", { name: "Name" }),
    "New ChatGPT",
  );
  await userEvent.click(
    within(form).getByRole("button", { name: "Sign in and add" }),
  );
  const row = (
    await screen.findByRole("rowheader", { name: "New ChatGPT" })
  ).closest("tr")!;
  expect(
    screen.queryByRole("region", { name: "Add account" }),
  ).not.toBeInTheDocument();
  expect(within(row).getByRole("status")).toHaveTextContent(
    "Signed in; save the page",
  );
  const request = JSON.parse(
    String(requests.find((r) => r.init?.method === "POST")!.init!.body),
  );
  expect(request.number).toBeLessThan(0);
  expect(request.settings.accounts[0].id).toBe("New ChatGPT");
  expect(changed.mock.lastCall?.[0]?.value.accounts[0].login_draft).toBe(
    "new-draft",
  );
  expect(requests.some((r) => r.init?.method === "PUT")).toBe(false);
  expect(within(row).getByText("Configured")).toBeVisible();
  expect(within(row).getByRole("button", { name: "Test" })).toBeDisabled();
  rerender(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <AIConnection reloadVersion={1} onDraftChange={changed} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  await waitFor(() =>
    expect(
      requests.some(
        (r) =>
          r.path.endsWith("/sign-ins/new-draft") && r.init?.method === "DELETE",
      ),
    ).toBe(true),
  );
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  unmount();
  client.clear();
});

it("cleans up an authorization response that arrives after leaving the settings page", async () => {
  let finish!: (value: Response) => void;
  const requests: { path: string; method?: string }[] = [];
  const close = vi.fn();
  vi.spyOn(window, "open").mockReturnValue({
    opener: null,
    closed: false,
    close,
    location: { replace: vi.fn() },
  } as unknown as Window);
  const fixture = aiServicesFixture({
    accounts: [
      {
        number: 7,
        id: "ChatGPT",
        type: "chatgpt",
        proxy: 0,
        connected: false,
        email: "",
      },
    ],
  });
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      requests.push({ path, method: init?.method });
      if (path.endsWith("/sign-ins") && init?.method === "POST")
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      return Promise.resolve(
        Response.json(
          path.endsWith("/services") ? fixture : { supported: true },
        ),
      );
    }),
  );
  const { client, unmount } = mount(<AIConnection />);
  await userEvent.click(await screen.findByRole("button", { name: "Sign in" }));
  unmount();
  finish(
    Response.json({
      id: "abandoned-draft",
      url: "https://auth.openai.com/synthetic",
    }),
  );
  await waitFor(() =>
    expect(requests).toContainEqual({
      path: "/api/v1/ai/sign-ins/abandoned-draft",
      method: "DELETE",
    }),
  );
  expect(close).toHaveBeenCalled();
  client.clear();
});

it("offers only file transcription models and requests a capability-filtered catalog", async () => {
  const fixture = aiServicesFixture();
  const fetch = vi.fn(async (path: string) =>
    Response.json(
      path.includes("/models")
        ? [
            { id: "gpt-transcribe", name: "gpt-transcribe" },
            { id: "gpt-4o-mini-transcribe", name: "gpt-4o-mini-transcribe" },
            { id: "gpt-6-luna", name: "gpt-6-luna" },
            { id: "gpt-live-transcribe", name: "gpt-live-transcribe" },
            {
              id: "gpt-4o-transcribe-diarize",
              name: "gpt-4o-transcribe-diarize",
            },
          ]
        : path.endsWith("/services")
          ? fixture
          : { supported: true },
    ),
  );
  vi.stubGlobal("fetch", fetch);
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const region = await screen.findByRole("region", {
    name: "Speech recognition (STT)",
  });
  const select = within(region).getByRole("combobox", { name: "Model" });
  expect(select).toHaveTextContent("gpt-transcribe");
  await userEvent.click(select);
  await waitFor(() =>
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/ai/accounts/1/models?feature=transcription",
      expect.anything(),
    ),
  );
  expect(
    await screen.findByRole("option", { name: "gpt-transcribe" }),
  ).toBeVisible();
  expect(
    screen.queryByRole("option", { name: "gpt-6-luna" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("option", { name: "gpt-live-transcribe" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("option", { name: "gpt-4o-transcribe-diarize" }),
  ).not.toBeInTheDocument();
  const search = screen.getByRole("textbox", { name: "Search options" });
  await fill(search, "unsupported-audio-model");
  expect(screen.getByText("No matching options")).toBeVisible();
  expect(
    screen.queryByRole("option", { name: /Use model ID/ }),
  ).not.toBeInTheDocument();
  await userEvent.clear(search);
  await userEvent.click(
    screen.getByRole("option", { name: "gpt-4o-mini-transcribe" }),
  );
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.features.transcription.model).toBe(
      "gpt-4o-mini-transcribe",
    ),
  );
  unmount();
  client.clear();
});

it("retains the creation form and keeps a failed sign-in out of the account list", async () => {
  const fixture = aiServicesFixture({ accounts: [] });
  vi.spyOn(window, "open").mockReturnValue(null);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) =>
      Response.json(
        path.endsWith("/sign-ins") && init?.method === "POST"
          ? { id: "failed-draft", url: "https://auth.openai.com/synthetic" }
          : path.endsWith("/sign-ins/failed-draft")
            ? {
                id: "failed-draft",
                pending: false,
                connected: false,
                email: "",
                error: "Synthetic failure",
              }
            : path.endsWith("/services")
              ? fixture
              : { supported: true },
      ),
    ),
  );
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Add account" }),
  );
  const form = screen.getByRole("region", { name: "Add account" });
  await fill(within(form).getByRole("textbox", { name: "Name" }), "Not added");
  await userEvent.click(
    within(form).getByRole("button", { name: "Sign in and add" }),
  );
  expect(await within(form).findByRole("alert")).toHaveTextContent(
    "Sign-in failed or expired",
  );
  expect(
    screen.queryByRole("rowheader", { name: "Not added" }),
  ).not.toBeInTheDocument();
  expect(changed.mock.lastCall?.[0]?.blocked).toBe(true);
  expect(
    within(form).getByRole("button", { name: "Sign in and add" }),
  ).toBeEnabled();
  unmount();
  client.clear();
});

it("keeps per-feature failure policy in the page draft and can restore the inherited default", async () => {
  const fetch = reads();
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const ocr = await screen.findByRole("region", {
    name: "Text recognition (OCR)",
  });
  const pause = within(ocr).getByRole("checkbox", {
    name: "Pause background processing on failure",
  });
  expect(pause).toBeChecked();
  for (const name of [
    "Orientation correction",
    "Text recognition (OCR)",
    "Content overview",
    "Information extraction",
    "Speech recognition (STT)",
  ]) {
    const region = screen.getByRole("region", { name });
    const failurePolicy = within(region).getByRole("checkbox", {
      name: "Pause background processing on failure",
    });
    const automatic = within(region)
      .getAllByRole("checkbox")
      .find((field) => field !== failurePolicy)!;
    const lastParameter = within(region).getAllByRole("combobox").at(-1)!;
    expect(
      lastParameter.compareDocumentPosition(automatic) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      automatic.compareDocumentPosition(failurePolicy) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  }
  await userEvent.click(pause);
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.features.ocr.pause_on_error).toBe(
      false,
    ),
  );
  expect(
    within(screen.getByRole("region", { name: "Content overview" })).getByRole(
      "checkbox",
      { name: "Pause background processing on failure" },
    ),
  ).toBeChecked();
  await userEvent.click(
    within(ocr).getByRole("button", { name: "Restore default" }),
  );
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(pause).toBeChecked();
  expect(fetch.mock.calls.every(([, init]) => !init?.method)).toBe(true);
  unmount();
  client.clear();
});

it("keeps model search separate from the chosen value and explicitly accepts custom IDs", async () => {
  const fetch = reads();
  const changed = vi.fn();
  const { client, unmount } = mount(<AIConnection onDraftChange={changed} />);
  const ocr = await screen.findByRole("region", {
    name: "Text recognition (OCR)",
  });
  const model = within(ocr).getByRole("combobox", { name: "Model" });
  expect(model).toHaveTextContent("gpt-6-luna");
  await userEvent.click(model);
  let search = await screen.findByRole("textbox", { name: "Search options" });
  expect(search).toHaveValue("");
  await fill(search, "private-vision-model");
  expect(changed.mock.lastCall?.[0]).toBeNull();
  await userEvent.keyboard("{Escape}");
  expect(model).toHaveTextContent("gpt-6-luna");
  await userEvent.click(model);
  search = screen.getByRole("textbox", { name: "Search options" });
  expect(search).toHaveValue("");
  await fill(search, "private-vision-model");
  await userEvent.click(
    screen.getByRole("option", { name: "Use model ID: private-vision-model" }),
  );
  await waitFor(() =>
    expect(changed.mock.lastCall?.[0]?.value.features.ocr.model).toBe(
      "private-vision-model",
    ),
  );
  expect(model).toHaveTextContent("private-vision-model");
  await userEvent.click(model);
  expect(screen.getByRole("textbox", { name: "Search options" })).toHaveValue(
    "",
  );
  expect(
    screen.getByRole("option", { name: "private-vision-model" }),
  ).toBeVisible();
  expect(fetch.mock.calls.every(([, init]) => !init?.method)).toBe(true);
  unmount();
  client.clear();
});

it.each(["blur", "escape", "failure"])(
  "does not open a delayed model list after %s",
  async (outcome) => {
    let finish!: (value: Response) => void;
    const fetch = vi.fn(async (path: string) =>
      path.includes("/models")
        ? new Promise<Response>((resolve) => {
            finish = resolve;
          })
        : Response.json(
            path.endsWith("/services")
              ? aiServicesFixture()
              : { supported: true },
          ),
    );
    vi.stubGlobal("fetch", fetch);
    const { client, unmount } = mount(<AIConnection />);
    const region = await screen.findByRole("region", {
      name: "Text recognition (OCR)",
    });
    const model = within(region).getByRole("combobox", { name: "Model" });
    await userEvent.click(model);
    await screen.findByText("Loading models…");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    const automatic = within(region).getByRole("checkbox", {
      name: "Automatically recognize new images",
    });
    if (outcome === "blur") await userEvent.click(automatic);
    if (outcome === "escape") await userEvent.keyboard("{Escape}");
    finish(
      outcome === "failure"
        ? Response.json(
            { code: "internal", message: "Catalog unavailable" },
            { status: 502 },
          )
        : Response.json([{ id: "another-model", name: "Another model" }]),
    );
    await waitFor(() =>
      expect(screen.queryByText("Loading models…")).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    if (outcome === "blur") expect(automatic).toHaveFocus();
    if (outcome === "failure") {
      expect(screen.getByText(/Catalog unavailable/)).toBeVisible();
      await userEvent.click(model);
      expect(
        await screen.findByRole("textbox", { name: "Search options" }),
      ).toBeVisible();
      expect(
        fetch.mock.calls.filter(([path]) => path.includes("/models")),
      ).toHaveLength(1);
    }
    unmount();
    client.clear();
  },
);
