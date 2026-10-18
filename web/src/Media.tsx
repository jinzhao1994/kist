import { Button } from "@mantine/core";
import { lazy, Suspense, useCallback, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  contentURL,
  get,
  mediaAction,
  mediaPath,
  type ArchiveFile,
  type MediaJob,
} from "./api";
import { useI18n, type MessageKey } from "./i18n";

const PDFReader = lazy(() => import("./PDFReader"));
const GPXPreview = lazy(() => import("./GPXPreview"));
function isGPX(file: ArchiveFile) {
  return file.media_type === "application/gpx+xml" || /\.gpx$/i.test(file.name);
}

function isSTL(file: ArchiveFile) {
  return file.media_type === "model/stl" || /\.stl$/i.test(file.name);
}

export function hasDerivedMedia(file: ArchiveFile) {
  return (
    file.kind === "image" ||
    file.kind === "video" ||
    file.media_type === "application/pdf" ||
    isSTL(file) ||
    isGPX(file)
  );
}
const audioTypes = new Set([
  "audio/mpeg",
  "audio/wave",
  "audio/aiff",
  "audio/ogg",
]);
const reasons: Record<string, MessageKey> = {
  dependency_missing: "mediaDependency",
  unsupported: "previewUnavailable",
  limit_exceeded: "mediaLimit",
  timeout: "mediaTimeout",
};
const states: Record<MediaJob["state"], MessageKey> = {
  missing: "mediaQueued",
  queued: "mediaQueued",
  running: "mediaRunning",
  ready: "mediaFailed",
  blocked: "mediaDependency",
  failed: "mediaFailed",
  cancelled: "mediaCancelled",
};

export function Media({
  file,
  active = true,
  detail = false,
  paused = false,
}: {
  file: ArchiveFile;
  active?: boolean;
  detail?: boolean;
  paused?: boolean;
}) {
  const { t } = useI18n();
  const player = useRef<HTMLMediaElement | null>(null);
  const playerRef = useCallback((node: HTMLMediaElement | null) => {
    if (!node && player.current) {
      player.current.pause();
      player.current.removeAttribute("src");
      player.current.load();
    }
    player.current = node;
  }, []);
  const client = useQueryClient();
  const recipe = isGPX(file)
    ? "gpx_map_v1"
    : isSTL(file)
      ? detail
        ? "stl_preview_2048_v1"
        : "stl_thumb_512_v1"
      : detail
        ? file.kind === "video"
          ? "preview_1080p_v1"
          : "preview_2048_v1"
        : file.kind === "video"
          ? "video_thumb_512_v2"
          : "thumb_512_v1";
  const path = mediaPath(file, recipe);
  const key = ["media", file.id, file.blob_key, recipe];
  const readerPDF = detail && file.media_type === "application/pdf";
  const enabled = active && hasDerivedMedia(file) && !readerPDF;
  const [failedGeneration, setFailedGeneration] = useState("");
  const query = useQuery({
    queryKey: key,
    enabled: enabled && !paused,
    queryFn: async ({ signal }) => {
      const state = await get<MediaJob>(path, signal);
      return state.state === "missing"
        ? mediaAction(path, "request", signal)
        : state;
    },
    // Read fresh state after another process cancels, retries, or removes output.
    refetchInterval: (query) => {
      if (paused || !enabled || query.state.error) return false;
      return ["queued", "running", "blocked"].includes(
        query.state.data?.state || "",
      )
        ? 1500
        : 15000;
    },
  });
  const action = useMutation({
    mutationFn: (kind: "retry" | "cancel") => mediaAction(path, kind),
    onSuccess: (job) => client.setQueryData(key, job),
  });
  const job = query.data;
  const failed = failedGeneration === (job?.generation || file.id);
  if (readerPDF && active)
    return (
      <Suspense
        fallback={<div className="media-placeholder">{t("loading")}</div>}
      >
        <PDFReader
          key={file.blob_key}
          src={contentURL(file)}
          name={file.name}
        />
      </Suspense>
    );
  if (enabled && job?.state === "ready" && !failed) {
    const src = `/api/v1${path}/content?v=${encodeURIComponent(job.generation)}`;
    const fail = () => {
      setFailedGeneration(job.generation);
      void query.refetch();
    };
    if (isGPX(file))
      return (
        <Suspense
          fallback={<div className="media-placeholder">{t("mapLoading")}</div>}
        >
          <GPXPreview key={src} src={src} name={file.name} detail={detail} />
        </Suspense>
      );
    return detail && file.kind === "video" ? (
      <video
        ref={playerRef}
        className="detail-image"
        src={src}
        controls
        preload="metadata"
        onError={fail}
        aria-label={file.name}
      />
    ) : (
      <img
        className={detail ? "detail-image" : "file-image"}
        src={src}
        alt={file.name}
        decoding="async"
        onError={fail}
      />
    );
  }
  if (detail && !failed && audioTypes.has(file.media_type || ""))
    return (
      <audio
        ref={playerRef}
        src={contentURL(file)}
        controls
        preload="metadata"
        onError={() => setFailedGeneration(file.id)}
        aria-label={file.name}
      />
    );
  const kind =
    (["image", "video", "audio", "document", "other"] as const).find(
      (kind) => kind === file.kind,
    ) || "unknown";
  const busy = job && ["queued", "running", "blocked"].includes(job.state);
  const terminal =
    job && ["failed", "cancelled", "blocked"].includes(job.state);
  const message = query.isError
    ? "mediaRequestFailed"
    : failed
      ? "mediaFailed"
      : job?.reason
        ? reasons[job.reason] || "mediaFailed"
        : states[job?.state || "missing"];
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
      {enabled ? (
        <p role={detail ? "status" : undefined}>{t(message)}</p>
      ) : (
        detail && <p>{t("previewUnavailable")}</p>
      )}
      {detail && enabled && (
        <>
          {busy && (
            <Button
              disabled={action.isPending}
              onClick={() => action.mutate("cancel")}
            >
              {t("cancel")}
            </Button>
          )}
          {terminal && (
            <Button
              disabled={action.isPending}
              onClick={() => action.mutate("retry")}
            >
              {t("retry")}
            </Button>
          )}
          {(query.isError || failed) && (
            <Button
              disabled={query.isFetching}
              onClick={() => {
                setFailedGeneration("");
                void query.refetch();
              }}
            >
              {t("retry")}
            </Button>
          )}
          {action.isError && <p role="alert">{t("mediaRequestFailed")}</p>}
        </>
      )}
    </div>
  );
}
