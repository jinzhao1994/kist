import {
  Alert,
  Button,
  Checkbox,
  Group,
  NumberInput,
  PasswordInput,
  Stack,
  Tabs,
  Text,
  TextInput,
} from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ClientConflictEditor } from "./ClientConflictEditor";
import { OfflineRetention } from "./OfflineRetention";
import { LocalHistory } from "./LocalHistory";
import { Dialog } from "./Dialog";
import { actionLabel } from "./History";
import { errorMessage } from "./mutations";
import { formatBytes, formatInstant, useI18n } from "./i18n";
import {
  clientCommand,
  offlineClient,
  type ClientStatus,
  type SyncResponse,
} from "./offline-client";

export function OfflineClient({ close }: { close: () => void }) {
  const { t, prefs } = useI18n();
  const queries = useQueryClient();
  const status = useQuery({
    queryKey: ["client", "status"],
    queryFn: ({ signal }) => offlineClient.status(signal),
  });
  const cache = useQuery({
    queryKey: ["client", "cache"],
    queryFn: ({ signal }) => offlineClient.cache(signal),
  });
  const storage = useQuery({
    queryKey: ["client", "storage"],
    queryFn: ({ signal }) => offlineClient.storage(signal),
  });
  const [busy, setBusy] = useState(false);
  const [historyLocked, setHistoryLocked] = useState(false);
  const [tab, setTab] = useState<string | null>("sync");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editing, setEditing] = useState<{ id: string; action: string }>();
  const [decision, setDecision] = useState("");
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function run(command: (signal: AbortSignal) => Promise<void>) {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await command(controller.signal);
    } catch (err) {
      if (!controller.signal.aborted) setError(errorMessage(err, t));
    } finally {
      active.current = null;
      setBusy(false);
      await queries.invalidateQueries();
    }
  }
  const data = status.data;
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy && !historyLocked) close();
      }}
      title={t("clientTitle")}
      size="lg"
    >
      <Stack gap="md">
        {status.isPending && <p role="status">{t("loading")}</p>}
        {status.error && <p role="alert">{errorMessage(status.error, t)}</p>}
        {error && (
          <Alert color="red" role="alert">
            {error}
          </Alert>
        )}
        {notice && <Text role="status">{notice}</Text>}
        {data && (
          <Tabs
            value={tab}
            onChange={(value) => {
              if (!busy && !historyLocked) setTab(value);
            }}
          >
            <Tabs.List>
              <Tabs.Tab value="sync">{t("clientSync")}</Tabs.Tab>
              <Tabs.Tab value="cache">{t("clientCache")}</Tabs.Tab>
              <Tabs.Tab value="history">{t("clientHistory")}</Tabs.Tab>
              <Tabs.Tab value="connection">{t("clientConnection")}</Tabs.Tab>
            </Tabs.List>
            <Tabs.Panel value="sync" pt="md">
              {editing ? (
                <ClientConflictEditor
                  key={editing.id}
                  {...editing}
                  close={() => setEditing(undefined)}
                  onLocked={setHistoryLocked}
                />
              ) : (
                <Stack gap="md">
                  <Text size="sm" c="dimmed">
                    {data.connection?.server || t("clientNotConnected")}
                  </Text>
                  <Text size="sm">
                    {t(
                      data.catalog.complete
                        ? "clientCatalogComplete"
                        : "clientCatalogIncomplete",
                    )}
                  </Text>
                  <Group>
                    <Button
                      loading={busy}
                      disabled={!data.connection}
                      onClick={() =>
                        run(async (signal) => {
                          const response = await clientCommand<SyncResponse>(
                            "/sync",
                            {},
                            "POST",
                            signal,
                          );
                          queries.setQueryData(
                            ["client", "status"],
                            response.status,
                          );
                          setNotice(
                            t(
                              response.result.complete
                                ? "clientSyncComplete"
                                : "clientSyncConflict",
                            ),
                          );
                        })
                      }
                    >
                      {t("clientSyncNow")}
                    </Button>
                  </Group>
                  <Text fw={600}>
                    {t("clientPending", { count: (data.pending || []).length })}
                  </Text>
                  {!data.pending?.length && (
                    <Text size="sm" c="dimmed">
                      {t("clientNoPending")}
                    </Text>
                  )}
                  {(data.pending || []).map((op) => (
                    <Stack key={op.id} gap="xs" className="client-pending-item">
                      <Group justify="space-between">
                        <Text>{actionLabel(op.action, t)}</Text>
                        <Text size="sm" c="dimmed">
                          {formatInstant(op.occurred_at, prefs)}
                        </Text>
                      </Group>
                      <Text size="xs" c="dimmed">
                        {op.id}
                      </Text>
                      {op.awaiting_receipt && (
                        <Text size="sm">{t("clientAwaitingReceipt")}</Text>
                      )}
                      {op.conflict && (
                        <Alert color="yellow">{op.conflict}</Alert>
                      )}
                      {data.blocked_id === op.id && !op.awaiting_receipt && (
                        <>
                          <Group>
                            <Button
                              disabled={busy}
                              onClick={() =>
                                setEditing({ id: op.id, action: op.action })
                              }
                            >
                              {t("clientConflictReview")}
                            </Button>
                          </Group>
                          <Checkbox
                            checked={decision === op.id}
                            disabled={busy}
                            onChange={(event) =>
                              setDecision(
                                event.currentTarget.checked ? op.id : "",
                              )
                            }
                            label={t("clientDiscardConfirm")}
                          />
                          <Group>
                            <Button
                              color="red"
                              disabled={busy || decision !== op.id}
                              onClick={() =>
                                run(async (signal) => {
                                  await clientCommand(
                                    `/pending/${encodeURIComponent(op.id)}/resolve`,
                                    {
                                      reason:
                                        "User chose to discard the local change in the offline client UI",
                                    },
                                    "POST",
                                    signal,
                                  );
                                  setDecision("");
                                  setNotice(t("clientResolved"));
                                })
                              }
                            >
                              {t("clientDiscard")}
                            </Button>
                          </Group>
                        </>
                      )}
                    </Stack>
                  ))}
                </Stack>
              )}
            </Tabs.Panel>
            <Tabs.Panel value="cache" pt="md">
              <Stack gap="md">
                {cache.error && (
                  <Text role="alert">{errorMessage(cache.error, t)}</Text>
                )}
                {cache.data && (
                  <>
                    <Text>
                      {t("clientCacheUsed", {
                        size: formatBytes(
                          cache.data.cached_bytes + cache.data.partial_bytes,
                          prefs,
                        ),
                      })}
                    </Text>
                    <Text size="sm" c="dimmed">
                      {t("clientPinnedUsed", {
                        size: formatBytes(cache.data.pinned_bytes, prefs),
                      })}
                    </Text>
                    {cache.data.over_limit && (
                      <Alert color="yellow">{t("clientCacheOverLimit")}</Alert>
                    )}
                  </>
                )}
                {storage.error && (
                  <Text role="alert">{errorMessage(storage.error, t)}</Text>
                )}
                {storage.data && (
                  <Text size="sm">
                    {t("clientProtectedUsed", {
                      originals: formatBytes(
                        storage.data.originals_bytes,
                        prefs,
                      ),
                      journal: formatBytes(
                        storage.data.pending_journal_bytes,
                        prefs,
                      ),
                    })}
                  </Text>
                )}
                <OfflineRetention
                  active={tab === "cache"}
                  busy={busy}
                  run={run}
                />
                <CacheForm
                  key={`${data.settings.cache_max_age}:${data.settings.cache_max_bytes}`}
                  status={data}
                  busy={busy}
                  run={run}
                />
                <Group>
                  <Button
                    disabled={busy}
                    onClick={() =>
                      run(async (signal) => {
                        await clientCommand("/cache/prune", {}, "POST", signal);
                        setNotice(t("clientCachePruned"));
                      })
                    }
                  >
                    {t("clientPrune")}
                  </Button>
                </Group>
              </Stack>
            </Tabs.Panel>
            <Tabs.Panel value="history" pt="md">
              <LocalHistory
                active={tab === "history"}
                onLocked={setHistoryLocked}
              />
            </Tabs.Panel>
            <Tabs.Panel value="connection" pt="md">
              <ConnectionForm
                key={data.connection?.server || "none"}
                status={data}
                busy={busy}
                run={run}
              />
            </Tabs.Panel>
          </Tabs>
        )}
        <div className="actions">
          {busy && (
            <Button
              onClick={() => {
                active.current?.abort();
                setNotice(t("clientInterrupted"));
              }}
            >
              {t("cancel")}
            </Button>
          )}
          <Button disabled={busy || historyLocked} onClick={close}>
            {t("close")}
          </Button>
        </div>
      </Stack>
    </Dialog>
  );
}

