import { CopyTextPreview } from "./CopyTextPreview";
import {
  Alert,
  Button,
  Group,
  Stack,
  Text,
  Checkbox,
  PasswordInput,
} from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, result } from "./api";
import { Select } from "./Select";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Status = {
  supported: boolean;
  connected: boolean;
  email: string;
  pending: boolean;
  error: string;
};
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
    queryKey: ["ai-connection"],
    queryFn: ({ signal }) => get<Status>("/ai", signal),
    retry: false,
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
        <Text size="sm" c="dimmed">
          {t("aiLocalHint")}
        </Text>
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
type AISettingsData = {
  model: string;
  auto_ocr: boolean;
  proxy_url: string;
  revision: string;
};
type QueueStatus = {
  paused: boolean;
  scanning: boolean;
  queued: number;
  failed: number;
  running: string;
  error?: string;
};
type OCRJob = {
  state: "missing" | "queued" | "running" | "ready" | "failed";
  error?: string;
};
function useAISettings() {
  return useQuery({
    queryKey: ["ai-settings"],
    queryFn: ({ signal }) => get<AISettingsData>("/ai/settings", signal),
    retry: false,
  });
}
function useOCRQueue() {
  return useQuery({
    queryKey: ["ocr-queue"],
    queryFn: ({ signal }) => get<QueueStatus>("/ai/ocr", signal),
    retry: false,
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
  const queue = useOCRQueue();
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
  async function command(path: string, body: unknown) {
    await perform(async () => {
      const value = await post<QueueStatus>(path, body);
      queries.setQueryData(["ocr-queue"], value);
      if (path.endsWith("backfill")) setNotice(t("ocrBackfillStarted"));
    });
  }
  const modelOptions = (models.data || []).map((m) => ({
    value: m.id,
    label: m.name || m.id,
  }));
  if (draft?.model && !modelOptions.some((m) => m.value === draft.model))
    modelOptions.unshift({ value: draft.model, label: draft.model });
  return (
    <Stack>
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
        <>
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
          <PasswordInput
            label={t("aiProxy")}
            description={t("aiProxyHint")}
            value={draft.proxy_url}
            onChange={(e) => edit({ proxy_url: e.currentTarget.value })}
            disabled={disabled || busy}
            autoComplete="off"
          />
          <Checkbox
            label={t("ocrAuto")}
            description={t("ocrAutoHint")}
            checked={draft.auto_ocr}
            onChange={(e) => edit({ auto_ocr: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <Group>
            <Button
              onClick={() => void save()}
              disabled={
                disabled || busy || !dirty || (draft.auto_ocr && !draft.model)
              }
            >
              {t("save")}
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
              {t("settingsReload")}
            </Button>
          </Group>
        </>
      )}
      {queue.error && (
        <Group>
          <Text role="alert" size="sm">
            {errorMessage(queue.error, t)}
          </Text>
          <Button onClick={() => void queue.refetch()}>{t("retry")}</Button>
        </Group>
      )}
      {queue.data && (
        <>
          <Text size="sm" role="status">
            {queue.data.paused
              ? t("ocrQueuePaused")
              : queue.data.running
                ? t("ocrQueueRunning", { name: queue.data.running })
                : t("ocrQueueIdle")}
          </Text>
          <Text size="sm" c="dimmed">
            {t("ocrQueueCounts", {
              queued: queue.data.queued,
              failed: queue.data.failed,
            })}
          </Text>
          {queue.data.scanning && (
            <Text role="status" size="sm">
              {t("ocrScanning")}
            </Text>
          )}
          {queue.data.error && (
            <Text size="sm" role="alert">
              {queue.data.error}
            </Text>
          )}
          <Group>
            <Button
              disabled={
                disabled ||
                busy ||
                dirty ||
                !query.data?.model ||
                !connected ||
                queue.data.scanning
              }
              onClick={() => void command("/ai/ocr/backfill", { retry: false })}
            >
              {t("ocrBackfill")}
            </Button>
            <Button
              disabled={
                disabled ||
                busy ||
                dirty ||
                !connected ||
                !queue.data.failed ||
                queue.data.scanning
              }
              onClick={() => void command("/ai/ocr/backfill", { retry: true })}
            >
              {t("ocrRetryFailed")}
            </Button>
            <Button
              disabled={disabled || busy}
              onClick={() =>
                void command("/ai/ocr/pause", { paused: !queue.data?.paused })
              }
            >
              {t(queue.data.paused ? "ocrResume" : "ocrPause")}
            </Button>
          </Group>
          <Text size="sm" c="dimmed">
            {t("ocrBackfillHint")}
          </Text>
        </>
      )}
    </Stack>
  );
}
export function OCRAction({ fileID }: { fileID: string }) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const status = useStatus();
  const settings = useAISettings();
  const queue = useOCRQueue();
  const job = useQuery({
    queryKey: ["ocr-job", fileID],
    queryFn: ({ signal }) => get<OCRJob>(`/files/${fileID}/ocr`, signal),
    retry: false,
    refetchInterval: 2000,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (job.data?.state === "ready") void queries.invalidateQueries();
  }, [job.data?.state, queries]);
  const pending = job.data?.state === "queued" || job.data?.state === "running";
  async function request() {
    setBusy(true);
    setError("");
    try {
      const value = await post<OCRJob>(`/files/${fileID}/ocr`, {
        retry: job.data?.state === "failed",
      });
      queries.setQueryData(["ocr-job", fileID], value);
      await queries.invalidateQueries({ queryKey: ["ocr-queue"] });
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  const issue =
    error ||
    job.data?.error ||
    (job.error && errorMessage(job.error, t)) ||
    (settings.error && errorMessage(settings.error, t)) ||
    (status.error && errorMessage(status.error, t)) ||
    (queue.error && errorMessage(queue.error, t));
  return (
    <Stack gap="xs">
      {issue && (
        <Text role="alert" size="sm">
          {issue}
        </Text>
      )}
      {pending && (
        <Text role="status" size="sm">
          {t(job.data?.state === "running" ? "extracting" : "ocrQueued")}
        </Text>
      )}
      {queue.data?.paused && <Text size="sm">{t("ocrQueuePaused")}</Text>}
      {(!status.data?.connected || !settings.data?.model) && (
        <Text size="sm">{t("ocrConfigureFirst")}</Text>
      )}
      <Group>
        <Button
          disabled={
            busy ||
            pending ||
            !status.data?.connected ||
            !settings.data?.model ||
            job.isPending ||
            job.isError ||
            queue.data?.paused
          }
          onClick={() => void request()}
        >
          {t(job.data?.state === "failed" ? "retry" : "ocrExtract")}
        </Button>
        <Button component="a" href="/settings#settings-ai" variant="subtle">
          {t("ocrSettings")}
        </Button>
        {job.isError && (
          <Button onClick={() => void job.refetch()}>{t("retry")}</Button>
        )}
      </Group>
    </Stack>
  );
}
export function OCRText({ text }: { text: string }) {
  const { t } = useI18n();
  if (!text.trim()) return <>{t("ocrEmpty")}</>;
  return <CopyTextPreview text={text} />;
}
