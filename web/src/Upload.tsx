import { useEffect, useRef, useState } from "react";
import { useDropzone } from "react-dropzone";
import { useQueryClient } from "@tanstack/react-query";
import { api, APIError, createUpload } from "./api";
import { formatBytes, formatNumber, useI18n } from "./i18n";
export function Upload({
  box,
  maxFiles,
  maxBytes,
  onComplete,
}: {
  box: string;
  maxFiles: number;
  maxBytes: number;
  onComplete: () => Promise<void>;
}) {
  const { t, prefs } = useI18n();
  const client = useQueryClient();
  const operation = useRef<ReturnType<typeof createUpload> | null>(null);
  const [phase, setPhase] = useState<"idle" | "uploading" | "failed" | "saved">(
    "idle",
  );
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => () => operation.current?.cancel(), []);
  async function finish() {
    setPhase("saved");
    setError("");
    await client.invalidateQueries();
    try {
      await onComplete();
    } catch {
      setError(t("extractionFailed"));
    }
  }
  async function run() {
    if (!operation.current) return;
    setPhase("uploading");
    setError("");
    setProgress(0);
    try {
      await operation.current.run(setProgress);
      await finish();
    } catch (error) {
      setPhase("failed");
      setError(
        error instanceof APIError && error.status === 413
          ? t("uploadTooLarge")
          : error instanceof DOMException && error.name === "AbortError"
            ? t("uploadCancelled")
            : t("uploadFailed"),
      );
    }
  }
  const drop = useDropzone({
    noClick: true,
    noKeyboard: true,
    multiple: true,
    disabled: phase === "uploading",
    onDrop: (files, rejected) => {
      if (
        rejected.length ||
        files.length > maxFiles ||
        files.reduce((sum, file) => sum + file.size, 0) > maxBytes
      ) {
        setError(t("uploadTooLarge"));
        return;
      }
      if (!files.length) return;
      operation.current = createUpload(box, files);
      void run();
    },
  });
  async function check() {
    if (!operation.current) return;
    try {
      await api.receipt(operation.current.key);
      await finish();
    } catch (error) {
      setError(
        error instanceof APIError && error.status === 404
          ? t("requestPending")
          : t("error"),
      );
    }
  }
  return (
    <section
      {...drop.getRootProps({
        className: `dropzone ${drop.isDragActive ? "drag-active" : ""}`,
        role: "region",
        "aria-label": t("upload"),
      })}
    >
      <input {...drop.getInputProps({ "aria-label": t("chooseFiles") })} />
      <div>
        <strong>{t(phase === "saved" ? "uploadComplete" : "upload")}</strong>
        <p>{t("importHint")}</p>
        <small>
          {t("uploadLimit", {
            count: formatNumber(maxFiles, prefs),
            size: formatBytes(maxBytes, prefs),
          })}
        </small>
      </div>
      <div className="upload-actions">
        {phase === "uploading" ? (
          <>
            <progress value={progress} max={1} aria-label={t("uploading")} />
            <span role="status">
              {t("uploadProgress", {
                percent: formatNumber(Math.round(progress * 100), prefs),
              })}
            </span>
            <button onClick={() => operation.current?.cancel()}>
              {t("cancel")}
            </button>
          </>
        ) : (
          <>
            <button className="primary" onClick={drop.open}>
              {t("chooseFiles")}
            </button>
            {phase === "failed" && (
              <>
                <button onClick={run}>{t("retry")}</button>
                <button onClick={check}>{t("checkResult")}</button>
              </>
            )}
          </>
        )}
      </div>
      {error && (
        <p role="alert" className="upload-error">
          {error}
        </p>
      )}
    </section>
  );
}
