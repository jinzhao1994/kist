import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import {
  FilesetPreview,
  primaryFile,
  readingFile,
  readingPagination,
  captionText,
  zeroID,
  type Fileset,
} from "./Filesets";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile } from "./api";
vi.mock("./Media", () => ({
  Media: ({
    file,
    tracks,
    livePhoto,
  }: {
    file: ArchiveFile;
    tracks?: unknown[];
    livePhoto?: ArchiveFile;
  }) => (
    <div
      data-testid="media"
      data-tracks={JSON.stringify(tracks)}
      data-live-photo={livePhoto?.name}
    >
      {file.name}
    </div>
  ),
}));
const member = (id: string, role = "page") => ({
  file_id: id,
  folder_id: zeroID,
  role,
  language: "",
  default: false,
});
const child: Fileset = {
  folder_id: "live",
  kind: "live_photo",
  layout: "flat",
  primary: zeroID,
  members: [member("still", "image"), member("motion", "video")],
};
const parent: Fileset = {
  folder_id: "pages",
  kind: "document_pages",
  layout: "directory",
  primary: zeroID,
  members: [{ ...member(zeroID), folder_id: "live" }, member("second")],
};
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("resolves automatic nested and explicit primaries without cycling", () => {
  expect(primaryFile(parent, [parent, child])).toBe("still");
  expect(primaryFile({ ...parent, primary: "second" }, [parent, child])).toBe(
    "second",
  );
  expect(primaryFile({ ...parent, members: [] }, [])).toBeUndefined();
  expect(
    primaryFile(
      { ...parent, members: [{ ...member(zeroID), folder_id: "pages" }] },
      [parent],
    ),
  ).toBeUndefined();
});
it("preloads nested Live Photo motion with its still image and navigates pages", () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  const files = ["still", "motion", "second"].map((id) => ({
    id,
    name: id,
    folder_id: "live",
    extraction_pending: id === "still" ? ["media"] : [],
    blob_key: id,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  })) as ArchiveFile[];
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FilesetPreview set={parent} sets={[parent, child]} files={files} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  expect(screen.getByTestId("media")).toHaveTextContent("motion");
  expect(screen.getByTestId("media")).toHaveAttribute(
    "data-live-photo",
    "still",
  );
  expect(screen.getByRole("button", { name: "Read metadata" })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(screen.getByTestId("media")).toHaveTextContent("second");
});
it("converts SRT timestamps and retains native VTT", () => {
  expect(captionText("1\n00:00:01,234 --> 00:00:02,500\nHello\n")).toBe(
    "WEBVTT\n\n1\n00:00:01.234 --> 00:00:02.500\nHello\n",
  );
  expect(captionText("WEBVTT\n\n")).toBe("WEBVTT\n\n");
  expect(() => captionText("[Script Info]\nASS")).toThrow();
});

it("resolves nested reading pages independently of the primary and safely handles invalid pages", () => {
  const nested: Fileset = {
    ...child,
    kind: "document_pages",
    primary: "still",
    members: [member("still"), member("motion")],
  };
  expect(readingFile(parent, [nested], [1, 2])).toBe("motion");
  expect(readingFile(parent, [child], [1])).toBe("still");
  expect(readingFile(parent, [child], [2])).toBe("second");
  for (const invalid of [0, -1, 99, 1.5, NaN, Infinity])
    expect(readingFile(parent, [child], [invalid])).toBe("still");
  expect(readingFile({ ...parent, primary: "second" }, [child], [1])).toBe(
    "still",
  );
  expect(readingFile({ ...parent, members: [] }, [], [1])).toBeUndefined();
  const cycle = {
    ...parent,
    members: [{ ...member(zeroID), folder_id: parent.folder_id }],
  };
  expect(readingFile(cycle, [cycle], [1, 1])).toBeUndefined();
});

