import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { AIConnection, OCRAction, OCRText } from "./AI";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

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
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
function mount(content: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">{content}</ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return client;
}
function defaults(path: string) {
  if (path.endsWith("/ai/settings"))
    return Response.json({
      model: "gpt-6-luna",
      auto_ocr: false,
      proxy_url: "",
      revision: "one",
    });
  if (path.endsWith("/ai/ocr"))
    return Response.json({
      paused: false,
      scanning: false,
      queued: 0,
      failed: 0,
      running: "",
    });
  if (path.endsWith("/ai/models"))
    return Response.json([{ id: "gpt-6-luna", name: "Luna" }]);
  return undefined;
}
it("queues OCR with the saved model policy and reports request failures", async () => {
  const requests: RequestInit[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      const value = defaults(path);
      if (value) return value;
      if (path.endsWith("/ai"))
        return Response.json({ supported: true, connected: true });
      if (!init?.method) return Response.json({ state: "missing" });
      requests.push(init);
      return requests.length === 1
        ? Response.json(
            { code: "internal", message: "Queue unavailable" },
            { status: 502 },
          )
        : Response.json({ state: "queued" });
    }),
  );
  mount(<OCRAction fileID="file" />);
  const run = await screen.findByRole("button", { name: "Extract text (OCR)" });
  await waitFor(() => expect(run).toBeEnabled());
  await userEvent.click(run);
  await screen.findByText(/Queue unavailable/);
  await userEvent.click(run);
  await screen.findByText("Queued for OCR");
  expect(run).toBeDisabled();
  expect(JSON.parse(requests[0].body as string)).toEqual({ retry: false });
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
});
it("does not run OCR before a default model is saved", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path.endsWith("/ai/settings"))
        return Response.json({
          model: "",
          auto_ocr: false,
          proxy_url: "",
          revision: "one",
        });
      return (
        defaults(path) ||
        Response.json(
          path.endsWith("/ai")
            ? { supported: true, connected: true }
            : { state: "missing" },
        )
      );
    }),
  );
  mount(<OCRAction fileID="file" />);
  await screen.findByText(
    "Connect an account and save a default OCR model in AI settings first.",
  );
  expect(
    screen.getByRole("button", { name: "Extract text (OCR)" }),
  ).toBeDisabled();
});
it("shows model loading and saves shared options without triggering a backfill", async () => {
  let finish!: (response: Response) => void;
  let modelRequests = 0;
  const writes: { path: string; body: any }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === "PUT") {
        const body = JSON.parse(init.body as string);
        writes.push({ path, body });
        return Response.json({ ...body, revision: "two" });
      }
      if (path.endsWith("/ai/models") && modelRequests++ === 0)
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      if (path.endsWith("/ai/settings"))
        return Response.json({
          model: "",
          auto_ocr: false,
          proxy_url: "",
          revision: "one",
        });
      return (
        defaults(path) ||
        Response.json({
          supported: true,
          connected: true,
          pending: false,
          email: "reader@example.test",
        })
      );
    }),
  );
  mount(<AIConnection />);
  await screen.findByText("Loading models…");
  finish(Response.json([{ id: "gpt-6-luna", name: "Luna" }]));
  await userEvent.click(
    await screen.findByRole("button", { name: "Use GPT-6 Luna" }),
  );
  await userEvent.click(
    screen.getByRole("checkbox", {
      name: "Automatically recognize new images",
    }),
  );
  await userEvent.type(
    screen.getByLabelText("AI proxy address"),
    "http://127.0.0.1:7890",
  );
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toEqual({
    path: "/api/v1/ai/settings",
    body: {
      model: "gpt-6-luna",
      auto_ocr: true,
      proxy_url: "http://127.0.0.1:7890",
      revision: "one",
    },
  });
});
it("compacts OCR for display but copies the complete original text", async () => {
  const user = userEvent.setup();
  const copy = vi.spyOn(navigator.clipboard, "writeText");
  const text = "证件  名称\t详情\r\n \n\n姓名：张三\n".repeat(1000);
  mount(<OCRText text={text} />);
  const button = screen.getByRole("button", { name: "Copy recognized text" });
  expect(button.parentElement?.querySelector("bdi")?.textContent).toBe(
    Array(1000).fill("证件 名称 详情↵ 姓名：张三").join("↵ "),
  );
  await user.click(button);
  expect(copy).toHaveBeenCalledWith(text);
});
it("clears connection state and reports unconfirmed revocation", async () => {
  let connected = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const value = defaults(path);
      if (value) return value;
      if (path.endsWith("/disconnect")) {
        connected = false;
        return Response.json({ revoked: false });
      }
      return Response.json({
        supported: true,
        connected,
        email: "reader@example.test",
        pending: false,
        error: "",
      });
    }),
  );
  mount(<AIConnection />);
  await userEvent.click(
    await screen.findByRole("button", { name: "Disconnect" }),
  );
  await waitFor(() =>
    expect(screen.queryByText(/reader@example.test/)).not.toBeInTheDocument(),
  );
  expect(screen.getByText(/Local credentials cleared/)).toBeVisible();
});

it("opens ChatGPT from the initial click and detaches the new tab", async () => {
  const replace = vi.fn();
  const popup = {
    opener: window,
    closed: false,
    location: { replace },
    close: vi.fn(),
  };
  const open = vi
    .spyOn(window, "open")
    .mockReturnValue(popup as unknown as Window);
  let pending = false;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const value = defaults(path);
      if (value) return value;
      if (path.endsWith("/connect")) {
        pending = true;
        return Response.json({
          url: "https://auth.openai.com/authorize?state=test",
        });
      }
      return Response.json({
        supported: true,
        connected: false,
        pending,
        email: "",
        error: "",
      });
    }),
  );
  mount(<AIConnection />);
  const button = await screen.findByRole("button", {
    name: "Continue with ChatGPT",
  });
  await waitFor(() => expect(button).toBeEnabled());
  await userEvent.click(button);
  expect(open).toHaveBeenCalledWith("about:blank", "_blank");
  await waitFor(() =>
    expect(replace).toHaveBeenCalledWith(
      "https://auth.openai.com/authorize?state=test",
    ),
  );
  expect(popup.opener).toBeNull();
});

it("shows an explicit empty OCR result without a blank editor or copy action", () => {
  mount(<OCRText text="" />);
  expect(screen.getByText("No text was recognized.")).toBeVisible();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Copy recognized text" }),
  ).not.toBeInTheDocument();
});

it("allows model-list retry and explains an empty catalog", async () => {
  let modelRequests = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path.endsWith("/ai/models")) {
        modelRequests++;
        return modelRequests === 1
          ? Response.json(
              { code: "internal", message: "Catalog unavailable" },
              { status: 502 },
            )
          : Response.json([]);
      }
      return (
        defaults(path) || Response.json({ supported: true, connected: true })
      );
    }),
  );
  mount(<AIConnection />);
  await screen.findByText(/Catalog unavailable/);
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  await screen.findByText("No models are available for this account.");
  expect(modelRequests).toBe(2);
});
