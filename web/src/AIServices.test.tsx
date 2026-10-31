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
function reads(view = aiServicesFixture()) {
  const fetch = vi.fn(async (path: string, _init?: RequestInit) =>
    Response.json(path.endsWith("/services") ? view : { supported: true }),
  );
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

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
  let editor = screen.getByRole("region", { name: "Edit proxy" });
  await fill(
    within(editor).getByRole("textbox", { name: "ID (name)" }),
    "Home",
  );
  await fill(
    within(editor).getByLabelText("AI proxy address", { selector: "input" }),
    "http://localhost:7890",
  );
  await userEvent.click(screen.getByRole("button", { name: "Add proxy" }));
  editor = screen.getByRole("region", { name: "Edit proxy" });
  await choose(editor, "Type", "Trojan");
  await fill(
    within(editor).getByRole("textbox", { name: "ID (name)" }),
    "Travel",
  );
  await fill(
    within(editor).getByRole("textbox", { name: "Server address" }),
    "proxy.example",
  );
  await fill(
    within(editor).getByLabelText("Password", { selector: "input" }),
    "synthetic-password",
  );
  for (const [name, proxy] of [
    ["Primary", "Home"],
    ["Backup", "Travel"],
  ]) {
    await userEvent.click(screen.getByRole("button", { name: "Add account" }));
    editor = screen.getByRole("region", { name: "Edit AI account" });
    await fill(
      within(editor).getByRole("textbox", { name: "ID (name)" }),
      name,
    );
    await choose(editor, "Type", "OpenAI API");
    await choose(editor, "Proxy", proxy);
    await fill(
      within(editor).getByLabelText("API Key", { selector: "input" }),
      `synthetic-${name}`,
    );
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
  const ocr = await screen.findByRole("region", { name: "OCR" });
  expect(fetch.mock.calls.some(([path]) => path.includes("/models"))).toBe(
    false,
  );
  await choose(ocr, "AI account", "Second");
  const model = within(ocr).getByRole("combobox", { name: "Model" });
  await userEvent.click(model);
  expect(await screen.findByText("Loading models…")).toBeInTheDocument();
  expect(model).toBeEnabled();
  release(Response.json([{ id: "second-model", name: "Second model" }]));
  await userEvent.clear(model);
  await fill(model, "second-model");
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
    name: "Speech to text",
  });
  expect(
    within(audio).queryByRole("combobox", { name: "Reasoning effort" }),
  ).not.toBeInTheDocument();
  unmount();
  client.clear();
});

it("keeps secrets hidden and clears the dirty state when a secret edit is undone", async () => {
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
  expect(key).toHaveAttribute("type", "password");
  await fill(key, "temporary");
  await waitFor(() => expect(changed.mock.lastCall?.[0]).not.toBeNull());
  await userEvent.clear(key);
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  await choose(editor, "Type", "ChatGPT");
  await choose(editor, "Type", "OpenAI API");
  await waitFor(() => expect(changed.mock.lastCall?.[0]).toBeNull());
  expect(within(row).getByRole("button", { name: "Disconnect" })).toBeEnabled();
  expect(
    within(editor).getByLabelText("API Key", { selector: "input" }),
  ).toHaveAttribute("placeholder", "Configured; leave empty to keep");
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
  await fill(
    within(editor).getByRole("textbox", { name: "ID (name)" }),
    " updated",
  );
  expect(clear).toHaveBeenCalledWith("ai_services.accounts.0.id");
  unmount();
  client.clear();
});

it("opens ChatGPT during the click gesture and authenticates the selected account", async () => {
  let fixture = aiServicesFixture({
    accounts: [
      {
        number: 7,
        id: "ChatGPT home",
        type: "chatgpt",
        proxy: 0,
        connected: false,
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
        fixture = { ...fixture, pending_account: 7 };
        return Response.json({ url: "https://auth.openai.com/synthetic" });
      }
      return Response.json(
        path.endsWith("/services") ? fixture : { supported: true },
      );
    }),
  );
  const { client, unmount } = mount(<AIConnection onDraftChange={vi.fn()} />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Continue with ChatGPT" }),
  );
  await waitFor(() =>
    expect(replace).toHaveBeenCalledWith("https://auth.openai.com/synthetic"),
  );
  expect(order).toEqual(["open", "/api/v1/ai/accounts/7/connect"]);
  expect(login.opener).toBeNull();
  expect(
    await screen.findByRole("link", { name: "Open ChatGPT sign-in" }),
  ).toHaveAttribute("target", "_blank");
  unmount();
  client.clear();
});
