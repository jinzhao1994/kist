import { Button } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useDropzone } from "react-dropzone";
import { useQueryClient } from "@tanstack/react-query";
import { api, APIError, createUpload } from "./api";
import { formatBytes, formatNumber, useI18n } from "./i18n";
import { errorMessage } from "./mutations";
export function Upload({
  box,
  folder,
  maxFiles,
  maxBytes,
  onComplete,
}: {
  box: string;
  folder?: string;
  maxFiles: number;
  maxBytes: number;
  onComplete: () => Promise<void>;
}) {
  const { t, prefs } = useI18n();
  const client = useQueryClient();
  const storage = `kist.pending-upload.v1:${box}${folder ? `:${folder}` : ""}`;
  const [pendingKey, setPendingKey] = useState(() => {
    try {
      return sessionStorage.getItem(storage) || "";
    } catch {
      return "";
    }
  });
  function remember(key: string) {
    setPendingKey(key);
    try {
      if (key) sessionStorage.setItem(storage, key);
      else sessionStorage.removeItem(storage);
    } catch {
      /* The live upload still retains its request key. */
    }
  }
  const operation = useRef<ReturnType<typeof createUpload> | null>(null);
  const [phase, setPhase] = useState<"idle" | "uploading" | "failed" | "saved">(
    pendingKey ? "failed" : "idle",
  );
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => () => operation.current?.cancel(), []);
  async function finish() {
    remember("");
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
          : error instanceof APIError
            ? errorMessage(error, t)
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
    disabled: phase === "uploading" || phase === "failed",
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
      operation.current = createUpload(box, files, folder);
      remember(operation.current.key);
      void run();
    },
  });
  async function check() {
    if (!pendingKey) return;
    try {
      await api.receipt(pendingKey);
      await finish();
    } catch (error) {
      setError(
        error instanceof APIError && error.status === 404
          ? t("requestPending")
          : errorMessage(error, t),
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
        <p>{t(folder ? "uploadToFolder" : "importHint")}</p>
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
            <Button onClick={() => operation.current?.cancel()}>
              {t("cancel")}
            </Button>
          </>
        ) : (
          <>
            <Button
              variant="filled"
              disabled={phase === "failed"}
              onClick={drop.open}
            >
              {t("chooseFiles")}
            </Button>
            {phase === "failed" && (
              <>
                {operation.current && (
                  <Button onClick={run}>{t("retry")}</Button>
                )}
                <Button onClick={check}>{t("checkResult")}</Button>
                <Button
                  onClick={() => {
                    remember("");
                    operation.current = null;
                    setPhase("idle");
                    setError("");
                  }}
                >
                  {t("newOperation")}
                </Button>
              </>
            )}
          </>
        )}
      </div>
      {phase === "failed" && !error && <p role="status">{t("uploadFailed")}</p>}
      {error && (
        <p role="alert" className="upload-error">
          {error}
        </p>
      )}
    </section>
  );
}
