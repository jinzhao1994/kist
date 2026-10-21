import livePhotoIcon from "./assets/live-photo.svg?no-inline";
import { Button } from "@mantine/core";
import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
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

import { TextPreview } from "./TextPreview";
function isSpreadsheet(file: ArchiveFile) {
  return /\.(xlsx?|csv|tsv)$/i.test(file.name);
}
const SheetPreview = lazy(() => import("./SheetPreview"));

const PDFReader = lazy(() => import("./PDFReader"));
const GPXPreview = lazy(() => import("./GPXPreview"));
function isGPX(file: ArchiveFile) {
  return file.media_type === "application/gpx+xml" || /\.gpx$/i.test(file.name);
}

function isSTL(file: ArchiveFile) {
  return file.media_type === "model/stl" || /\.stl$/i.test(file.name);
}

export function isWord(file: ArchiveFile) {
  return (
    /\.docx?$/i.test(file.name) ||
    file.media_type === "application/msword" ||
    file.media_type ===
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
  );
}

function isPresentation(file: ArchiveFile) {
  return (
    /\.pptx?$/i.test(file.name) ||
    file.media_type === "application/vnd.ms-powerpoint" ||
    file.media_type ===
      "application/vnd.openxmlformats-officedocument.presentationml.presentation"
  );
}
function isOffice(file: ArchiveFile) {
  return isWord(file) || isPresentation(file);
}
export function usesPDFReader(file: ArchiveFile) {
  return file.media_type === "application/pdf" || isOffice(file);
}
function officePDFRecipe(file: ArchiveFile) {
  return isPresentation(file) ? "slides_pdf_v1" : "office_pdf_v1";
}
function mediaKey(file: ArchiveFile, recipe: string) {
  return ["media", file.id, file.blob_key, recipe];
}

// Keep this aligned with the content endpoint's passive image allowlist.
const originalImageTypes = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
  "image/x-icon",
]);
function usesOriginalImage(file: ArchiveFile) {
  return file.kind === "image" && originalImageTypes.has(file.media_type || "");
}
function OriginalImage({ file }: { file: ArchiveFile }) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  return failed ? (
    <div className="media-placeholder">
      <p role="status">{t("imageLoadFailed")}</p>
      <Button onClick={() => setFailed(false)}>{t("retry")}</Button>
    </div>
  ) : (
    <img
      className="detail-image"
      src={contentURL(file)}
      alt={file.name}
      decoding="async"
      onError={() => setFailed(true)}
    />
  );
}

export function OpenMediaButton({ file }: { file: ArchiveFile }) {
  const { t } = useI18n();
  const officePDF = officePDFRecipe(file);
  // Media owns requests and polling. This observer follows the same cache entry.
  const { data: job } = useQuery<MediaJob>({
    queryKey: mediaKey(file, officePDF),
    enabled: false,
  });
  const original = file.media_type === "application/pdf";
  const image = usesOriginalImage(file);
  if (!image && !original && !(isOffice(file) && job?.state === "ready"))
    return null;
  const href =
    original || image
      ? contentURL(file)
      : `/api/v1${mediaPath(file, officePDF)}/content?v=${encodeURIComponent(job!.generation)}`;
  return (
    <Button component="a" href={href} target="_blank" rel="noopener noreferrer">
      {t(image ? "openImage" : "openPDF")}
    </Button>
  );
}

