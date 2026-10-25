import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { AIConnection, OCRDialog, OCRText } from "./AI";
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
it("keeps a failed OCR dialog open and retries with the same request identity", async () => {
  const requests: RequestInit[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (path.endsWith("/ai"))
        return Response.json({ supported: true, connected: true });
      if (path.endsWith("/ai/models"))
        return Response.json([{ id: "gpt-6-luna", name: "Luna" }]);
      requests.push(init!);
      return requests.length === 1
        ? Response.json(
            { code: "internal", class: "internal", message: "OCR interrupted" },
            { status: 502 },
          )
        : Response.json({ op_id: "saved" });
    }),
  );
  const close = vi.fn();
  const client = mount(<OCRDialog fileID="file" onClose={close} />);
  const invalidation = vi.spyOn(client, "invalidateQueries");
  const run = await screen.findByRole("button", { name: "Extract text (OCR)" });
  await waitFor(() => expect(run).toBeEnabled());
  await userEvent.click(run);
  await screen.findByText(/OCR interrupted/);
  expect(close).not.toHaveBeenCalled();
  await userEvent.click(run);
  await waitFor(() => expect(close).toHaveBeenCalledTimes(1));
  expect(invalidation).toHaveBeenCalled();
  expect(requests[0].headers).toEqual(requests[1].headers);
  expect(JSON.parse(requests[0].body as string)).toEqual({
    model: "gpt-6-luna",
  });
});
it("does not silently select a different model", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      Response.json(
        path.endsWith("/models")
          ? [{ id: "another-model", name: "Another model" }]
          : { supported: true, connected: true },
      ),
    ),
  );
  mount(<OCRDialog fileID="file" onClose={() => {}} />);
  await screen.findByRole("button", { name: "Extract text (OCR)" });
  expect(
    screen.getByRole("button", { name: "Extract text (OCR)" }),
  ).toBeDisabled();
});
it("preserves multiline OCR text without truncating the value", () => {
  const text = "证件\n姓名：张三\n".repeat(1000);
  mount(<OCRText text={text} />);
  expect(screen.getByRole("textbox", { name: "Recognized text" })).toHaveValue(
    text,
  );
});
it("clears connection state and reports unconfirmed revocation", async () => {
  let connected = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
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
  expect(screen.getByRole("status")).toBeVisible();
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
