import { useConnectionStatus } from "./ClientConnectionStatus";
import { SectionIndex } from "./SectionIndex";
import { Alert, Button, Checkbox, Group, Stack, Text } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ClientConflictEditor } from "./ClientConflictEditor";
import { OfflineRetention } from "./OfflineRetention";
import { LocalHistory } from "./LocalHistory";
import { actionLabel } from "./History";
import { errorMessage } from "./mutations";
import { formatBytes, formatInstant, useI18n } from "./i18n";
import {
  clientCommand,
  offlineClient,
  type SyncResponse,
} from "./offline-client";

export function OfflineClient({ openSettings }: { openSettings: () => void }) {
  const { t, prefs } = useI18n();
  const connection = useConnectionStatus();
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
    <section aria-label={t("clientTitle")}>
      <header className="page-header">
        <div>
          <h1>{t("clientTitle")}</h1>
          <Text size="sm" c="dimmed">
            {data?.connection?.server || t("clientNotConnected")}
          </Text>
        </div>
        <Group className="header-actions">
          <Button
            variant="filled"
            loading={busy}
            disabled={!data?.connection || historyLocked || !!editing}
            onClick={() =>
              run(async (signal) => {
                const response = await clientCommand<SyncResponse>(
                  "/sync",
                  {},
                  "POST",
                  signal,
                );
                queries.setQueryData(["client", "status"], response.status);
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
      </header>
      <div className="instance-settings-content">
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
            <>
              <SectionIndex
                items={[
                  { id: "sync-status", label: t("clientSync") },
                  { id: "sync-cache", label: t("clientCache") },
                  { id: "sync-history", label: t("clientHistory") },
                ]}
              >
                <section
                  id="sync-status"
                  className="flat-page-section"
                  aria-labelledby="sync-status-title"
                >
                  <h2 id="sync-status-title">{t("clientSync")}</h2>
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
                        {t("clientSyncDescription")}
                      </Text>
                      <div className="sync-status-row">
                        <div>
                          <Text size="xs" c="dimmed">
                            {t("clientCatalogLabel")}
                          </Text>
                          <Text size="sm">
                            {t(
                              data.catalog.complete
                                ? "clientCatalogReady"
                                : "clientCatalogIncomplete",
                            )}
                          </Text>
                        </div>
                        <div>
                          <Text size="xs" c="dimmed">
                            {t("clientRemoteUpdates")}
                          </Text>
                          <Text size="sm">
                            {t("clientDownloadCount", {
                              count:
                                !connection.isError &&
                                connection.data?.state === "connected"
                                  ? (connection.data.pending_download ?? "—")
                                  : "—",
                            })}
                          </Text>
                        </div>
                      </div>
                      {Boolean(data.pending?.length) && (
                        <Text size="sm" fw={600}>
                          {t("clientPending", {
                            count: data.pending?.length || 0,
                          })}
                        </Text>
                      )}
                      {!data.pending?.length && (
                        <Text size="sm" c="dimmed">
                          {t("clientNoPending")}
                        </Text>
                      )}
                      {(data.pending || []).map((op) => (
                        <Stack
                          key={op.id}
                          gap="xs"
                          className="client-pending-item"
                        >
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
                          {data.blocked_id === op.id &&
                            !op.awaiting_receipt && (
                              <>
                                <Group>
                                  <Button
                                    disabled={busy}
                                    onClick={() =>
                                      setEditing({
                                        id: op.id,
                                        action: op.action,
                                      })
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
                </section>
                <section
                  id="sync-cache"
                  className="flat-page-section"
                  aria-labelledby="sync-cache-title"
                >
                  <h2 id="sync-cache-title">{t("clientCache")}</h2>
                  <Stack gap="md">
                    {cache.error && (
                      <Text role="alert">{errorMessage(cache.error, t)}</Text>
                    )}
                    {cache.data && (
                      <>
                        <Text>
                          {t("clientCacheUsed", {
                            size: formatBytes(
                              cache.data.cached_bytes +
                                cache.data.partial_bytes,
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
                          <Alert color="yellow">
                            {t("clientCacheOverLimit")}
                          </Alert>
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
                    <OfflineRetention active busy={busy} run={run} />
                    <Text size="sm" c="dimmed">
                      {t("clientSyncCacheNote")}
                    </Text>
                    <Group>
                      <Button onClick={openSettings} disabled={busy}>
                        {t("settingManageCache")}
                      </Button>
                    </Group>
                    <Group>
                      <Button
                        disabled={busy}
                        onClick={() =>
                          run(async (signal) => {
                            await clientCommand(
                              "/cache/prune",
                              {},
                              "POST",
                              signal,
                            );
                            setNotice(t("clientCachePruned"));
                          })
                        }
                      >
                        {t("clientPrune")}
                      </Button>
                    </Group>
                  </Stack>
                </section>
                <section
                  id="sync-history"
                  className="flat-page-section"
                  aria-labelledby="sync-history-title"
                >
                  <h2 id="sync-history-title">{t("clientHistory")}</h2>
                  <LocalHistory active onLocked={setHistoryLocked} />
                </section>
              </SectionIndex>
            </>
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
          </div>
        </Stack>
      </div>
    </section>
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