export function hasDerivedMedia(file: ArchiveFile) {
  return (
    file.kind === "image" ||
    file.kind === "video" ||
    file.media_type === "application/pdf" ||
    isSTL(file) ||
    isGPX(file) ||
    isOffice(file)
  );
}
const audioTypes = new Set([
  "audio/mpeg",
  "audio/wave",
  "audio/aiff",
  "audio/ogg",
]);
export function usesTextPreview(file: ArchiveFile) {
  return (
    !hasDerivedMedia(file) &&
    !["image", "video", "audio"].includes(file.kind || "") &&
    !audioTypes.has(file.media_type || "")
  );
}
export function hasPreview(file: ArchiveFile) {
  return hasDerivedMedia(file) || usesTextPreview(file);
}
const reasons: Record<string, MessageKey> = {
  dependency_missing: "mediaDependency",
  not_cached: "mediaNotCached",
  cache_quota: "mediaCacheQuota",
  cache_metadata: "mediaCacheMetadata",
  blob_integrity_mismatch: "mediaCacheCorrupt",
  sync_version: "mediaSyncVersion",
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

export type SubtitleTrack = {
  src: string;
  label: string;
  language: string;
  default?: boolean;
};

function LivePhotoPlayer({
  photo,
  src,
  retry,
}: {
  photo: ArchiveFile;
  src?: string;
  retry?: () => void;
}) {
  const { t } = useI18n();
  const surface = useRef<HTMLDivElement>(null);
  const [inset, setInset] = useState({ x: 0, y: 0 });
  const alignButton = useCallback(() => {
    const node = surface.current;
    const image = node?.querySelector("img");
    if (!node || !image?.naturalWidth || !image.naturalHeight) return;
    const scale = Math.min(
      node.clientWidth / image.naturalWidth,
      node.clientHeight / image.naturalHeight,
    );
    setInset({
      x: (node.clientWidth - image.naturalWidth * scale) / 2,
      y: (node.clientHeight - image.naturalHeight * scale) / 2,
    });
  }, []);
  useEffect(() => {
    const observer = new ResizeObserver(alignButton);
    if (surface.current) observer.observe(surface.current);
    alignButton();
    return () => observer.disconnect();
  }, [alignButton]);
  const video = useRef<HTMLVideoElement>(null);
  const requested = useRef(false);
  const attempt = useRef(0);
  const [playing, setPlaying] = useState(false);
  const [waiting, setWaiting] = useState(false);
  const [failed, setFailed] = useState(false);
  const stop = () => {
    attempt.current++;
    requested.current = false;
    video.current?.pause();
    if (video.current) video.current.currentTime = 0;
    setPlaying(false);
    setWaiting(false);
  };
  useEffect(() => {
    const element = video.current;
    return () => {
      requested.current = false;
      element?.pause();
      element?.removeAttribute("src");
      element?.load();
    };
  }, []);
  const play = () => {
    if (requested.current) {
      stop();
      return;
    }
    if (!src) {
      retry?.();
      return;
    }
    requested.current = true;
    const currentAttempt = ++attempt.current;
    setWaiting(true);
    setFailed(false);
    const element = video.current!;
    if (failed) element.load();
    element.currentTime = 0;
    void element.play().catch(() => {
      if (!requested.current || currentAttempt !== attempt.current) return;
      stop();
      setFailed(true);
    });
  };
  return (
    <div
      ref={surface}
      onLoadCapture={alignButton}
      className={`live-photo${playing ? " is-playing" : ""}`}
    >
      <div className="live-photo-still">
        <Media file={photo} detail />
      </div>
      <video
        ref={video}
        src={src}
        preload="auto"
        playsInline
        aria-hidden="true"
        onPlaying={() => {
          if (!requested.current) {
            video.current?.pause();
            return;
          }
          setPlaying(true);
          setWaiting(false);
        }}
        onEnded={stop}
        onError={() => {
          stop();
          setFailed(true);
        }}
      />
      <button
        type="button"
        className="live-photo-button"
        style={{ insetInlineStart: inset.x + 16, bottom: inset.y + 16 }}
        onClick={play}
        aria-label={t(playing ? "filesetStill" : "filesetMotion")}
        aria-pressed={playing}
        disabled={!src && !retry}
        title={
          failed || retry
            ? t("retry")
            : !src || waiting
              ? t("loading")
              : undefined
        }
      >
        <span
          className="live-photo-icon"
          aria-hidden="true"
          style={{
            maskImage: `url(${livePhotoIcon})`,
            WebkitMaskImage: `url(${livePhotoIcon})`,
          }}
        />
        {t("filesetLive")}
        {((!src && !retry) || waiting) && (
          <span className="live-photo-loading" aria-hidden="true">
            ···
          </span>
        )}
      </button>
      {failed && (
        <span className="live-photo-error" role="status">
          {t("mediaFailed")}
        </span>
      )}
    </div>
  );
}

export function Media({
  file,
  active = true,
  detail = false,
  paused = false,
  tracks,
  livePhoto,
}: {
  file: ArchiveFile;
  tracks?: SubtitleTrack[];
  livePhoto?: ArchiveFile;
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
  const recipe = isOffice(file)
    ? detail
      ? officePDFRecipe(file)
      : isPresentation(file)
        ? "slides_thumb_512_v1"
        : "office_thumb_512_v1"
    : isGPX(file)
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
  const key = mediaKey(file, recipe);
  const readerPDF = detail && file.media_type === "application/pdf";
  const originalImage = detail && usesOriginalImage(file);
  const enabled =
    active && hasDerivedMedia(file) && !readerPDF && !originalImage;
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
  if (isSpreadsheet(file))
    return (
      <Suspense
        fallback={<div className="media-placeholder">{t("loading")}</div>}
      >
        <SheetPreview
          key={`${file.id}:${file.blob_key}`}
          file={file}
          detail={detail}
          active={active}
          paused={paused}
        />
      </Suspense>
    );
  if (usesTextPreview(file))
    return (
      <TextPreview
        key={`${file.id}:${file.blob_key}`}
        file={file}
        active={active}
        detail={detail}
        paused={paused}
      />
    );
  if (originalImage && active)
    return <OriginalImage key={`${file.id}:${file.blob_key}`} file={file} />;
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
  if (livePhoto && detail && file.kind === "video") {
    const retryable =
      query.isError ||
      failed ||
      (job && ["failed", "cancelled", "blocked"].includes(job.state));
    return (
      <LivePhotoPlayer
        key={`${file.id}:${file.blob_key}`}
        photo={livePhoto}
        src={
          job?.state === "ready" && !failed
            ? `/api/v1${path}/content?v=${encodeURIComponent(job.generation)}`
            : undefined
        }
        retry={
          retryable
            ? () => {
                setFailedGeneration("");
                if (query.isError || failed) void query.refetch();
                else action.mutate("retry");
              }
            : undefined
        }
      />
    );
  }
  if (enabled && job?.state === "ready" && !failed) {
    const src = `/api/v1${path}/content?v=${encodeURIComponent(job.generation)}`;
    const fail = () => {
      setFailedGeneration(job.generation);
      void query.refetch();
    };
    if (detail && isOffice(file))
      return (
        <Suspense
          fallback={<div className="media-placeholder">{t("loading")}</div>}
        >
          <PDFReader key={src} src={src} name={file.name} />
        </Suspense>
      );
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
      >
        {tracks?.map((track) => (
          <track
            key={track.src}
            kind="subtitles"
            src={track.src}
            label={track.label}
            srcLang={track.language}
            default={track.default}
          />
        ))}
      </video>
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
