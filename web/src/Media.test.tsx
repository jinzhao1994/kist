import { ThemeProvider } from "./theme";
import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Media } from "./Media";
import { I18nProvider } from "./i18n";
import type { ArchiveFile, MediaJob } from "./api";

const file: ArchiveFile = {
  id: "photo",
  folder_id: "folder",
  blob_key: "sha256:photo",
  name: "Photo.heic",
  kind: "image",
  media_type: "image/heic",
  captured_at: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};
function job(
  state: MediaJob["state"],
  generation = "v1",
  reason = "",
): MediaJob {
  return {
    state,
    generation,
    reason,
    blob_key: file.blob_key,
    recipe: "preview_2048_v1",
    updated_at: "2026-01-01T00:00:00Z",
    result: {
      bytes: 30,
      width: 32,
      height: 16,
      generation_ms: 5,
      engine: "libheif",
    },
  };
}
function mount(props: Partial<Parameters<typeof Media>[0]> = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return {
    client,
    ...render(
      <QueryClientProvider client={client}>
        <ThemeProvider env="test">
          <I18nProvider>
            <Media file={file} detail {...props} />
          </I18nProvider>
        </ThemeProvider>
      </QueryClientProvider>,
    ),
  };
}
afterEach(() => {
  vi.unstubAllGlobals();
});

it("requests missing HEIC previews and only loads derived bytes", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(Response.json(job("missing")))
    .mockResolvedValueOnce(Response.json(job("ready")));
  vi.stubGlobal("fetch", fetch);
  mount();
  const img = await screen.findByRole("img");
  expect(img.getAttribute("src")).toBe(
    "/api/v1/files/photo/derived/preview_2048_v1/content?v=v1",
  );
  expect(fetch.mock.calls[1][1]).toMatchObject({
    method: "POST",
    body: '{"retry":false}',
  });
  expect(
    fetch.mock.calls.some(([path]) => path === "/api/v1/files/photo/content"),
  ).toBe(false);
});

it("shows missing dependencies and supports cancel followed by explicit retry", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(
      Response.json(job("blocked", "v1", "dependency_missing")),
    )
    .mockResolvedValueOnce(Response.json(job("cancelled", "v2")))
    .mockResolvedValueOnce(Response.json(job("ready", "v3")));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(await screen.findByText(/tools are missing/i)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  expect(await screen.findByText(/generation cancelled/i)).toBeVisible();
  expect(fetch.mock.calls[1][1]).toMatchObject({ method: "DELETE" });
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(await screen.findByRole("img")).toHaveAttribute(
    "src",
    expect.stringContaining("v=v3"),
  );
  expect(fetch.mock.calls[2][1]).toMatchObject({
    method: "POST",
    body: '{"retry":true}',
  });
});

it("reconciles a deleted output without falling back to its original", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(Response.json(job("ready")))
    .mockResolvedValueOnce(Response.json(job("missing")))
    .mockResolvedValueOnce(Response.json(job("ready", "new")));
  vi.stubGlobal("fetch", fetch);
  mount();
  fireEvent.error(await screen.findByRole("img"));
  await waitFor(() =>
    expect(screen.getByRole("img")).toHaveAttribute(
      "src",
      expect.stringContaining("v=new"),
    ),
  );
});

it("does not request inactive media and uses MP4 for video details", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(job("ready")));
  vi.stubGlobal("fetch", fetch);
  const inactive = mount({ active: false });
  expect(fetch).not.toHaveBeenCalled();
  inactive.unmount();
  const pause = vi
    .spyOn(HTMLMediaElement.prototype, "pause")
    .mockImplementation(() => {});
  const load = vi
    .spyOn(HTMLMediaElement.prototype, "load")
    .mockImplementation(() => {});
  const view = mount({
    file: {
      ...file,
      kind: "video",
      name: "Clip.mov",
      media_type: "video/quicktime",
    },
  });
  const video = await screen.findByLabelText("Clip.mov");
  expect(video.tagName).toBe("VIDEO");
  expect(video).toHaveAttribute(
    "src",
    expect.stringContaining("/derived/preview_1080p_v1/content"),
  );
  view.unmount();
  expect(pause).toHaveBeenCalledOnce();
  expect(load).toHaveBeenCalledOnce();
  expect(video).not.toHaveAttribute("src");
  pause.mockRestore();
  load.mockRestore();
});

it("retries status failures and keeps a failed browser preview as a placeholder", async () => {
  const fetch = vi
    .fn()
    .mockRejectedValueOnce(new Error("offline"))
    .mockImplementation(async () => Response.json(job("ready")));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(await screen.findByText(/could not read/i)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  fireEvent.error(await screen.findByRole("img"));
  expect(await screen.findByText(/generation failed/i)).toBeVisible();
  expect(screen.queryByRole("img")).not.toBeInTheDocument();
});

for (const [name, kind, media_type] of [
  ["Clip.MOV", "video", "video/quicktime"],
  ["Report.pdf", "document", "application/pdf"],
]) {
  it(`requests a derived thumbnail for ${name}`, async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(job("ready")));
    vi.stubGlobal("fetch", fetch);
    mount({ detail: false, file: { ...file, name, kind, media_type } });
    expect(await screen.findByRole("img", { name })).toHaveAttribute(
      "src",
      expect.stringContaining(
        kind === "video"
          ? "/derived/video_thumb_512_v2/content"
          : "/derived/thumb_512_v1/content",
      ),
    );
  });
}
it("renders a PDF first-page preview without embedding the original document", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(Response.json(job("ready"))),
  );
  mount({
    file: {
      ...file,
      name: "Report.pdf",
      kind: "document",
      media_type: "application/pdf",
    },
  });
  expect(await screen.findByRole("img")).toHaveAttribute(
    "src",
    expect.stringContaining("/derived/preview_2048_v1/content"),
  );
  expect(document.querySelector("iframe, object, embed")).toBeNull();
});

for (const detail of [false, true]) {
  it(`renders an existing STL file with detail=${detail}`, async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(Response.json(job("ready"))),
    );
    mount({
      detail,
      file: {
        ...file,
        name: "Model.STL",
        kind: "other",
        media_type: "application/octet-stream",
      },
    });
    expect(
      await screen.findByRole("img", { name: "Model.STL" }),
    ).toHaveAttribute(
      "src",
      expect.stringContaining(
        `/derived/${detail ? "stl_preview_2048_v1" : "stl_thumb_512_v1"}/content`,
      ),
    );
  });
}
