import { settingsQueries, type MetadataTask } from "./settingsQueries";
import {
  SettingsCheckbox,
  SettingsFieldHeading,
  SettingsReset,
  SettingsHint,
  useSettingsPending,
  settingsDraftValue,
  type SettingsSectionDraft,
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
  onPendingChange,
  onDraftChange,
  reloadVersion,
  configurationPending = false,
}: {
  disabled?: boolean;
  onChanged?: () => Promise<void>;
  onPendingChange?: (pending: boolean) => void;
  onDraftChange?: (draft: SettingsSectionDraft | null) => void;
  reloadVersion?: number;
  configurationPending?: boolean;
} = {}) {
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
            disabled={
              disabled ||
              configurationPending ||
              busy ||
              !status?.supported ||
              status.pending
            }
            onClick={connect}
          >
            {t("aiContinue")}
          </Button>
          {(status?.connected || status?.pending) && (
            <Button
              disabled={disabled || configurationPending || busy}
              onClick={disconnect}
            >
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
            onPendingChange={onPendingChange}
            onDraftChange={onDraftChange}
            reloadVersion={reloadVersion}
          />
        )}
      </Stack>
    </section>
  );
}
type AISettingsData = NonNullable<ReturnType<typeof useAISettings>["data"]>;
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
function AISettings({
  connected,
  disabled,
  onChanged,
  onPendingChange,
  onDraftChange,
  reloadVersion = 0,
}: {
  connected: boolean;
  disabled: boolean;
  onChanged?: () => Promise<void>;
  onPendingChange?: (pending: boolean) => void;
  onDraftChange?: (draft: SettingsSectionDraft | null) => void;
  reloadVersion?: number;
}) {
  const { t } = useI18n();
  const query = useAISettings();
  const proxyID = useId();
  const modelID = useId();
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
  useSettingsPending(disabled || busy || dirty, onPendingChange);
  useEffect(() => {
    if (query.data && !dirty) setDraft(query.data);
  }, [query.data, dirty]);
  function edit(change: Partial<AISettingsData>) {
    setDirty(true);
    setDraft((old) =>
      old
        ? {
            ...old,
            ...change,
            changed: [
              ...new Set([...(old.changed || []), ...Object.keys(change)]),
            ],
            reset: (old.reset || []).filter(
              (key) => !Object.hasOwn(change, key),
            ),
          }
        : old,
    );
    setNotice("");
  }
  function resetAction(key: keyof NonNullable<AISettingsData["defaults"]>) {
    const value = query.data?.defaults?.[key];
    const specified =
      !!draft &&
      (draft.changed?.includes(key) ||
        (!!query.data?.explicit?.includes(key) && !draft.reset?.includes(key)));
    return (
      <SettingsReset
        specified={!!specified}
        disabled={disabled || busy || value === undefined || !specified}
        onReset={() => {
          if (value === undefined) return;
          setDraft((old) =>
            old
              ? {
                  ...old,
                  [key]: value,
                  changed: (old.changed || []).filter((field) => field !== key),
                  reset: [...new Set([...(old.reset || []), key])],
                }
              : old,
          );
          setDirty(true);
          setNotice("");
        }}
      />
    );
  }
  const valid = !(
    (draft?.auto_ocr ||
      draft?.auto_desc ||
      draft?.auto_facts ||
      draft?.auto_orientation) &&
    !draft?.model
  );
  useEffect(() => {
    setDirty(false);
    setError("");
    setNotice("");
    setDraft(queries.getQueryData(["ai-settings"]));
  }, [reloadVersion, queries]);
  useEffect(() => {
    onDraftChange?.(
      dirty && draft ? { value: settingsDraftValue(draft), valid } : null,
    );
  }, [dirty, draft, valid, onDraftChange]);
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
      const value = await post<AISettingsData>(
        "/ai/settings",
        {
          ...draft,
          defaults: undefined,
          explicit: undefined,
          changed: draft.changed || [],
          reset: draft.reset?.length ? draft.reset : undefined,
        },
        "PUT",
      );
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
          <div className="setting-field">
            <SettingsFieldHeading
              label={t("aiModel")}
              htmlFor={modelID}
              action={resetAction("model")}
            />
            <Select
              id={modelID}
              aria-label={t("aiModel")}
              value={draft.model}
              onValueChange={(model) => edit({ model })}
              options={[
                { value: "", label: t("aiChooseModel"), special: true },
                ...modelOptions,
              ]}
              disabled={disabled || busy || models.isFetching || !connected}
            />
          </div>
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
          <div className="setting-field">
            <SettingsFieldHeading
              label={t("aiProxy")}
              action={resetAction("proxy_url")}
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
            action={resetAction("auto_orientation")}
            help={t("orientationAutoHint")}
            checked={!!draft.auto_orientation}
            onChange={(e) =>
              edit({ auto_orientation: e.currentTarget.checked })
            }
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("ocrAuto")}
            action={resetAction("auto_ocr")}
            help={t("ocrAutoHint")}
            checked={draft.auto_ocr}
            onChange={(e) => edit({ auto_ocr: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("descAuto")}
            action={resetAction("auto_desc")}
            help={t("descAutoHint")}
            checked={!!draft.auto_desc}
            onChange={(e) => edit({ auto_desc: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          <SettingsCheckbox
            label={t("factsAuto")}
            action={resetAction("auto_facts")}
            help={t("factsAutoHint")}
            checked={!!draft.auto_facts}
            onChange={(e) => edit({ auto_facts: e.currentTarget.checked })}
            disabled={disabled || busy}
          />
          {!onDraftChange && (
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
          )}
        </div>
      )}
    </div>
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
