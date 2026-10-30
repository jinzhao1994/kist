import { Button } from "@mantine/core";
import { useEffect, useRef, useState, type RefObject } from "react";
import type { HlsConfig } from "hls.js";
import hlsWorkerURL from "hls.js/dist/hls.worker.js?url";
import type { SubtitleTrack } from "./Media";
import { useI18n } from "./i18n";

// The media engine is loaded only for active video details. No fragments are
// requested before play; pausing aborts pending fragment reads and unmounting
// releases MSE, network requests, buffers and the native media element.
export function useHLS(
  ref: RefObject<HTMLVideoElement | null>,
  src: string | undefined,
  enabled: boolean,
  onError: () => void,
  onReady?: () => void,
) {
  const callbacks = useRef({ onError, onReady });
  callbacks.current = { onError, onReady };
  useEffect(() => {
    const video = ref.current;
    if (!video || !enabled || !src) return;
    let disposed = false;
    let cleanup = () => {};
    const failed = () => {
      if (!disposed) callbacks.current.onError();
    };
    void import("hls.js")
      .then(({ default: Hls }) => {
        if (disposed) return;
        if (Hls.isSupported()) {
          const policy = (timeout: number) => ({
            default: {
              maxTimeToFirstByteMs: timeout,
              maxLoadTimeMs: timeout,
              timeoutRetry: null,
              errorRetry: null,
            },
          });
          const config: Partial<HlsConfig> = {
            workerPath: hlsWorkerURL,
            autoStartLoad: false,
            maxBufferLength: 12,
            maxMaxBufferLength: 12,
            backBufferLength: 6,
            maxBufferSize: 12 << 20,
            fragLoadPolicy: policy(95_000),
            manifestLoadPolicy: policy(35_000),
            playlistLoadPolicy: policy(35_000),
          };
          const hls = new Hls(config);
          const play = () => hls.startLoad(video.currentTime);
          const pause = () => hls.stopLoad();
          const seek = () => {
            if (!video.paused) hls.startLoad(video.currentTime);
          };
          hls.on(Hls.Events.ERROR, (_, data) => {
            if (data.fatal) {
              hls.stopLoad();
              failed();
            }
          });
          hls.on(Hls.Events.MEDIA_ATTACHED, () => hls.loadSource(src));
          hls.on(Hls.Events.MANIFEST_PARSED, () => {
            callbacks.current.onReady?.();
            if (!video.paused) play();
          });
          video.addEventListener("play", play);
          video.addEventListener("pause", pause);
          video.addEventListener("seeking", seek);
          hls.attachMedia(video);
          cleanup = () => {
            video.removeEventListener("play", play);
            video.removeEventListener("pause", pause);
            video.removeEventListener("seeking", seek);
            hls.destroy();
          };
        } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
          video.src = src;
          callbacks.current.onReady?.();
        } else failed();
      })
      .catch(failed);
    return () => {
      disposed = true;
      cleanup();
      video.pause();
      video.removeAttribute("src");
      video.load();
    };
  }, [ref, src, enabled]);
}

export default function HLSPlayer({
  src,
  name,
  tracks,
}: {
  src: string;
  name: string;
  tracks?: SubtitleTrack[];
}) {
  const { t } = useI18n();
  const [attempt, setAttempt] = useState(0);
  const [failed, setFailed] = useState(false);
  return failed ? (
    <div className="media-placeholder">
      <p role="status">{t("mediaFailed")}</p>
      <Button
        onClick={() => {
          setAttempt(attempt + 1);
          setFailed(false);
        }}
      >
        {t("retry")}
      </Button>
    </div>
  ) : (
    <HLSVideo
      key={attempt}
      src={src}
      name={name}
      tracks={tracks}
      fail={() => setFailed(true)}
    />
  );
}

function HLSVideo({
  src,
  name,
  tracks,
  fail,
}: {
  src: string;
  name: string;
  tracks?: SubtitleTrack[];
  fail: () => void;
}) {
  const video = useRef<HTMLVideoElement>(null);
  useHLS(video, src, true, fail);
  return (
    <video
      ref={video}
      className="detail-image"
      controls
      playsInline
      preload="none"
      aria-label={name}
      onError={fail}
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
  );
}
