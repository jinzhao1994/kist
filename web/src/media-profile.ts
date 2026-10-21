import type { ArchiveFile } from "./api";
import { mediaProfiles } from "./media-profiles.generated";

export function mediaProfile(
  file: Pick<ArchiveFile, "name" | "kind" | "media_type">,
) {
  const extension = /\.[^./\\]+$/.exec(file.name)?.[0].toLowerCase() || "";
  return mediaProfiles.find(
    (profile) =>
      (!profile.kind || profile.kind === file.kind) &&
      (profile.extensions.length + profile.media_types.length === 0 ||
        profile.extensions.some((value) => value === extension) ||
        profile.media_types.some((value) => value === file.media_type)),
  );
}