type FormProps = {
  status: ClientStatus;
  busy: boolean;
  run: (command: (signal: AbortSignal) => Promise<void>) => Promise<void>;
};
function ConnectionForm({ status, busy, run }: FormProps) {
  const { t } = useI18n();
  const [server, setServer] = useState(status.connection?.server || "");
  const [token, setToken] = useState("");
  const [allowHTTP, setAllowHTTP] = useState(
    status.connection?.allow_http || false,
  );
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        run(async (signal) => {
          await clientCommand(
            "/connection",
            { server, token, allow_http: allowHTTP },
            "PUT",
            signal,
          );
          setToken("");
        });
      }}
    >
      <Stack gap="md">
        <TextInput
          label={t("clientServer")}
          value={server}
          onChange={(event) => setServer(event.currentTarget.value)}
          required
          withAsterisk={false}
          disabled={busy}
          type="url"
        />
        <PasswordInput
          label={t("clientToken")}
          value={token}
          onChange={(event) => setToken(event.currentTarget.value)}
          required
          withAsterisk={false}
          disabled={busy}
          autoComplete="off"
        />
        <Checkbox
          label={t("clientAllowHTTP")}
          checked={allowHTTP}
          onChange={(event) => setAllowHTTP(event.currentTarget.checked)}
          disabled={busy}
        />
        <Text size="sm" c="dimmed">
          {t("clientConnectionHint")}
        </Text>
        <Group>
          <Button type="submit" disabled={busy}>
            {t("clientConnect")}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
