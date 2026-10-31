export { AIConnection } from "./AIServices";
import {
  settingsQueries,
  aiFeatureAccess,
  type MetadataTask,
} from "./settingsQueries";
import {
  TaskError,
  TaskStatus,
  taskPollInterval,
  taskErrorMessage,
} from "./TaskStatus";
import { CopyTextPreview } from "./CopyTextPreview";
import { Button } from "@mantine/core";
import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, result } from "./api";
import { useI18n, type MessageKey } from "./i18n";
import { errorMessage } from "./mutations";

type OCRJob = {
  manual?: boolean;
  state: "missing" | "queued" | "running" | "ready" | "failed";
  error?: string;
};
export type ImageTask = Exclude<MetadataTask, "geo" | "metadata">;
const taskMessage = (kind: MetadataTask, suffix: string) =>
  `${kind}${suffix}` as MessageKey;
async function post<T>(path: string, body: unknown): Promise<T> {
  return result<T>(
    await fetch(`/api/v1${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  );
}
function useStatus() {
  return useQuery(settingsQueries.connection);
}
export function OCRAction({
  fileID,
  kind = "ocr",
  children,
}: {
  fileID: string;
  kind?: ImageTask;
  children?: (controls: { action: ReactNode; status: ReactNode }) => ReactNode;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const status = useStatus();
  const settings = useQuery({
    ...settingsQueries.services,
    enabled: !!status.data?.supported,
  });
  const access = aiFeatureAccess(settings.data, kind);
  const job = useQuery<OCRJob>({
    queryKey: [`${kind}-job`, fileID],
    queryFn: ({ signal }) => get<OCRJob>(`/files/${fileID}/${kind}`, signal),
    retry: false,
    // The containing metadata group already observes this job; remounting must not retry errors.
    refetchOnMount: false,
    retryOnMount: false,
    staleTime: 10_000,
    refetchInterval: taskPollInterval,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (job.data?.state === "ready") void queries.invalidateQueries();
  }, [job.data?.state, queries]);
  const pending = job.data?.state === "queued" || job.data?.state === "running";
  const waiting =
    pending && (job.data?.state === "running" || job.data?.manual);
  async function request() {
    setBusy(true);
    setError("");
    try {
      const value = await post<OCRJob>(`/files/${fileID}/${kind}`, {
        retry: job.data?.state === "failed",
      });
      queries.setQueryData([`${kind}-job`, fileID], value);
      await queries.invalidateQueries({ queryKey: [`${kind}-queue`] });
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  const issue =
    error ||
    taskErrorMessage(job.data?.error, t) ||
    (!job.data && job.error && errorMessage(job.error, t)) ||
    (!settings.data && settings.error && errorMessage(settings.error, t)) ||
    (!status.data && status.error && errorMessage(status.error, t));
  const readFailed =
    (!job.data && job.isError) ||
    (!settings.data && settings.isError) ||
    (!status.data && status.isError);
  const loading = job.isPending || settings.isPending || status.isPending;
  const needsSetup = !loading && !readFailed && !access.configured;
  const label = t(
    busy || job.data?.state === "running"
      ? "extracting"
      : waiting
        ? "reextractQueued"
        : readFailed || job.data?.state === "failed"
          ? "retry"
          : taskMessage(kind, "Extract"),
  );
  const action =
    needsSetup && !pending ? (
      <Button
        component="a"
        href="/settings#settings-ai"
        variant="subtle"
        size="compact-xs"
      >
        {t("ocrSettings")}
      </Button>
    ) : (
      <Button
        variant="subtle"
        size="compact-xs"
        className="metadata-refresh-action"
        data-active
        disabled={
          busy || !!waiting || loading || (!readFailed && !access.configured)
        }
        onClick={() => {
          if (readFailed) {
            void Promise.all([
              job.isError && job.refetch(),
              settings.isError && settings.refetch(),
              status.isError && status.refetch(),
            ]);
          } else void request();
        }}
      >
        {label}
      </Button>
    );
  const feedback = (
    <>
      {issue && <TaskError>{issue}</TaskError>}
      {needsSetup && !pending && (
        <TaskStatus>{t("ocrConfigureFirst")}</TaskStatus>
      )}
    </>
  );
  return children ? (
    children({ action, status: feedback })
  ) : (
    <>
      {action}
      {feedback}
    </>
  );
}
export function OCRText({ text }: { text: string }) {
  const { t } = useI18n();
  if (!text.trim()) return <>{t("ocrEmpty")}</>;
  return <CopyTextPreview text={text} />;
}
