import { HelpTooltip } from "./HelpTooltip";
import {
  useConnectionStatus,
  connectionStatusLabel,
  connectionStatusKey,
  type ConnectionStatus,
} from "./ClientConnectionStatus";
import { SectionIndex } from "./SectionIndex";
import { Alert, Button, Checkbox, Group, Stack, Text } from "@mantine/core";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { useElementSize } from "@mantine/hooks";
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
  const { ref: headerRef, height: headerHeight } = useElementSize();
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
  const downloads =
    !connection.isError && connection.data?.state === "connected"
      ? (connection.data.pending_download ?? 0)
      : 0;
  const uploads = data?.pending?.length ?? 0;
  return (
    <section
      aria-label={t("clientTitle")}
      style={
        { "--section-top-offset": `${headerHeight + 40}px` } as CSSProperties
      }
    >
      <header ref={headerRef} className="page-header settings-page-header">
        <div>
          <h1>{t("clientTitle")}</h1>
        </div>
        <Group className="header-actions">
          <Button
            variant="filled"
            loading={busy || connection.data?.syncing}
            disabled={
              !data?.connection ||
              historyLocked ||
              !!editing ||
              connection.data?.syncing
            }
            onClick={() =>
              run(async (signal) => {
                queries.setQueryData<ConnectionStatus>(
                  connectionStatusKey,
                  (value) => (value ? { ...value, syncing: true } : value),
                );
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
          {status.isPending && <Text role="status">{t("loading")}</Text>}
          {status.error && (
            <Alert color="red" role="alert">
              {errorMessage(status.error, t)}
            </Alert>
          )}
          {error && (
            <Alert color="red" role="alert">
              {error}
            </Alert>
          )}
          {notice && (
            <Text size="sm" role="status">
              {notice}
            </Text>
          )}
          {data && (
            <>
              <SectionIndex
                ready={headerHeight > 0}
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
                  <h2
                    id="sync-status-title"
                    className="settings-section-heading"
                  >
                    {t("clientSync")}
                    <HelpTooltip label={t("clientSyncDescription")}>
                      {t("clientSyncDescription")}
                    </HelpTooltip>
                  </h2>
                  {editing ? (
                    <ClientConflictEditor
                      key={editing.id}
                      {...editing}
                      close={() => setEditing(undefined)}
                      onLocked={setHistoryLocked}
                    />
                  ) : (
                    <Stack gap="sm">
                      <Text size="sm" c="dimmed">
                        {data?.connection?.server
                          ? `${t("clientServer")}: ${data.connection.server}`
                          : t("clientNotConnected")}
                      </Text>
                      <Text size="sm" role="status">
                        {connectionStatusLabel(
                          connection.data,
                          connection.isError,
                          t,
                        )}
                      </Text>
                      {!data.catalog.complete && (
                        <Text size="sm" c="dimmed">
                          {t("clientCatalogIncomplete")}
                        </Text>
                      )}
                      {!connection.data?.syncing &&
                        (downloads > 0 || uploads > 0) && (
                          <Group gap="lg">
                            {downloads > 0 && (
                              <Text size="sm" c="dimmed">
                                {t("clientDownloadCount", {
                                  count: downloads,
                                })}
                              </Text>
                            )}
                            {uploads > 0 && (
                              <Text size="sm" c="dimmed">
                                {t("clientUploadCount", {
                                  count: uploads,
                                })}
                              </Text>
                            )}
                          </Group>
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
                  <h2
                    id="sync-cache-title"
                    className="settings-section-heading"
                  >
                    {t("clientCache")}
                    <HelpTooltip label={t("clientCache")}>
                      {t("clientSyncCacheNote")}
                    </HelpTooltip>
                  </h2>
                  <Stack gap="md">
                    {cache.error && (
                      <Text role="alert">{errorMessage(cache.error, t)}</Text>
                    )}
                    {cache.data && (
                      <>
                        <Text size="sm">
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
                    <Group>
                      <Button onClick={openSettings} disabled={busy}>
                        {t("settingManageCache")}
                      </Button>
                    </Group>
                  </Stack>
                </section>
                <section
                  id="sync-history"
                  className="flat-page-section"
                  aria-labelledby="sync-history-title"
                >
                  <h2
                    id="sync-history-title"
                    className="settings-section-heading"
                  >
                    {t("clientHistory")}
                    <HelpTooltip label={t("clientHistory")}>
                      {t("clientHistoryHint")}
                    </HelpTooltip>
                  </h2>
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
