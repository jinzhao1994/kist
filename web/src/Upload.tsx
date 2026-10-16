import { UploadPasteDialog } from "./UploadPasteDialog";
import { UploadPanel } from "./UploadPanel";
import { Button } from "@mantine/core";
import {
  useEffect,
  useRef,
  useState,
  useImperativeHandle,
  type Ref,
  type ReactNode,
} from "react";
import { useUploadDropzone } from "./useUploadDropzone";
import { useQueryClient } from "@tanstack/react-query";
import { api, APIError, createUpload } from "./api";
import { formatBytes, formatNumber, useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import { ResumableUpload } from "./ResumableUpload";
export type UploadHandle = { open: () => void };
type UploadProps = {
  pickerRef?: Ref<UploadHandle>;
  children?: ReactNode;
  compact?: boolean;
  box: string;
  folder?: string;
  maxFiles: number;
  maxBytes: number;
  resumableVersion?: number;
  chunkBytes?: number;
  onComplete: () => Promise<void>;
};
export function Upload(props: UploadProps) {
  let legacyPending = false;
  try {
    legacyPending = !!sessionStorage.getItem(
      `kist.pending-upload.v1:${props.box}${props.folder ? `:${props.folder}` : ""}`,
    );
  } catch {
    /* Preserve the legacy fallback. */
  }
  if (
    !legacyPending &&
    (props.resumableVersion === 1 || props.resumableVersion === 2) &&
    props.folder &&
    props.chunkBytes
  )
    return (
      <ResumableUpload
        {...props}
        folder={props.folder}
        chunkBytes={props.chunkBytes}
      />
    );
  return <LegacyUpload {...props} />;
}
function LegacyUpload({
  pickerRef,
  children,
  compact,
  box,
  folder,
  maxFiles,
  maxBytes,
  onComplete,
}: UploadProps) {
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
  const [phase, setPhase] = useState<
    "idle" | "hashing" | "uploading" | "processing" | "failed" | "saved"
  >(pendingKey ? "failed" : "idle");
  const busy =
    phase === "hashing" || phase === "uploading" || phase === "processing";
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
    setPhase("hashing");
    setError("");
    setProgress(0);
    try {
      await operation.current.run(setProgress, setPhase);
      await finish();
    } catch (error) {
      setPhase("failed");
      setError(
        error instanceof APIError && error.status === 413
          ? t("uploadTooLarge")
          : error instanceof APIError &&
              error.body.code === "upload_integrity_mismatch"
            ? t("uploadIntegrityMismatch")
            : error instanceof APIError
              ? errorMessage(error, t)
              : error instanceof DOMException && error.name === "AbortError"
                ? t("uploadCancelled")
                : t("uploadFailed"),
      );
    }
  }
  const drop = useUploadDropzone({
    noClick: true,
    noKeyboard: true,
    multiple: true,
    disabled: busy || phase === "failed",
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
  useImperativeHandle(pickerRef, () => ({ open: drop.open }), [drop.open]);
  const showPanel =
    !compact || busy || !!pendingKey || !!error || phase === "failed";
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
        className: `${compact ? "file-dropzone" : "dropzone"} ${drop.isDragActive ? "drag-active" : ""}`,
        role: "region",
        "aria-label": t("upload"),
        // Disable file intake without disabling cancel/retry controls inside.
        "aria-disabled": undefined,
      })}
    >
      <input {...drop.getInputProps({ "aria-label": t("chooseFiles") })} />
      <UploadPanel
        visible={showPanel}
        title={t(
          busy
            ? phase === "hashing"
              ? "uploadHashing"
              : phase === "processing"
                ? "uploadProcessing"
                : "uploading"
            : phase === "saved"
              ? "uploadComplete"
              : "upload",
        )}
        progress={busy ? (phase === "processing" ? null : progress) : undefined}
        actions={
          <>
            {busy ? (
              <>
                <Button size="xs" onClick={() => operation.current?.cancel()}>
                  {t("cancel")}
                </Button>
              </>
            ) : (
              <>
                <Button
                  size="xs"
                  variant="filled"
                  disabled={phase === "failed"}
                  onClick={drop.open}
                >
                  {t("chooseFiles")}
                </Button>
                {phase === "failed" && (
                  <>
                    {operation.current && (
                      <Button size="xs" onClick={run}>
                        {t("retry")}
                      </Button>
                    )}
                    <Button size="xs" onClick={check}>
                      {t("checkResult")}
                    </Button>
                    <Button
                      size="xs"
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
          </>
        }
      >
        {!busy && (
          <p className="upload-hint">
            {t("uploadLimit", {
              count: formatNumber(maxFiles, prefs),
              size: formatBytes(maxBytes, prefs),
            })}
          </p>
        )}
        {phase === "failed" && !error && (
          <p role="status">{t("uploadFailed")}</p>
        )}
        {error && (
          <p role="alert" className="upload-error">
            {error}
          </p>
        )}
      </UploadPanel>
      <UploadPasteDialog
        files={drop.pasteFiles}
        confirm={drop.confirmPaste}
        cancel={drop.cancelPaste}
      />
      {children}
    </section>
  );
}
