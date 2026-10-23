import { ThemeProvider } from "./theme";
import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Media } from "./Media";
import { I18nProvider } from "./i18n";
import type { ArchiveFile, MediaJob } from "./api";

vi.mock("./PDFReader", () => ({
  default: ({ src, name }: { src: string; name: string }) => (
    <div aria-label={name} data-src={src} />
  ),
}));

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
it("opens PDF originals without requesting a derived preview", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  mount({
    file: {
      ...file,
      name: "Report.pdf",
      kind: "document",
      media_type: "application/pdf",
    },
  });
  expect(await screen.findByLabelText("Report.pdf")).toHaveAttribute(
    "data-src",
    "/api/v1/files/photo/content",
  );
  expect(fetch).not.toHaveBeenCalled();
  expect(screen.queryByRole("img")).toBeNull();
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

for (const detail of [false, true]) {
  it(`requests GPX coordinates for an existing unclassified file with detail=${detail}`, async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(Response.json(job("failed", "v1", "decode_failed")));
    vi.stubGlobal("fetch", fetch);
    mount({
      detail,
      file: {
        ...file,
        name: "Route.GPX",
        kind: "other",
        media_type: "application/octet-stream",
      },
    });
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(fetch.mock.calls[0][0]).toBe(
      "/api/v1/files/photo/derived/gpx_map_v1",
    );
    expect(screen.queryByRole("img")).toBeNull();
  });
}

for (const name of ["Report.DOC", "Report.docx", "Slides.PPT", "Slides.pptx"]) {
  it(`renders ${name} through a derived PDF and keeps the original untouched`, async () => {
    const fetch = vi
      .fn()
      .mockImplementation(async () => Response.json(job("ready")));
    vi.stubGlobal("fetch", fetch);
    const family = /\.pptx?$/i.test(name) ? "slides" : "office";
    const word = {
      ...file,
      name,
      kind: "other",
      media_type: "application/octet-stream",
    };
    const view = mount({ file: word });
    expect(await screen.findByLabelText(name)).toHaveAttribute(
      "data-src",
      `/api/v1/files/photo/derived/${family}_pdf_v1/content?v=v1`,
    );
    expect(fetch.mock.calls[0][0]).toBe(
      `/api/v1/files/photo/derived/${family}_pdf_v1`,
    );
    expect(
      fetch.mock.calls.some(([url]) => url === "/api/v1/files/photo/content"),
    ).toBe(false);
    view.unmount();
    mount({ file: word, detail: false });
    expect(await screen.findByRole("img", { name })).toHaveAttribute(
      "src",
      `/api/v1/files/photo/derived/${family}_thumb_512_v1/content?v=v1`,
    );
  });
}

for (const media_type of [
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
  "image/x-icon",
]) {
  it(`loads ${media_type} originals only in active details`, async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(job("ready")));
    vi.stubGlobal("fetch", fetch);
    const native = { ...file, name: "Image", media_type };
    const view = mount({ file: native });
    expect(screen.getByRole("img")).toHaveAttribute(
      "src",
      "/api/v1/files/photo/content",
    );
    expect(fetch).not.toHaveBeenCalled();
    view.unmount();
    const inactive = mount({ file: native, active: false });
    expect(screen.queryByRole("img")).toBeNull();
    expect(fetch).not.toHaveBeenCalled();
    inactive.unmount();
    mount({ file: native, detail: false });
    expect(await screen.findByRole("img")).toHaveAttribute(
      "src",
      expect.stringContaining("/derived/thumb_512_v1/content"),
    );
  });
}
it("shows and retries original image failures without generating a compressed substitute", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  mount({ file: { ...file, media_type: "image/png" } });
  fireEvent.error(screen.getByRole("img"));
  expect(screen.getByRole("status")).toHaveTextContent(
    "Could not load the original image.",
  );
  expect(screen.queryByRole("img")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(screen.getByRole("img")).toHaveAttribute(
    "src",
    "/api/v1/files/photo/content",
  );
  expect(fetch).not.toHaveBeenCalled();
});

it("exposes all subtitle tracks to native controls with only the configured default", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(Response.json(job("ready"))),
  );
  const { container } = mount({
    file: {
      ...file,
      kind: "video",
      media_type: "video/mp4",
      name: "movie.mp4",
    },
    tracks: [
      { src: "blob:en", label: "English", language: "en" },
      { src: "blob:zh", label: "中文", language: "zh", default: true },
    ],
  });
  await screen.findByLabelText("movie.mp4");
  const tracks = container.querySelectorAll("video track");
  expect(tracks).toHaveLength(2);
  expect(tracks[0]).not.toHaveAttribute("default");
  expect(tracks[1]).toHaveAttribute("default");
  const track = { mode: "disabled" };
  Object.defineProperty(tracks[1], "track", { value: track });
  fireEvent.load(tracks[1]);
  expect(track.mode).toBe("disabled");
});

it("preloads Live Photo motion inline, toggles playback and restores the photo on end", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(Response.json(job("ready"))),
  );
  const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
  const pause = vi
    .spyOn(HTMLMediaElement.prototype, "pause")
    .mockImplementation(() => {});
  const load = vi
    .spyOn(HTMLMediaElement.prototype, "load")
    .mockImplementation(() => {});
  const view = mount({
    file: {
      ...file,
      id: "motion",
      kind: "video",
      media_type: "video/mp4",
      name: "motion.mp4",
    },
    livePhoto: { ...file, name: "photo.jpg", media_type: "image/jpeg" },
  });
  expect(screen.getByRole("img", { name: "photo.jpg" })).toBeVisible();
  const button = await screen.findByRole("button", { name: "Play motion" });
  await waitFor(() => expect(button).toBeEnabled());
  const video = view.container.querySelector("video")!;
  expect(video).toHaveAttribute("preload", "auto");
  expect(video).toHaveAttribute("playsinline");
  expect(video).not.toHaveAttribute("controls");
  expect(play).not.toHaveBeenCalled();
  fireEvent.click(button);
  expect(play).toHaveBeenCalledOnce();
  fireEvent.playing(video);
  expect(button).toHaveAttribute("aria-pressed", "true");
  fireEvent.click(button);
  expect(button).toHaveAttribute("aria-pressed", "false");
  fireEvent.click(button);
  fireEvent.playing(video);
  fireEvent.ended(video);
  expect(button).toHaveAttribute("aria-pressed", "false");
  play.mockRejectedValueOnce(new Error("Playback denied"));
  fireEvent.click(button);
  await waitFor(() => expect(screen.getByRole("status")).toBeVisible());
  expect(button).toHaveAttribute("aria-pressed", "false");
  view.unmount();
  expect(video).not.toHaveAttribute("src");
  play.mockRestore();
  pause.mockRestore();
  load.mockRestore();
});

it("shows an uncached preview without claiming the original is damaged", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(Response.json(job("blocked", "v1", "not_cached"))),
  );
  mount();
  expect(
    await screen.findByText(
      "This preview is not cached. Connect to the remote and retry.",
    ),
  ).toBeInTheDocument();
  expect(screen.queryByText(/generation failed/i)).not.toBeInTheDocument();
});
