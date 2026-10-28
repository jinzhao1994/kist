import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { MetadataGroup } from "./MetadataGroup";
import { ImageRotation } from "./ImageRotation";
import type { ArchiveFile } from "./api";
import { FileDetail } from "./FileDetail";
import { AIConnection, BackgroundTaskQueue, OCRAction, OCRText } from "./AI";
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
      auto_desc: false,
      auto_orientation: false,
      proxy_url: "",
      revision: "one",
    });
  if (
    path.endsWith("/ai/ocr") ||
    path.endsWith("/ai/desc") ||
    path.endsWith("/ai/orientation")
  )
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
        : Response.json({ state: "queued", manual: true });
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
          auto_desc: false,
          auto_orientation: false,
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
    "Connect an account and save a default AI model in AI settings first.",
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
          auto_desc: false,
          auto_orientation: false,
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
  await userEvent.click(
    screen.getByRole("button", { name: "Save AI settings" }),
  );
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toEqual({
    path: "/api/v1/ai/settings",
    body: {
      model: "gpt-6-luna",
      auto_ocr: true,
      auto_desc: false,
      auto_orientation: false,
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
  expect(button.querySelector("bdi")?.textContent).toBe(
    Array(1000).fill("证件 名称 详情↵ 姓名：张三").join("↵ "),
  );
  await user.click(button.querySelector("bdi")!);
  expect(copy).toHaveBeenLastCalledWith(text);
  await user.click(button.querySelector(".copy-text-action")!);
  await user.keyboard("{Enter}");
  await user.keyboard(" ");
  expect(copy).toHaveBeenCalledTimes(4);
  expect(copy).toHaveBeenLastCalledWith(text);
  expect(button).toHaveAccessibleName("Copied");
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

it("queues a description independently of OCR", async () => {
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        writes.push(path);
        return Response.json({ state: "queued", manual: true });
      }
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
  mount(<OCRAction fileID="file" kind="desc" />);
  const run = await screen.findByRole("button", {
    name: "Generate description",
  });
  await waitFor(() => expect(run).toBeEnabled());
  await userEvent.click(run);
  await screen.findByText("Queued for description");
  expect(writes).toEqual(["/api/v1/files/file/desc"]);
});

it.each([
  ["scan.PNG", ""],
  ["scan.HEIC", ""],
  ["scan.HEIF", ""],
  ["renamed.bin", "image/heic"],
  ["renamed.bin", "image/heif"],
])("offers image AI actions for %s (%s)", async (name, mediaType) => {
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        writes.push(path);
        return Response.json({ state: "queued", manual: true });
      }
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
  mount(
    <FileDetail
      data={{
        file: {
          id: "file",
          name,
          captured_at: null,
          folder_id: "folder",
          blob_key: "blob",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
          media_type: mediaType,
          extraction_pending: ["media"],
        },
        metadata: [],
        tags: [],
        extracted: [],
      }}
      tags={[]}
    />,
  );
  const run = await screen.findByRole("button", {
    name: "Generate description",
  });
  await waitFor(() => expect(run).toBeEnabled());
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Extract text (OCR)" }),
    ).toBeEnabled(),
  );
  await userEvent.click(run);
  expect(writes).toEqual(["/api/v1/files/file/desc"]);
});
it("shows original OCR whitespace in the full-text tooltip", async () => {
  const text = "First  line\n\nSecond line\tvalue";
  mount(<OCRText text={text} />);
  await userEvent.hover(document.querySelector(".copy-text-preview")!);
  expect((await screen.findByRole("tooltip")).textContent).toBe(text);
});

it("restores automatic direction with a stable request key after a lost response", async () => {
  const writes: { path: string; init: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method) {
        writes.push({ path, init });
        if (
          path.endsWith("/orientation") &&
          writes.filter((w) => w.path === path).length === 1
        )
          return Response.json(
            { code: "internal", message: "Lost response" },
            { status: 502 },
          );
      }
      return Response.json({ state: "queued", manual: true });
    }),
  );
  const file = {
    id: "file",
    blob_key: "blob",
    image_rotation: 90,
    image_rotation_manual: true,
  } as ArchiveFile;
  mount(<ImageRotation file={file} close={vi.fn()} />);
  await userEvent.click(
    screen.getByRole("button", { name: "Restore AI orientation" }),
  );
  await screen.findByText(/Lost response/);
  await userEvent.click(
    screen.getByRole("button", { name: "Restore AI orientation" }),
  );
  const jobs = writes.filter((w) => w.path.endsWith("/orientation"));
  expect(jobs).toHaveLength(2);
  expect(JSON.parse(jobs[0].init.body as string)).toEqual(
    expect.objectContaining({ force: true, retry: false }),
  );
  expect(jobs[0].init.body).toBe(jobs[1].init.body);
  const overrides = writes.filter((w) => w.path.endsWith("/image-rotation"));
  expect(JSON.parse(overrides[0].init.body as string)).toEqual({
    blob_key: "blob",
    clockwise_degrees: null,
  });
  expect(overrides[0].init.headers).toEqual(overrides[1].init.headers);
});

