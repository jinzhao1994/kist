import { afterEach, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  FilesetPreview,
  primaryFile,
  captionText,
  zeroID,
  type Fileset,
} from "./Filesets";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile } from "./api";
vi.mock("./Media", () => ({
  Media: ({ file }: { file: ArchiveFile }) => (
    <div data-testid="media">{file.name}</div>
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
it("reads nested Live Photo pages and opens the displayed original", () => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  const open = vi.fn();
  const files = ["still", "motion", "second"].map((id) => ({
    id,
    name: id,
    folder_id: "live",
    blob_key: id,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  })) as ArchiveFile[];
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <FilesetPreview
          set={parent}
          sets={[parent, child]}
          files={files}
          open={open}
        />
      </I18nProvider>
    </ThemeProvider>,
  );
  expect(screen.getByTestId("media")).toHaveTextContent("still");
  fireEvent.click(screen.getByRole("button", { name: "Play motion" }));
  expect(screen.getByTestId("media")).toHaveTextContent("motion");
  fireEvent.click(screen.getByRole("button", { name: "Open this file" }));
  expect(open).toHaveBeenCalledWith(files[1]);
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
