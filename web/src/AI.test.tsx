import { aiServicesFixture } from "./aiServices.fixture";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { MetadataGroup } from "./MetadataGroup";
import { ImageRotation } from "./ImageRotation";
import type { ArchiveFile } from "./api";
import { FileDetail } from "./FileDetail";
import { OCRAction, OCRText } from "./AI";
import { BackgroundTasks } from "./BackgroundTasks";
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
  if (path.endsWith("/ai/services")) return Response.json(aiServicesFixture());
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
  await screen.findByRole("button", { name: "Queued" });
  expect(run).toBeDisabled();
  expect(JSON.parse(requests[0].body as string)).toEqual({ retry: false });
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
});
it("does not run OCR before a default model is saved", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path.endsWith("/ai/services"))
        return Response.json(aiServicesFixture({ model: "" }));
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
    "Choose an account and model for this feature in AI settings first.",
  );
  expect(
    screen.queryByRole("button", { name: "Extract text (OCR)" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "AI settings" })).toHaveAttribute(
    "href",
    "/settings#settings-ai",
  );
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
it("shows an explicit empty OCR result without a blank editor or copy action", () => {
  mount(<OCRText text="" />);
  expect(screen.getByText("No text was recognized.")).toBeVisible();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Copy recognized text" }),
  ).not.toBeInTheDocument();
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
  await screen.findByRole("button", { name: "Queued" });
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
        path.endsWith("/ai")
          ? { supported: true, connected: true }
          : path.endsWith("/ai/services")
            ? aiServicesFixture()
            : path.endsWith("/ai/ocr")
              ? {
                  paused: true,
                  scanning: false,
                  queued: 0,
                  failed: 1,
                  running: "",
                  error: "context canceled",
                }
              : path.includes("/files/")
                ? { state: "failed", error: "context canceled" }
                : { queued: 0, failed: 0, paused: false },
      ),
    ),
  );
  mount(
    <>
      <BackgroundTasks />
      <MetadataGroup title="OCR" fileID="file" extractor="ocr">
        Existing text
      </MetadataGroup>
    </>,
  );
  await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(2));
  for (const alert of screen.getAllByRole("alert")) {
    expect(alert).toHaveTextContent("Task canceled.");
  }
  expect(screen.queryByText("context canceled")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("link", { name: "AI settings" }),
  ).not.toBeInTheDocument();
  expect(
    within(
      screen.getByRole("rowheader", { name: /^OCR/ }).closest("tr")!,
    ).getByRole("button", { name: "Retry" }),
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
  mount(<MetadataGroup title="OCR" fileID="file" extractor="ocr" empty />);
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

it.each([
  ["desc", "queued", "Queued"],
  ["desc", "running", "Extracting…"],
  ["ocr", "running", "Extracting…"],
  ["facts", "running", "Extracting…"],
  ["orientation", "running", "Extracting…"],
])(
  "keeps the initial %s %s state in its header",
  async (kind, state, label) => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async (path: string) =>
          defaults(path) ||
          Response.json(
            path.endsWith("/ai")
              ? { supported: true, connected: true }
              : { state, manual: true },
          ),
      ),
    );
    mount(<MetadataGroup title="Task" fileID="file" extractor={kind} empty />);
    const button = await screen.findByRole("button", { name: label });
    expect(button).toBeDisabled();
    expect(button.closest(".metadata-group-heading")).not.toBeNull();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "AI settings" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  },
);

it.each([
  ["synthetic.wav", "audio", "audio/wave"],
  ["synthetic.mov", "video", "video/quicktime"],
] as const)(
  "offers transcription for %s and keeps its output in metadata",
  async (name, kind, media_type) => {
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
            path.endsWith("/ai") ? { supported: true } : { state: "missing" },
          )
        );
      }),
    );
    mount(
      <FileDetail
        data={{
          file: {
            id: "audio-file",
            name,
            captured_at: null,
            folder_id: "folder",
            blob_key: "blob",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
            kind,
            media_type,
          },
          metadata: [],
          extracted: [],
          tags: [],
        }}
        tags={[]}
      />,
    );
    const run = await screen.findByRole("button", {
      name: "Transcribe speech",
    });
    await waitFor(() => expect(run).toBeEnabled());
    expect(
      screen.queryByRole("button", { name: "Extract text (OCR)" }),
    ).not.toBeInTheDocument();
    await userEvent.click(run);
    expect(writes).toEqual(["/api/v1/files/audio-file/transcription"]);
  },
);
