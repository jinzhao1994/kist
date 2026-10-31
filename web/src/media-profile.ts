import type { ArchiveFile } from "./api";
import { mediaProfiles } from "./media-profiles.generated";

export function mediaProfile(
  file: Pick<
    ArchiveFile,
    "name" | "kind" | "media_type" | "primary_name" | "image_rotation"
  >,
) {
  const extension =
    /\.[^./\\]+$/.exec(file.primary_name || file.name)?.[0].toLowerCase() || "";
  const profile = mediaProfiles.find((profile) => {
    const extensionMatch =
      profile.extensions.some((value) => value === extension) &&
      (!profile.kind || !file.media_type);
    const kindMatch = !!profile.kind && profile.kind === file.kind;
    return (
      (!profile.kind || kindMatch || (!file.kind && extensionMatch)) &&
      (profile.extensions.length + profile.media_types.length === 0 ||
        extensionMatch ||
        profile.media_types.some((value) => value === file.media_type) ||
        (kindMatch && profile.media_types.length === 0))
    );
  });
  const angle = file.image_rotation || 0;
  if (
    profile &&
    [90, 180, 270].includes(angle) &&
    ["native-image", "image"].includes(profile.id)
  ) {
    return {
      ...profile,
      id: "image" as const,
      thumbnail: `${profile.thumbnail}_cw${angle}`,
      detail: `${profile.corrected_detail}_cw${angle}`,
    };
  }
  return profile;
}

// Catalog hints control UI availability; the backend validates the actual bytes.
export function supportsImageAI(
  file: Pick<ArchiveFile, "name" | "media_type">,
) {
  return file.media_type
    ? [
        "image/jpeg",
        "image/png",
        "image/webp",
        "image/heic",
        "image/heif",
      ].includes(file.media_type)
    : /\.(?:jpe?g|png|webp|heic|heif)$/i.test(file.name);
}

export function supportsDescriptionAI(
  file: Parameters<typeof mediaProfile>[0],
) {
  return (
    supportsImageAI(file) ||
    ["pdf", "word", "slides"].includes(mediaProfile(file)?.id || "")
  );
}

// UI availability is a hint; the server checks actual audio bytes and limits.
export function supportsTranscriptionAI(
  file: Pick<ArchiveFile, "name" | "kind" | "media_type">,
) {
  return (
    file.kind === "audio" ||
    (file.media_type
      ? file.media_type.startsWith("audio/")
      : /\.(?:mp3|wav|m4a|flac|ogg|mpga|webm)$/i.test(file.name))
  );
}
