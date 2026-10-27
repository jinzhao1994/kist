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
