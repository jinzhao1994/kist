import type { ArchiveFile } from "./api";
import { mediaProfiles } from "./media-profiles.generated";

export function mediaProfile(
  file: Pick<ArchiveFile, "name" | "kind" | "media_type" | "primary_name">,
) {
  const extension =
    /\.[^./\\]+$/.exec(file.primary_name || file.name)?.[0].toLowerCase() || "";
  return mediaProfiles.find((profile) => {
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
}