it("loads all subtitle tracks, preserves the default and tolerates a broken subtitle", async () => {
  vi.spyOn(URL, "createObjectURL")
    .mockReturnValueOnce("blob:en")
    .mockReturnValueOnce("blob:zh");
  const revoke = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (url: string) =>
        new Response(
          url.includes("broken")
            ? "invalid"
            : "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello\n",
        ),
    ),
  );
  const files = ["video", "en", "zh", "broken"].map(
    (id) => ({ id, name: id, blob_key: id }) as ArchiveFile,
  );
  const set: Fileset = {
    ...child,
    kind: "video_subtitles",
    members: [
      member("video", "video"),
      member("en", "subtitle"),
      { ...member("zh", "subtitle"), language: "zh", default: true },
      member("broken", "subtitle"),
    ],
  };
  const { unmount } = render(
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FilesetPreview set={set} sets={[set]} files={files} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  await waitFor(() =>
    expect(JSON.parse(screen.getByTestId("media").dataset.tracks!)).toEqual([
      { src: "blob:en", label: "en", language: "und", default: false },
      { src: "blob:zh", label: "zh", language: "zh", default: true },
    ]),
  );
  expect(screen.getByRole("alert")).toBeVisible();
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  unmount();
  expect(revoke).toHaveBeenCalledWith("blob:en");
  expect(revoke).toHaveBeenCalledWith("blob:zh");
});

it("uses one continuous pager across nested page groups and retains hierarchical paths", () => {
  const group = (id: string, ids: string[]): Fileset => ({
    ...parent,
    folder_id: id,
    members: ids.map((id) => member(id)),
  });
  const first = group("first", ["a", "b"]);
  const second = group("second", ["c", "d"]);
  const root: Fileset = {
    ...parent,
    members: [first, second].map((s) => ({
      ...member(zeroID),
      folder_id: s.folder_id,
    })),
  };
  const sets = [root, first, second];
  expect(readingPagination(root, sets, [2, 1])).toEqual({
    paths: [
      [1, 1],
      [1, 2],
      [2, 1],
      [2, 2],
    ],
    index: 2,
  });
  expect(readingPagination(root, sets, [99, 99]).index).toBe(0);
  const files = ["a", "b", "c", "d"].map(
    (id) => ({ id, name: id }) as ArchiveFile,
  );
  const changePage = vi.fn();
  const { rerender } = render(
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FilesetPreview
            set={root}
            sets={sets}
            files={files}
            pagePath={[1, 2]}
            changePage={changePage}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  expect(
    screen.getAllByRole("toolbar", { name: "Page navigation" }),
  ).toHaveLength(1);
  expect(screen.getByRole("status", { name: "Page number" })).toHaveTextContent(
    "2 / 4",
  );
  expect(screen.getByTestId("media")).toHaveTextContent("b");
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(changePage).toHaveBeenLastCalledWith([2, 1]);
  rerender(
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider env="test">
        <I18nProvider>
          <FilesetPreview
            set={root}
            sets={sets}
            files={files}
            pagePath={[2, 1]}
            changePage={changePage}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  expect(
    screen.getAllByRole("toolbar", { name: "Page navigation" }),
  ).toHaveLength(1);
  expect(screen.getByRole("status", { name: "Page number" })).toHaveTextContent(
    "3 / 4",
  );
  expect(screen.getByTestId("media")).toHaveTextContent("c");
  fireEvent.click(screen.getByRole("button", { name: "Previous page" }));
  expect(changePage).toHaveBeenLastCalledWith([1, 2]);
});

it("keeps empty nested groups reachable and terminates cyclic page references", () => {
  const empty = { ...child, kind: "document_pages", members: [] };
  expect(readingPagination(parent, [empty], [1])).toEqual({
    paths: [[1], [2]],
    index: 0,
  });
  const cycle: Fileset = {
    ...parent,
    members: [{ ...member(zeroID), folder_id: parent.folder_id }],
  };
  expect(readingPagination(cycle, [cycle], [1, 1])).toEqual({
    paths: [[1]],
    index: 0,
  });
});
