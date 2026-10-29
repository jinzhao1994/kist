import {
  settingsQueries,
  queuePath,
  type MetadataTask,
} from "./settingsQueries";
import {
  SettingsCheckbox,
  SettingsFieldHeading,
  SettingsHint,
  SettingsTask,
} from "./SettingsFields";
import {
  TaskError,
  TaskStatus,
  taskPollInterval,
  taskErrorMessage,
} from "./TaskStatus";
import { CopyTextPreview } from "./CopyTextPreview";
import {
  Alert,
  Button,
  Group,
  Stack,
  Text,
  PasswordInput,
} from "@mantine/core";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, result } from "./api";
import { Select } from "./Select";
import { useI18n, type MessageKey } from "./i18n";
import { errorMessage } from "./mutations";

type Model = { id: string; name: string };
async function post<T>(
  path: string,
  body: unknown = {},
  method = "POST",
): Promise<T> {
  return result<T>(
    await fetch(`/api/v1${path}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  );
}
function useStatus() {
  return useQuery({
    ...settingsQueries.connection,
    refetchInterval: (query) => (query.state.data?.pending ? 1000 : false),
  });
}
export function AIConnection({
  disabled = false,
  onChanged,
}: { disabled?: boolean; onChanged?: () => Promise<void> } = {}) {
  const { t } = useI18n();
  const query = useStatus();
  const queries = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [url, setURL] = useState("");
  const wasPending = useRef(false);
  useEffect(() => {
    const pending = query.data?.pending;
    if (wasPending.current && pending === false) void onChanged?.();
    wasPending.current = !!pending;
  }, [query.data?.pending, onChanged]);
  async function connect() {
    // Open synchronously within the click gesture so popup blockers allow it.
    const login = window.open("about:blank", "_blank");
    if (login) login.opener = null;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const out = await post<{ url: string }>("/ai/connect");
      setURL(out.url);
      if (login && !login.closed) login.location.replace(out.url);
    } catch (e) {
      login?.close();
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["ai-connection"] });
      await onChanged?.();
    }
  }
  async function disconnect() {
    setBusy(true);
    setError("");
    setURL("");
    try {
      const out = await post<{ revoked: boolean }>("/ai/disconnect");
      if (!out.revoked) setNotice(t("aiRevocationUnconfirmed"));
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["ai-connection"] });
      await onChanged?.();
      await queries.invalidateQueries({ queryKey: ["ai-models"] });
    }
  }
  const status = query.data;
  return (
    <section
      id="settings-ai"
      className="flat-page-section"
      aria-labelledby="settings-ai-title"
    >
      <h2 id="settings-ai-title">{t("aiConnection")}</h2>
      <Stack>
        <SettingsHint>{t("aiLocalHint")}</SettingsHint>
        {(error || query.error || status?.error) && (
          <Alert role="alert" color="red">
            {error ||
              (query.error ? errorMessage(query.error, t) : status?.error)}
          </Alert>
        )}
        {notice && <Text role="status">{notice}</Text>}
        {disabled && (
          <Text size="sm" c="dimmed">
            {t("settingsPending")}
          </Text>
        )}
        <Text>
          {status?.connected
            ? `${t("aiConnected")}: ${status.email}`
            : t("aiDisconnected")}
        </Text>
        <Group>
          <Button
            disabled={disabled || busy || !status?.supported || status.pending}
            onClick={connect}
          >
            {t("aiContinue")}
          </Button>
          {(status?.connected || status?.pending) && (
            <Button disabled={disabled || busy} onClick={disconnect}>
              {t(status.pending ? "cancel" : "aiDisconnect")}
            </Button>
          )}
          {url && status?.pending && (
            <Button component="a" href={url} target="_blank" rel="noreferrer">
              {t("aiOpenSignIn")}
            </Button>
          )}
        </Group>
        {status?.pending && (
          <Text role="status" size="sm">
            {t("aiWaiting")}
          </Text>
        )}
        {status?.supported && (
          <AISettings
            connected={status.connected}
            disabled={disabled || busy}
            onChanged={onChanged}
          />
        )}
      </Stack>
    </section>
  );
}
type AISettingsData = NonNullable<ReturnType<typeof useAISettings>["data"]>;
type QueueStatus = NonNullable<ReturnType<typeof useOCRQueue>["data"]>;
type OCRJob = {
  manual?: boolean;
  state: "missing" | "queued" | "running" | "ready" | "failed";
  error?: string;
};
function useAISettings() {
  return useQuery({
    ...settingsQueries.ai,
  });
}
export type ImageTask = Exclude<MetadataTask, "geo">;
const taskMessage = (kind: MetadataTask, suffix: string) =>
  `${kind}${suffix}` as MessageKey;
function useOCRQueue(kind: MetadataTask = "ocr") {
  return useQuery({
    ...settingsQueries.queue(kind),
    refetchInterval: 2000,
  });
}
function AISettings({
  connected,
  disabled,
  onChanged,
}: {
  connected: boolean;
  disabled: boolean;
  onChanged?: () => Promise<void>;
}) {
  const { t } = useI18n();
  const query = useAISettings();
  const proxyID = useId();
  const queries = useQueryClient();
  const models = useQuery({
    queryKey: ["ai-models"],
    queryFn: ({ signal }) => get<Model[]>("/ai/models", signal),
    enabled: connected,
    retry: false,
  });
  const [draft, setDraft] = useState<AISettingsData>();
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    if (query.data && !dirty) setDraft(query.data);
  }, [query.data, dirty]);
  function edit(change: Partial<AISettingsData>) {
    setDirty(true);
    setDraft((old) => (old ? { ...old, ...change } : old));
    setNotice("");
  }
  async function perform(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn();
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  async function save() {
    if (!draft) return;
    await perform(async () => {
      const value = await post<AISettingsData>("/ai/settings", draft, "PUT");
      queries.setQueryData(["ai-settings"], value);
      setDirty(false);
      setDraft(value);
      await queries.invalidateQueries({ queryKey: ["ai-models"] });
      await onChanged?.();
      setNotice(t("settingsSaved"));
    });
  }
  const modelOptions = (models.data || []).map((m) => ({
    value: m.id,
    label: m.name || m.id,
  }));
  if (draft?.model && !modelOptions.some((m) => m.value === draft.model))
    modelOptions.unshift({ value: draft.model, label: draft.model });
  return (
    <div className="settings-blocks">
      {query.isPending && (
        <Text role="status" size="sm">
          {t("loading")}
        </Text>
      )}
      {(error || query.error) && (
        <Alert color="red" role="alert">
          {error || (query.error && errorMessage(query.error, t))}
        </Alert>
      )}
      {query.error && !draft && (
        <Button onClick={() => void query.refetch()}>{t("retry")}</Button>
      )}
      {notice && (
        <Text role="status" size="sm">
          {notice}
        </Text>
      )}
      {draft && (
        <div className="settings-fields">
          <Select
            label={t("aiModel")}
            value={draft.model}
            onValueChange={(model) => edit({ model })}
            options={[
              { value: "", label: t("aiChooseModel"), special: true },
              ...modelOptions,
            ]}
            disabled={disabled || busy || models.isFetching || !connected}
          />
          {models.isFetching && (
            <Text role="status" size="sm">
              {t("aiModelsLoading")}
            </Text>
          )}
          {connected && models.isError && (
            <Group>
              <Text role="alert" size="sm">
                {errorMessage(models.error, t)}
              </Text>
              <Button disabled={busy} onClick={() => void models.refetch()}>
                {t("retry")}
              </Button>
            </Group>
          )}
          {connected && models.isSuccess && models.data.length === 0 && (
            <Text size="sm">{t("aiModelsEmpty")}</Text>
          )}
          {!draft.model && models.data?.some((m) => m.id === "gpt-6-luna") && (
            <Button
              disabled={disabled || busy}
              onClick={() => edit({ model: "gpt-6-luna" })}
            >
              {t("aiUseLuna")}
            </Button>
          )}
          <div>
            <SettingsFieldHeading
              label={t("aiProxy")}
              htmlFor={proxyID}
              help={t("aiProxyHint")}
            />
            <PasswordInput
              id={proxyID}
              value={draft.proxy_url}
              onChange={(e) => edit({ proxy_url: e.currentTarget.value })}
              disabled={disabled || busy}
              autoComplete="off"
            />
          </div>
          <SettingsCheckbox
            label={t("orientationAuto")}
            help={t("orientationAutoHint")}
            checked={!!draft.auto_orientation}
            onChange={(e) =>
              edit({ auto_orientation: e.currentTarget.checked })
            }
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("ocrAuto")}
            help={t("ocrAutoHint")}
            checked={draft.auto_ocr}
            onChange={(e) => edit({ auto_ocr: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("descAuto")}
            help={t("descAutoHint")}
            checked={!!draft.auto_desc}
            onChange={(e) => edit({ auto_desc: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("factsAuto")}
            help={t("factsAutoHint")}
            checked={!!draft.auto_facts}
            onChange={(e) => edit({ auto_facts: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <Group>
            <Button
              onClick={() => void save()}
              disabled={
                disabled ||
                busy ||
                !dirty ||
                ((draft.auto_ocr ||
                  draft.auto_desc ||
                  draft.auto_facts ||
                  draft.auto_orientation) &&
                  !draft.model)
              }
            >
              {t("aiSaveSettings")}
            </Button>
            <Button
              disabled={busy || disabled}
              onClick={() => {
                setDirty(false);
                setError("");
                void query.refetch();
                if (connected) void models.refetch();
              }}
            >
              {t("aiReloadSettings")}
            </Button>
          </Group>
        </div>
      )}
      {(["orientation", "ocr", "desc", "facts"] as const).map((kind) => (
        <BackgroundTaskQueue
          key={kind}
          kind={kind}
          disabled={disabled || busy || dirty}
          connected={connected}
          model={query.data?.model}
        />
      ))}
    </div>
  );
}
export function BackgroundTaskQueue({
  kind,
  disabled,
  connected,
  model,
}: {
  kind: MetadataTask;
  disabled: boolean;
  connected: boolean;
  model?: string;
}) {
  const { t } = useI18n();
  const queue = useOCRQueue(kind);
  const queries = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  async function command(path: string, body: unknown) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const value = await post<QueueStatus>(path, body);
      queries.setQueryData([`${kind}-queue`], value);
      if (path.endsWith("backfill")) setNotice(t("ocrBackfillStarted"));
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  return (
    <SettingsTask
      title={t(
        kind === "geo" ? "backgroundProcessing" : taskMessage(kind, "Title"),
      )}
    >
      {error && <TaskError>{error}</TaskError>}
      {notice && (
        <Text role="status" size="sm">
          {notice}
        </Text>
      )}
      {queue.error && (
        <Group>
          <TaskError>{errorMessage(queue.error, t)}</TaskError>
          <Button onClick={() => void queue.refetch()}>{t("retry")}</Button>
        </Group>
      )}
      {queue.data && (
        <>
          <div className="settings-task-status">
            <Text className="settings-hint" role="status">
              {queue.data.paused
                ? t(taskMessage(kind, "QueuePaused"))
                : queue.data.running
                  ? t("extractionQueueRunning", {
                      name: queue.data.running,
                    })
                  : t(taskMessage(kind, "QueueIdle"))}
            </Text>
            <SettingsHint>
              {t("ocrQueueCounts", {
                queued: queue.data.queued,
                failed: queue.data.failed,
              })}
            </SettingsHint>
          </div>
          {queue.data.scanning && (
            <Text role="status" size="sm">
              {t(taskMessage(kind, "Scanning"))}
            </Text>
          )}
          {queue.data.error && (
            <TaskError>{taskErrorMessage(queue.data.error, t)}</TaskError>
          )}
          <Group>
            <Button
              disabled={
                disabled || busy || !model || !connected || queue.data.scanning
              }
              onClick={() =>
                void command(`${queuePath(kind)}/backfill`, { retry: false })
              }
            >
              {t(taskMessage(kind, "Backfill"))}
            </Button>
            <Button
              disabled={
                disabled ||
                busy ||
                !connected ||
                !queue.data.failed ||
                queue.data.scanning
              }
              onClick={() =>
                void command(`${queuePath(kind)}/backfill`, { retry: true })
              }
            >
              {t("ocrRetryFailed")}
            </Button>
            <Button
              disabled={disabled || busy}
              onClick={() =>
                void command(`${queuePath(kind)}/pause`, {
                  paused: !queue.data?.paused,
                })
              }
            >
              {t(queue.data.paused ? "ocrResume" : "ocrPause")}
            </Button>
          </Group>
          <SettingsHint>{t(taskMessage(kind, "BackfillHint"))}</SettingsHint>
        </>
      )}
    </SettingsTask>
  );
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
  const settings = useAISettings();
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
  const needsSetup =
    !loading &&
    !readFailed &&
    (!status.data?.connected || !settings.data?.model);
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
          busy ||
          !!waiting ||
          loading ||
          (!readFailed && (!status.data?.connected || !settings.data?.model))
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
