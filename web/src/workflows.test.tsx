vi.mock("./upload-integrity", () => ({
  calculateIntegrity: vi.fn(async (files: File[]) =>
    files.map((file) => ({
      sha256: "0".repeat(64),
      size: file.size,
    })),
  ),
}));
import { ThemeProvider } from "./theme";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { BoxEditor } from "./BoxEditor";
import { TagManager, TagAssignment } from "./Tags";
import { activeImages } from "./FileGrid";
import { useExtraction } from "./Extraction";
import { Upload } from "./Upload";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile, Tag } from "./api";
function wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>{children}</I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
const file = (id: string, mime = "image/jpeg"): ArchiveFile => ({
  id,
  name: `${id}.jpg`,
  folder_id: "folder",
  blob_key: "blob",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  captured_at: null,
  media_type: mime,
  kind: "image",
});
const tags: Tag[] = [
  {
    id: "parent",
    name: "Travel",
    parent_id: null,
    count: 0,
    children: [{ id: "child", parent_id: "parent", name: "Japan", count: 0 }],
  },
];
beforeEach(() => {
  sessionStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "auto",
    }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  sessionStorage.clear();
});
it("locks a failed box edit and retries the exact request before closing", async () => {
  const fetch = vi
    .fn()
    .mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(new Response('{"op_id":42}'));
  vi.stubGlobal("fetch", fetch);
  const close = vi.fn();
  render(<BoxEditor close={close} />, { wrapper });
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Name"), "旅行 / رحلة");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(screen.getByLabelText("Name")).toBeDisabled();
  expect(close).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(fetch.mock.calls[1][1]).toEqual(fetch.mock.calls[0][1]);
  expect(JSON.parse(fetch.mock.calls[0][1].body).name).toBe("旅行 / رحلة");
});
it("excludes a tag and its descendants from parent choices and preserves keys across a partial edit", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(new Response('{"op_id":1}'))
    .mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(new Response('{"op_id":1}'))
    .mockResolvedValueOnce(new Response('{"op_id":2}'));
  vi.stubGlobal("fetch", fetch);
  render(<TagManager tags={tags} close={vi.fn()} />, { wrapper });
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Select a tag" }));
  await user.click(await screen.findByRole("option", { name: "Travel" }));
  await user.click(screen.getByRole("combobox", { name: "Parent tag" }));
  expect(screen.getAllByRole("option")).toHaveLength(1);
  await user.keyboard("{Escape}");
  await user.clear(screen.getByLabelText("Name"));
  await user.type(screen.getByLabelText("Name"), "Journeys");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByRole("alert");
  await user.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(4));
  expect(fetch.mock.calls[2][1]).toEqual(fetch.mock.calls[0][1]);
  expect(fetch.mock.calls[3][1]).toEqual(fetch.mock.calls[1][1]);
});
it("adds and removes associations through explicit target IDs", async () => {
  const fetch = vi.fn(
    async (_path: string, _init: RequestInit) => new Response('{"op_id":3}'),
  );
  vi.stubGlobal("fetch", fetch);
  const view = render(
    <TagAssignment target="file-1" kind="files" tags={tags} assigned={[]} />,
    { wrapper },
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Select a tag" }));
  await user.click(
    await screen.findByRole("option", { name: "Travel / Japan" }),
  );
  await user.click(screen.getByRole("button", { name: "Add tag" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledOnce());
  expect(fetch.mock.calls[0][0]).toBe("/api/v1/tags/child/files");
  expect(fetch.mock.calls[0][1]).toMatchObject({
    method: "PUT",
    body: '{"ids":["file-1"]}',
  });
  view.unmount();
  render(
    <TagAssignment
      target="box-1"
      kind="boxes"
      tags={tags}
      assigned={["child"]}
    />,
    { wrapper },
  );
  await user.click(screen.getByRole("button", { name: "Remove tag: Japan" }));
  expect(fetch.mock.calls[1][0]).toBe("/api/v1/tags/child/boxes");
  expect(fetch.mock.calls[1][1]).toMatchObject({
    method: "DELETE",
    body: '{"ids":["box-1"]}',
  });
});
it("bounds derived media to visible images including HEIC", () => {
  const files = Array.from({ length: 1000 }, (_, i) =>
    file(String(i), i % 2 ? "image/jpeg" : "image/heic"),
  );
  const active = activeImages(files, 100, 200);
  expect(active.size).toBe(100);
  for (const id of active) {
    expect(Number(id)).toBeGreaterThanOrEqual(100);
    expect(Number(id)).toBeLessThan(200);
  }
});
it("serializes extraction per file, caps concurrency and retries failures without upload", async () => {
  const active = new Set<string>();
  let peak = 0;
  let fail = true;
  const calls: { path: string; key: string }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit) => {
      const id = path.split("/")[4];
      expect(active.has(id)).toBe(false);
      active.add(id);
      peak = Math.max(peak, active.size);
      calls.push({
        path,
        key: (init.headers as Record<string, string>)["Idempotency-Key"],
      });
      await new Promise((resolve) => setTimeout(resolve, 5));
      active.delete(id);
      if (fail && path.endsWith("/exif")) throw new TypeError("Unavailable");
      return new Response('{"op_id":4}');
    }),
  );
  const files = Array.from({ length: 5 }, (_, i) => ({
    ...file(String(i)),
    extraction_pending: ["media", "exif"],
  }));
  const hook = renderHook(useExtraction, { wrapper });
  await act(() => hook.result.current.run(files));
  expect(peak).toBeLessThanOrEqual(3);
  expect(peak).toBeGreaterThan(1);
  expect(hook.result.current.failed).toBe(true);
  expect(hook.result.current.done).toBe(10);
  fail = false;
  await act(() =>
    hook.result.current.run(
      files.map((f) => ({ ...f, extraction_pending: ["exif"] })),
    ),
  );
  expect(hook.result.current.failed).toBe(false);
  for (const call of calls.slice(10))
    expect(call.key).toBe(calls.find((first) => first.path === call.path)?.key);
});
it("does not report an extraction failure as an upload failure", async () => {
  class XHR {
    upload = {};
    status = 200;
    responseText = '{"op_id":8}';
    onload?: () => void;
    onloadend?: () => void;
    open() {}
    setRequestHeader() {}
    abort() {}
    send() {
      queueMicrotask(() => {
        this.onload?.();
        this.onloadend?.();
      });
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  const complete = vi.fn().mockRejectedValue(new Error("Metadata unavailable"));
  render(
    <Upload box="box" maxFiles={10} maxBytes={10000} onComplete={complete} />,
    { wrapper },
  );
  const user = userEvent.setup();
  await user.upload(
    screen.getByLabelText("Choose files"),
    new File(["hello"], "report.txt"),
  );
  await waitFor(() => expect(complete).toHaveBeenCalledOnce());
  expect(screen.getByText("Files saved")).toBeVisible();
  expect(screen.getByRole("alert")).toHaveTextContent(/metadata/i);
  expect(
    screen.queryByRole("button", { name: "Try again" }),
  ).not.toBeInTheDocument();
});
it("recovers an upload receipt after remount before allowing a new batch", async () => {
  sessionStorage.setItem("kist.pending-upload.v1:box", "retained-request-key");
  const fetch = vi.fn(async (_path: string) => new Response('{"op_id":21}'));
  vi.stubGlobal("fetch", fetch);
  render(
    <Upload
      box="box"
      maxFiles={10}
      maxBytes={10000}
      onComplete={async () => {}}
    />,
    { wrapper },
  );
  const user = userEvent.setup();
  expect(screen.getByRole("button", { name: "Choose files" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Check result" }));
  await screen.findByText("Files saved");
  expect(fetch.mock.calls[0][0]).toBe("/api/v1/requests/retained-request-key");
  expect(sessionStorage.getItem("kist.pending-upload.v1:box")).toBeNull();
  expect(screen.getByRole("button", { name: "Choose files" })).toBeEnabled();
});

function droppedFiles(files: File[]) {
  return {
    dataTransfer: {
      files,
      items: files.map((file) => ({
        kind: "file",
        type: file.type,
        getAsFile: () => file,
      })),
      types: ["Files"],
    },
  };
}

it("keeps a dropped batch locked after cancellation and retries it unchanged", async () => {
  const requests: XHR[] = [];
  class XHR {
    upload: {
      onprogress?: (event: {
        lengthComputable: boolean;
        loaded: number;
        total: number;
      }) => void;
    } = {};
    status = 200;
    responseText = '{"op_id":31}';
    key = "";
    body?: FormData;
    onabort?: () => void;
    onload?: () => void;
    onloadend?: () => void;
    constructor() {
      requests.push(this);
    }
    open() {}
    setRequestHeader(_name: string, value: string) {
      this.key = value;
    }
    send(body: FormData) {
      this.body = body;
    }
    abort() {
      this.onabort?.();
      this.onloadend?.();
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  const complete = vi.fn(async () => {});
  render(
    <Upload
      box="drop-box"
      maxFiles={10}
      maxBytes={1000}
      onComplete={complete}
    />,
    { wrapper },
  );
  const user = userEvent.setup();
  const region = screen.getByRole("region", { name: "Add files" });
  const original = new File(["original"], "旅行.txt", {
    type: "text/plain",
    lastModified: 1234,
  });
  await act(async () => {
    fireEvent.drop(region, droppedFiles([original]));
  });
  expect(requests).toHaveLength(1);
  act(() =>
    requests[0].upload.onprogress?.({
      lengthComputable: true,
      loaded: 1,
      total: 2,
    }),
  );
  expect(screen.getByText("50%")).toBeVisible();
  expect(screen.getByRole("progressbar")).toHaveAttribute("value", "0.5");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(/cancelled/i);
  expect(screen.getByRole("button", { name: "Choose files" })).toBeDisabled();
  expect(sessionStorage.getItem("kist.pending-upload.v1:drop-box")).toBe(
    requests[0].key,
  );
  await act(async () => {
    fireEvent.drop(region, droppedFiles([new File(["new"], "other.txt")]));
  });
  expect(requests).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Try again" }));
  expect(requests).toHaveLength(2);
  expect(requests[1].key).toBe(requests[0].key);
  expect(requests[1].body?.get("manifest")).toBe(
    requests[0].body?.get("manifest"),
  );
  expect((requests[1].body?.get("files") as File).name).toBe("旅行.txt");
  await act(async () => {
    requests[1].onload?.();
    requests[1].onloadend?.();
  });
  expect(screen.getByText("Files saved")).toBeVisible();
  expect(complete).toHaveBeenCalledOnce();
  expect(sessionStorage.getItem("kist.pending-upload.v1:drop-box")).toBeNull();
});

it("rejects oversized dropped batches before sending and accepts a corrected batch", async () => {
  const send = vi.fn();
  class XHR {
    upload = {};
    open() {}
    setRequestHeader() {}
    abort() {}
    send = send;
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  render(
    <Upload
      box="limits"
      maxFiles={1}
      maxBytes={4}
      onComplete={async () => {}}
    />,
    { wrapper },
  );
  const region = screen.getByRole("region", { name: "Add files" });
  for (const files of [
    [new File(["12345"], "large.txt")],
    [new File(["a"], "a.txt"), new File(["b"], "b.txt")],
  ]) {
    await act(async () => {
      fireEvent.drop(region, droppedFiles(files));
    });
    expect(screen.getByRole("alert")).toBeVisible();
    expect(send).not.toHaveBeenCalled();
    expect(sessionStorage.getItem("kist.pending-upload.v1:limits")).toBeNull();
  }
  await act(async () => {
    fireEvent.drop(region, droppedFiles([new File(["ok"], "small.txt")]));
  });
  expect(send).toHaveBeenCalledOnce();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("keeps cancellation accessible while file intake is disabled", async () => {
  class XHR {
    upload = {};
    onabort?: () => void;
    open() {}
    setRequestHeader() {}
    send() {}
    abort() {
      this.onabort?.();
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  render(
    <Upload
      box="cancel-accessible"
      maxFiles={10}
      maxBytes={10000}
      onComplete={async () => {}}
    />,
    { wrapper },
  );
  await userEvent
    .setup()
    .upload(
      screen.getByLabelText("Choose files"),
      new File(["hello"], "cancel.txt"),
    );
  const cancel = await screen.findByRole("button", {
    name: "Cancel",
  });
  expect(cancel.closest('[aria-disabled="true"]')).toBeNull();
  expect(cancel).toBeEnabled();
  await userEvent.setup().click(cancel);
  expect(await screen.findByRole("alert")).toHaveTextContent(/cancelled/i);
});

it("shows resumable recovery after remount and requires original files before missing chunks", async () => {
  const id = "b".repeat(64);
  let state: any;
  let hold = true;
  const offsets: number[] = [];
  const fetch = vi.fn(async (path: string, options?: RequestInit) => {
    if (path === "/api/v1/upload-sessions") {
      const body = JSON.parse(options!.body as string);
      state = {
        version: 1,
        id,
        manifest: { ...body, target: "c".repeat(64) },
        offsets: [0],
        expires: "2030-01-01T00:00:00Z",
      };
    }
    return new Response(
      JSON.stringify(
        path.endsWith("/complete")
          ? { op_id: 45 }
          : { session: structuredClone(state) },
      ),
    );
  });
  vi.stubGlobal("fetch", fetch);
  class XHR {
    upload = {};
    status = 200;
    responseText = "";
    url = "";
    onload?: () => void;
    onabort?: () => void;
    open(_method: string, url: string) {
      this.url = url;
    }
    setRequestHeader() {}
    abort() {
      this.onabort?.();
    }
    send(blob: Blob) {
      const offset = Number(this.url.split("offset=")[1]);
      offsets.push(offset);
      if (hold) return;
      state.offsets[0] = offset + blob.size;
      this.responseText = JSON.stringify({ session: structuredClone(state) });
      queueMicrotask(() => this.onload?.());
    }
  }
  vi.stubGlobal("XMLHttpRequest", XHR);
  const complete = vi.fn(async () => {});
  const props = {
    box: "box",
    folder: "folder",
    maxFiles: 10,
    maxBytes: 10000,
    resumableVersion: 1,
    chunkBytes: 4,
    onComplete: complete,
  };
  localStorage.setItem(
    "kist.upload-timing:box:folder",
    JSON.stringify({
      totalMs: 999000,
      transferMs: 1000,
      bytes: 1000,
      hashMs: 0,
      processingMs: 0,
    }),
  );
  const first = render(<Upload {...props} />, { wrapper });
  expect(document.querySelector(".upload-timing")).toBeNull();
  const user = userEvent.setup();
  const file = new File(["abcdefghij"], "original.bin", { lastModified: 1000 });
  await user.upload(screen.getByLabelText("Choose files"), file);
  await waitFor(() => expect(offsets).toEqual([0]));
  expect(
    document.querySelector(".upload-live-metrics")?.closest(".upload-panel"),
  ).not.toBeNull();
  expect(document.querySelector(".upload-timing")).toBeNull();
  expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
  await user.click(screen.getByRole("button", { name: "Pause" }));
  await screen.findByRole("button", { name: "Resume" });
  // The server confirmed the block, but the response was lost before the UI saw it.
  state.offsets[0] = 4;
  first.unmount();
  hold = false;
  render(<Upload {...props} />, { wrapper });
  await screen.findByText(/Select the entire original batch again/);
  await user.upload(
    screen.getByLabelText("Select original files again"),
    new File(["abcdefghij"], "wrong.bin", { lastModified: 1000 }),
  );
  await screen.findByText(/These are not the original files/);
  expect(offsets).toEqual([0]);
  await user.upload(screen.getByLabelText("Select original files again"), file);
  await screen.findByText("Files saved");
  expect(offsets).toEqual([0, 4, 8]);
  expect(document.querySelector(".upload-timing")).toHaveTextContent(
    "Upload complete",
  );
  expect(document.querySelector(".upload-live-metrics")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Close" }));
  expect(document.querySelector(".upload-timing")).toBeNull();
  expect(complete).toHaveBeenCalledOnce();
  expect(
    localStorage.getItem("kist.resumable-upload.v1:box:folder"),
  ).toBeNull();
});

for (const resumable of [false, true]) {
  it(`keeps the compact ${resumable ? "resumable" : "legacy"} drop target active and reveals intake errors`, async () => {
    render(
      <Upload
        compact
        box="compact"
        folder="folder"
        maxFiles={1}
        maxBytes={4}
        resumableVersion={resumable ? 1 : undefined}
        chunkBytes={resumable ? 4 : undefined}
        onComplete={async () => {}}
      >
        <p>File list content</p>
      </Upload>,
      { wrapper },
    );
    expect(screen.getByText("File list content")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Choose files" })).toBeNull();
    await act(async () => {
      fireEvent.drop(
        screen.getByRole("region", { name: "Add files" }),
        droppedFiles([new File(["12345"], "large.txt")]),
      );
    });
    expect(screen.getByRole("alert")).toBeVisible();
    expect(screen.getByRole("button", { name: "Choose files" })).toBeVisible();
    expect(screen.getByText("File list content")).toBeVisible();
  });
}

it.each(["Overwrite", "Keep both"])(
  "confirms upload conflicts with %s",
  async (choice) => {
    const completed = vi.fn(async () => {});
    const fetcher = vi.fn(async (_url: string, options?: RequestInit) => {
      const body = JSON.parse(String(options?.body || "{}"));
      if (body.conflict_mode)
        return new Response(JSON.stringify({ op_id: 42 }), { status: 200 });
      return new Response(
        JSON.stringify({
          conflicts: {
            token: "c".repeat(64),
            paths: ["same.txt"],
            can_overwrite: true,
          },
        }),
        { status: 200 },
      );
    });
    vi.stubGlobal("fetch", fetcher);
    render(
      <Upload
        box="conflicts"
        folder="root"
        resumableVersion={1}
        chunkBytes={4}
        maxFiles={10}
        maxBytes={10000}
        onComplete={completed}
      />,
      { wrapper },
    );
    const user = userEvent.setup();
    await user.upload(
      screen.getByLabelText("Choose files"),
      new File(["new"], "same.txt"),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Files already exist",
    });
    expect(within(dialog).getByText("same.txt")).toBeVisible();
    expect(completed).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: choice }));
    await waitFor(() => expect(completed).toHaveBeenCalledOnce());
    expect(
      fetcher.mock.calls.some(
        ([, options]) =>
          JSON.parse(String(options?.body || "{}")).conflict_mode ===
          (choice === "Overwrite" ? "overwrite" : "keep_both"),
      ),
    ).toBe(true);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  },
);

it.each([false, true])(
  "confirms clipboard intake before validating or uploading (resumable: %s)",
  async (resumable) => {
    render(
      <main>
        <Upload
          compact
          box="paste"
          folder="folder"
          maxFiles={1}
          maxBytes={4}
          resumableVersion={resumable ? 1 : undefined}
          chunkBytes={resumable ? 4 : undefined}
          onComplete={async () => {}}
        >
          <input aria-label="Rename" />
        </Upload>
      </main>,
      { wrapper },
    );
    const user = userEvent.setup();
    const clipboardData = droppedFiles([
      new File(["12345"], "screenshot.png", { type: "image/png" }),
    ]).dataTransfer;
    fireEvent.paste(screen.getByRole("textbox", { name: "Rename" }), {
      clipboardData,
    });
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.paste(document.body, { clipboardData });
    expect(
      await screen.findByRole("dialog", { name: "Upload pasted files?" }),
    ).toBeVisible();
    expect(screen.getByText("screenshot.png")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.paste(document.body, { clipboardData });
    await user.click(
      await screen.findByRole("button", { name: "Upload files" }),
    );
    expect(await screen.findByRole("alert")).toBeVisible();
  },
);
