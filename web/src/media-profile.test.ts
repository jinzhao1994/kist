import { expect, it } from "vitest";
import { mediaProfile } from "./media-profile";

it.each([
  ["photo.png", "image", "image/png", "thumb_512_v1", ""],
  ["photo.heic", "image", "image/heic", "thumb_512_v1", "preview_2048_v1"],
  [
    "clip.mov",
    "video",
    "video/quicktime",
    "video_thumb_512_v2",
    "preview_1080p_v1",
  ],
  ["scan.pdf", "document", "application/pdf", "thumb_512_v1", ""],
  ["REPORT.DOCX", "other", "", "office_thumb_512_v1", "office_pdf_v1"],
  [
    "slides",
    "other",
    "application/vnd.ms-powerpoint",
    "slides_thumb_512_v1",
    "slides_pdf_v1",
  ],
  ["track.GPX", "other", "", "gpx_map_v1", "gpx_map_v1"],
  ["model.stl", "other", "", "stl_thumb_512_v1", "stl_preview_2048_v1"],
])(
  "selects both preview contexts for %s",
  (name, kind, media_type, thumbnail, detail) => {
    const profile = mediaProfile({ name, kind, media_type });
    expect(profile?.thumbnail).toBe(thumbnail);
    expect(profile?.detail).toBe(detail);
  },
);
it("leaves text and spreadsheet readers on their original bytes", () => {
  expect(
    mediaProfile({ name: "note.txt", kind: "text", media_type: "text/plain" }),
  ).toBeUndefined();
  expect(mediaProfile({ name: "sheet.xlsx", kind: "other" })).toBeUndefined();
});
