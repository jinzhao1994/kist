import { Dialog, DialogPresence } from "./Dialog";
import { UploadPanel } from "./UploadPanel";
import type { UploadHandle } from "./Upload";
import { Button, Text } from "@mantine/core";
import {
  useEffect,
  useRef,
  useState,
  useImperativeHandle,
  type Ref,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useDropzone } from "react-dropzone";
import { APIError } from "./api";
import { formatBytes, formatNumber, useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import {
  resumableUpload,
  ResumeError,
  UploadConflictError,
  type UploadConflictMode,
  type PendingUpload,
  type UploadPhase,
  type UploadMetrics,
} from "./resumable-upload";

export function ResumableUpload({
  pickerRef,
  children,
  compact,
  box,
  folder,
  chunkBytes,
  resumableVersion,
  maxFiles,
  maxBytes,
  onComplete,
}: {
  pickerRef?: Ref<UploadHandle>;
  children?: ReactNode;
  compact?: boolean;
  box: string;
  folder: string;
  chunkBytes: number;
  resumableVersion?: number;
  maxFiles: number;
  maxBytes: number;
  onComplete: () => Promise<void>;
}) {
  const { t, prefs } = useI18n();
  const client = useQueryClient();
  const mounted = useRef(true);
  const [pending, setPending] = useState<PendingUpload>();
  const [setup] = useState(() => {
    try {
      return {
        operation: resumableUpload(
          box,
          folder,
          chunkBytes,
          (value) => {
            if (mounted.current) setPending(value);
          },
          resumableVersion,
        ),
      };
    } catch (error) {
      return { error };
    }
  });
  const operation = setup.operation;
  const [phase, setPhase] = useState<
    UploadPhase | "idle" | "checking" | "paused" | "failed" | "saved"
  >(operation?.pending() ? "paused" : "idle");
  const [conflict, setConflict] = useState<UploadConflictError>();
  const [metrics, setMetrics] = useState<UploadMetrics>();
  const [dismissed, setDismissed] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  const running = useRef<Promise<void>>(undefined);
  const activePending = pending ?? operation?.pending();
  const busy = ["checking", "hashing", "uploading", "processing"].includes(
    phase,
  );

  function message(error: unknown) {
    if (error instanceof ResumeError) {
      return t(
        error.kind === "source"
          ? "uploadWrongSource"
          : error.kind === "storage"
            ? "uploadStorageUnavailable"
            : error.kind === "selection"
              ? "uploadReselectHint"
              : "uploadProtocolError",
      );
    }
    if (error instanceof APIError) {
      switch (error.body.code) {
        case "upload_integrity_mismatch":
        case "upload_chunk_integrity":
          return t("uploadIntegrityMismatch");
        case "upload_session_expired":
        case "upload_session_missing":
        case "upload_session_canceled":
          return t("uploadExpired");
        case "upload_target_changed":
        case "box_not_found":
        case "folder_in_another_box":
          return t("uploadTargetChanged");
        case "storage_full":
          return t("uploadStorageFull");
        case "upload_quota":
          return t("uploadSessionQuota");
        case "upload_session_corrupt":
        case "upload_session_version":
          return t("uploadSessionDamaged");
      }
      if (error.status === 413) return t("uploadTooLarge");
      return errorMessage(error, t);
    }
    return t("uploadResumeFailed");
  }
  async function finish() {
    if (!mounted.current) return;
    setPhase("saved");
    setError("");
    setPending(undefined);
    await client.invalidateQueries();
    try {
      await onComplete();
    } catch {
      if (mounted.current) setError(t("extractionFailed"));
    }
  }
  async function run() {
    if (!operation) return;
    setPhase("hashing");
    setMetrics(undefined);
    setDismissed(false);
    setError("");
    try {
      await operation.run(
        (value) => {
          if (mounted.current) setProgress(value);
        },
        (value) => {
          if (mounted.current) setPhase(value);
        },
        (value) => {
          if (mounted.current) setMetrics(value);
        },
      );
      await finish();
    } catch (error) {
      if (!mounted.current) return;
      setProgress(operation.confirmedProgress());
      if (error instanceof UploadConflictError) {
        setConflict(error);
        setPhase("paused");
        setError("");
      } else if (error instanceof DOMException && error.name === "AbortError") {
        setPhase("paused");
        setError("");
      } else {
        setPhase("failed");
        setError(message(error));
      }
    }
  }
  function start() {
    running.current = run();
  }
  async function check() {
    if (!operation) return;
    setPhase("checking");
    setError("");
    try {
      const result = await operation.check();
      if (result.receipt) {
        await finish();
        return;
      }
      if (!mounted.current) return;
      if (result.state) {
        const total = result.state.manifest.files.reduce(
          (sum, file) => sum + file.size,
          0,
        );
        setProgress(
          total
            ? result.state.offsets.reduce((sum, value) => sum + value, 0) /
                total
            : 1,
        );
      }
      setPhase("paused");
    } catch (error) {
      if (mounted.current) {
        if (error instanceof UploadConflictError) {
          setConflict(error);
          setPhase("paused");
          setError("");
        } else {
          setPhase("failed");
          setError(message(error));
        }
      }
    }
  }
  async function cancel() {
    if (!operation) return;
    operation.pause();
    await running.current;
    if (!mounted.current) return;
    setPhase("checking");
    setError("");
    try {
      const receipt = await operation.cancel();
      if (receipt) {
        await finish();
        return;
      }
      if (mounted.current) {
        setPhase("idle");
        setProgress(0);
        setPending(undefined);
      }
    } catch (error) {
      if (mounted.current) {
        setPhase("failed");
        setError(message(error));
      }
    }
  }
  useEffect(() => {
    mounted.current = true;
    let stopped = false;
    // React's development remount must not start two concurrent status checks.
    queueMicrotask(() => {
      if (!stopped && operation?.pending()?.files) void check();
    });
    return () => {
      stopped = true;
      mounted.current = false;
      operation?.pause();
    };
  }, []);

  const drop = useDropzone({
    noClick: true,
    noKeyboard: true,
    multiple: true,
    disabled: busy || !!conflict || !operation,
    onDrop: (files, rejected) => {
      if (
        rejected.length ||
        files.length > maxFiles ||
        files.reduce((sum, file) => sum + file.size, 0) > maxBytes
      ) {
        setError(t("uploadTooLarge"));
        return;
      }
      if (!files.length || !operation) return;
      operation.select(files);
      start();
    },
  });
  useImperativeHandle(pickerRef, () => ({ open: drop.open }), [drop.open]);
  function chooseConflict(mode: UploadConflictMode) {
    if (!operation || !conflict) return;
    try {
      operation.resolveConflict(mode, conflict.conflicts.token);
      setConflict(undefined);
      start();
    } catch (error) {
      setError(message(error));
      setConflict(undefined);
      setPhase("failed");
    }
  }
  const showPanel =
    !compact || busy || !!activePending || !!error || !!setup.error;
  const label =
    phase === "hashing"
      ? t("uploadHashing")
      : phase === "processing"
        ? t("uploadProcessing")
        : phase === "checking"
          ? t("uploadChecking")
          : t("uploading");
  return (
    <section
      {...drop.getRootProps({
        className: `${compact ? "file-dropzone" : "dropzone"} ${drop.isDragActive ? "drag-active" : ""}`,
        role: "region",
        "aria-label": t("upload"),
        "aria-disabled": undefined,
      })}
    >
      <input
        {...drop.getInputProps({
          "aria-label": activePending ? t("uploadReselect") : t("chooseFiles"),
        })}
      />
      <DialogPresence>
        {conflict && (
          <Dialog
            open
            onOpenChange={(open) => {
              if (!open) {
                setConflict(undefined);
                void cancel();
              }
            }}
            title={t("uploadConflictTitle")}
            size="sm"
          >
            <Text size="sm" c="dimmed">
              {t("uploadConflictHint")}
            </Text>
            <ul className="upload-conflict-files">
              {conflict.conflicts.paths?.map((path) => (
                <li key={path}>{path}</li>
              ))}
            </ul>
            {!conflict.conflicts.can_overwrite && (
              <p>{t("uploadFolderConflict")}</p>
            )}
            <div className="actions">
              <Button
                variant="subtle"
                onClick={() => {
                  setConflict(undefined);
                  void cancel();
                }}
              >
                {t("cancel")}
              </Button>
              <Button
                disabled={!conflict.conflicts.can_overwrite}
                onClick={() => chooseConflict("overwrite")}
              >
                {t("uploadOverwrite")}
              </Button>
              <Button
                variant="filled"
                onClick={() => chooseConflict("keep_both")}
              >
                {t("uploadKeepBoth")}
              </Button>
            </div>
          </Dialog>
        )}
      </DialogPresence>
      <UploadPanel
        visible={showPanel}
        title={
          busy ? label : t(phase === "saved" ? "uploadComplete" : "upload")
        }
        progress={
          busy
            ? phase === "processing" || phase === "checking"
              ? null
              : progress
            : undefined
        }
        progressLabel={label}
        actions={
          <>
            {busy ? (
              <>
                <Button
                  size="xs"
                  disabled={phase === "checking"}
                  onClick={() => operation?.pause()}
                >
                  {t("uploadPause")}
                </Button>
              </>
            ) : (
              <>
                <Button
                  size="xs"
                  variant="filled"
                  disabled={!operation}
                  onClick={drop.open}
                >
                  {activePending ? t("uploadReselect") : t("chooseFiles")}
                </Button>
                {activePending && (
                  <>
                    {operation?.hasFiles() && (
                      <Button size="xs" onClick={start}>
                        {t("uploadResume")}
                      </Button>
                    )}
                    {activePending.files && (
                      <Button size="xs" onClick={() => void check()}>
                        {t("checkResult")}
                      </Button>
                    )}
                  </>
                )}
              </>
            )}
            {activePending && (
              <Button
                size="xs"
                disabled={phase === "checking"}
                onClick={() => void cancel()}
              >
                {t("cancel")}
              </Button>
            )}
          </>
        }
      >
        {!busy && !activePending && (
          <p className="upload-hint">
            {t("uploadLimit", {
              count: formatNumber(maxFiles, prefs),
              size: formatBytes(maxBytes, prefs),
            })}
          </p>
        )}
        {metrics && phase !== "saved" && (
          <p className="upload-live-metrics">
            {t("uploadCurrentStats", {
              seconds: formatNumber(Math.round(metrics.totalMs / 1000), prefs),
              speed: formatBytes(
                metrics.transferMs > 0
                  ? (metrics.bytes * 1000) / metrics.transferMs
                  : 0,
                prefs,
              ),
            })}
          </p>
        )}
        {activePending && !busy && (
          <p role="status">
            {t("uploadPaused", {
              percent: formatNumber(Math.round(progress * 100), prefs),
            })}
          </p>
        )}
        {activePending && !busy && !operation?.hasFiles() && (
          <p>{t("uploadReselectHint")}</p>
        )}
        {activePending && <small>{t("uploadBackgroundHint")}</small>}
        {!!(error || setup.error) && (
          <p role="alert" className="upload-error">
            {error || message(setup.error)}
          </p>
        )}
      </UploadPanel>
      {phase === "saved" && metrics && !dismissed && (
        <div className="upload-result">
          <details className="upload-timing">
            <summary>
              <span>
                {t("uploadElapsed", {
                  seconds: formatNumber(
                    Math.round(metrics.totalMs / 1000),
                    prefs,
                  ),
                  speed: formatBytes(
                    metrics.transferMs > 0
                      ? (metrics.bytes * 1000) / metrics.transferMs
                      : 0,
                    prefs,
                  ),
                })}
              </span>
            </summary>
            <small>
              {t("uploadTiming", {
                hash: formatNumber(Math.round(metrics.hashMs / 1000), prefs),
                transfer: formatNumber(
                  Math.round(metrics.transferMs / 1000),
                  prefs,
                ),
                processing: formatNumber(
                  Math.round(metrics.processingMs / 1000),
                  prefs,
                ),
              })}
            </small>
          </details>
          <button
            type="button"
            className="upload-result-dismiss"
            aria-label={t("close")}
            onClick={() => setDismissed(true)}
          >
            ×
          </button>
        </div>
      )}
      {children}
    </section>
  );
}