it("explains canceled tasks in both queue settings and metadata", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      Response.json(
        path.endsWith("/ai/ocr")
          ? {
              paused: true,
              scanning: false,
              queued: 0,
              failed: 1,
              running: "",
              error: "context canceled",
            }
          : { state: "failed", error: "context canceled" },
      ),
    ),
  );
  mount(
    <>
      <BackgroundTaskQueue
        kind="ocr"
        disabled={false}
        connected
        model="gpt-6-luna"
      />
      <MetadataGroup title="OCR" fileID="file" extractor="ocr">
        Existing text
      </MetadataGroup>
    </>,
  );
  await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(2));
  for (const alert of screen.getAllByRole("alert")) {
    expect(alert).toHaveTextContent("Task canceled. You can retry it.");
  }
  expect(screen.queryByText("context canceled")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("link", { name: "AI settings" }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Retry failed tasks" }),
  ).toBeEnabled();
});

it("allows single-file OCR without checking or resuming the paused bulk queue", async () => {
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      paths.push(path);
      if (path.endsWith("/ai/ocr")) return Response.json({ paused: true });
      if (init?.method === "POST")
        return Response.json({ state: "queued", manual: true });
      return (
        defaults(path) ||
        Response.json(
          path.endsWith("/ai")
            ? { supported: true, connected: true }
            : { state: "queued", automatic: true },
        )
      );
    }),
  );
  mount(<OCRAction fileID="file" />);
  const button = screen.getByRole("button", { name: "Extract text (OCR)" });
  await waitFor(() => expect(button).toBeEnabled());
  await userEvent.click(button);
  await waitFor(() => expect(button).toBeDisabled());
  expect(paths).toContain("/api/v1/files/file/ocr");
  expect(paths.some((path) => path.includes("/ai/ocr"))).toBe(false);
  expect(screen.queryByText(/paused/i)).not.toBeInTheDocument();
});

it.each(["queued", "running", "failed"])(
  "does not show empty image metadata groups for background %s tasks",
  async (state) => {
    const reads: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (path: string) => {
        reads.push(path);
        return defaults(path) || Response.json({ state, manual: false });
      }),
    );
    mount(
      <FileDetail
        data={{
          file: {
            id: "file",
            name: "synthetic.png",
            captured_at: null,
            folder_id: "folder",
            blob_key: "blob",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
            media_type: "image/png",
            extraction_pending: [],
          },
          metadata: [],
          tags: [],
          extracted: [],
        }}
        tags={[]}
      />,
    );
    await waitFor(() =>
      expect(reads).toEqual(
        expect.arrayContaining([
          "/api/v1/files/file/ocr",
          "/api/v1/files/file/desc",
          "/api/v1/files/file/orientation",
        ]),
      ),
    );
    expect(
      screen.queryByRole("heading", { name: "OCR" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Description" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Image orientation" }),
    ).not.toBeInTheDocument();
  },
);

it("keeps a failed empty-group status request stable until explicit retry", async () => {
  let jobReads = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path.endsWith("/files/file/ocr")) {
        jobReads++;
        return jobReads === 1
          ? Response.json(
              { code: "internal", message: "Job unavailable" },
              { status: 502 },
            )
          : Response.json({ state: "missing" });
      }
      return (
        defaults(path) || Response.json({ supported: true, connected: true })
      );
    }),
  );
  mount(
    <MetadataGroup title="OCR" fileID="file" extractor="ocr" empty>
      <OCRAction fileID="file" />
    </MetadataGroup>,
  );
  await screen.findByRole("alert");
  expect(jobReads).toBe(1);
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Extract text (OCR)" }),
    ).toBeEnabled(),
  );
  expect(jobReads).toBe(2);
});
