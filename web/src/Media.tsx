import { useState } from "react";
import { contentURL, type ArchiveFile } from "./api";
import { useI18n } from "./i18n";
export const imageTypes = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
  "image/x-icon",
]);
const videoTypes = new Set(["video/mp4", "video/webm", "video/ogg"]);
const audioTypes = new Set([
  "audio/mpeg",
  "audio/wave",
  "audio/aiff",
  "audio/ogg",
]);
export function Media({
  file,
  active = true,
  detail = false,
}: {
  file: ArchiveFile;
  active?: boolean;
  detail?: boolean;
}) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  const mime = file.media_type || "";
  if (active && !failed && imageTypes.has(mime))
    return (
      <img
        className={detail ? "detail-image" : "file-image"}
        src={contentURL(file)}
        alt={file.name}
        decoding="async"
        onError={() => setFailed(true)}
      />
    );
  if (detail && !failed && videoTypes.has(mime))
    return (
      <video
        className="detail-image"
        src={contentURL(file)}
        controls
        preload="metadata"
        onError={() => setFailed(true)}
        aria-label={file.name}
      />
    );
  if (detail && !failed && audioTypes.has(mime))
    return (
      <audio
        src={contentURL(file)}
        controls
        preload="metadata"
        onError={() => setFailed(true)}
        aria-label={file.name}
      />
    );
  const kind =
    (["image", "video", "audio", "document", "other"] as const).find(
      (kind) => kind === file.kind,
    ) || "unknown";
  return (
    <div className="media-placeholder">
      <span aria-hidden="true">
        {kind === "image"
          ? "▧"
          : kind === "video"
            ? "▷"
            : kind === "audio"
              ? "♫"
              : "▤"}
      </span>
      <small>{t(kind)}</small>
      {detail && <p>{t("previewUnavailable")}</p>}
    </div>
  );
}
