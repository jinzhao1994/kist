import { expect, it } from "vitest";
import {
  mediaProfile,
  supportsDescriptionAI,
  supportsImageAI,
  supportsTranscriptionAI,
} from "./media-profile";

it.each([
  ["photo.PNG", "", "", "thumb_512_v1", ""],
  ["photo.heic", "", "", "thumb_512_v1", "preview_2048_v1"],
  ["clip.mov", "", "", "video_thumb_512_v2", "preview_1080p_v1"],
  ["scan.pdf", "", "", "thumb_512_v1", ""],
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

it("uses a combination's primary filename to select its preview", () => {
  expect(
    mediaProfile({ name: "Passport", primary_name: "front.png" })?.id,
  ).toBe("native-image");
});

it.each([90, 180, 270] as const)(
  "uses immutable corrected recipes for %s degrees",
  (image_rotation) => {
    const native = mediaProfile({ name: "photo.jpg", image_rotation });
    expect(native?.id).toBe("image");
    expect(native?.thumbnail).toBe(`thumb_512_v1_cw${image_rotation}`);
    expect(native?.detail).toBe(`image_upright_v1_cw${image_rotation}`);
    expect(mediaProfile({ name: "photo.heic", image_rotation })?.detail).toBe(
      `preview_2048_v1_cw${image_rotation}`,
    );
    expect(mediaProfile({ name: "clip.mov", image_rotation })?.thumbnail).toBe(
      "video_thumb_512_v2",
    );
  },
);

it.each(["scan.pdf", "report.doc", "report.docx", "slides.ppt", "slides.pptx"])(
  "enables description but not image tasks for %s",
  (name) => {
    expect(supportsDescriptionAI({ name })).toBe(true);
    expect(supportsImageAI({ name })).toBe(false);
  },
);
it.each(["sheet.xls", "sheet.xlsx", "table.csv", "clip.mp4"])(
  "does not offer descriptions for %s",
  (name) => {
    expect(supportsDescriptionAI({ name })).toBe(false);
  },
);

it.each([
  "recording.mp3",
  "recording.wav",
  "recording.m4a",
  "recording.flac",
  "movie.mp4",
  "movie.MOV",
  "movie.mkv",
  "movie.webm",
  "movie.avi",
])("offers speech transcription for %s", (name) => {
  expect(supportsTranscriptionAI({ name, kind: "other" })).toBe(true);
});
it("does not treat documents or images as recordings", () => {
  expect(
    supportsTranscriptionAI({
      name: "scan.pdf",
      kind: "document",
      media_type: "application/pdf",
    }),
  ).toBe(false);
  expect(
    supportsTranscriptionAI({
      name: "scan.jpg",
      kind: "image",
      media_type: "image/jpeg",
    }),
  ).toBe(false);
});

it("offers byte-probed container fallbacks without treating text as media", () => {
  expect(
    supportsTranscriptionAI({
      name: "clip.ts",
      kind: "other",
      media_type: "application/octet-stream",
    }),
  ).toBe(true);
  expect(
    supportsTranscriptionAI({
      name: "recording.ogg",
      kind: "other",
      media_type: "application/ogg",
    }),
  ).toBe(true);
  expect(
    supportsTranscriptionAI({
      name: "playlist.ts",
      kind: "text",
      media_type: "text/plain",
    }),
  ).toBe(false);
});