function CacheForm({ status, busy, run }: FormProps) {
  const { t } = useI18n();
  const [age, setAge] = useState(status.settings.cache_max_age);
  const [bytes, setBytes] = useState<number | string>(
    status.settings.cache_max_bytes / (1024 * 1024),
  );
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        run(async (signal) => {
          await clientCommand(
            "/cache",
            {
              max_age: age,
              max_bytes: Math.floor(Number(bytes) * 1024 * 1024),
            },
            "PUT",
            signal,
          );
        });
      }}
    >
      <Stack gap="md">
        <TextInput
          label={t("clientMaxAge")}
          description={t("clientMaxAgeHint")}
          value={age}
          onChange={(event) => setAge(event.currentTarget.value)}
          required
          withAsterisk={false}
          disabled={busy}
        />
        <NumberInput
          label={t("clientMaxSize")}
          value={bytes}
          onChange={setBytes}
          min={1}
          allowDecimal={false}
          required
          withAsterisk={false}
          disabled={busy}
        />
        <Group>
          <Button type="submit" disabled={busy}>
            {t("save")}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}

export function ClientCatalogNotice() {
  const { t } = useI18n();
  const status = useQuery({
    queryKey: ["client", "status"],
    queryFn: ({ signal }) => offlineClient.status(signal),
  });
  if (status.error)
    return <Alert color="yellow">{errorMessage(status.error, t)}</Alert>;
  if (!status.data?.catalog.complete)
    return (
      <Text size="sm" c="dimmed" role="status">
        {t("clientCatalogIncomplete")}
      </Text>
    );
  return null;
}
