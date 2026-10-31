import { Anchor, Group, Table, Text } from "@mantine/core";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { BackgroundTaskJobs } from "./BackgroundTaskJobs";
import { TreeToggle } from "./TreeToggle";
import { FullTextTooltip } from "./FullTextTooltip";
import { HelpTooltip } from "./HelpTooltip";
import { taskErrorMessage } from "./TaskStatus";
import { result } from "./api";
import type { components } from "./api-schema";
import {
  settingsQueries,
  queuePath,
  type MetadataTask,
} from "./settingsQueries";
import { useI18n, type MessageKey } from "./i18n";
import { errorMessage } from "./mutations";

type QueueStatus = components["schemas"]["OCRQueueStatus"];

export function BackgroundTasks({
  disabled = false,
  aiPending = false,
  locationPending = false,
}: {
  disabled?: boolean;
  aiPending?: boolean;
  locationPending?: boolean;
}) {
  const { t } = useI18n();
  const connection = useQuery(settingsQueries.connection);
  const ai = useQuery({
    ...settingsQueries.ai,
    enabled: !!connection.data?.supported,
  });
  const location = useQuery(settingsQueries.location);
  const kinds: MetadataTask[] = connection.data?.supported
    ? ["metadata", "orientation", "ocr", "desc", "facts", "geo"]
    : ["metadata", "geo"];
  return (
    <section
      id="settings-background-tasks"
      className="flat-page-section"
      aria-labelledby="settings-background-tasks-title"
    >
      <h2
        id="settings-background-tasks-title"
        className="settings-section-heading"
        aria-label={t("backgroundTasks")}
      >
        {t("backgroundTasks")}
        <HelpTooltip label={t("backgroundTasks")}>
          {t("backgroundTasksHint")}
        </HelpTooltip>
      </h2>
      <Table.ScrollContainer minWidth={640}>
        <Table
          highlightOnHover
          verticalSpacing="xs"
          horizontalSpacing="sm"
          className="settings-table settings-table--compact task-table"
          aria-label={t("backgroundTasks")}
        >
          <Table.Thead>
            <Table.Tr>
              <Table.Th className="task-table-toggle-cell">
                <span className="sr-only">{t("backgroundTaskDetails")}</span>
              </Table.Th>
              <Table.Th>{t("backgroundTaskType")}</Table.Th>
              <Table.Th>{t("backgroundTaskState")}</Table.Th>
              <Table.Th className="task-table-count">
                {t("backgroundTaskQueued")}
              </Table.Th>
              <Table.Th className="task-table-count">
                {t("backgroundTaskFailed")}
              </Table.Th>
              <Table.Th className="task-table-actions">
                <span className="sr-only">{t("settingCredentialActions")}</span>
              </Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {kinds.map((kind) => (
              <BackgroundTaskRow
                key={kind}
                kind={kind}
                disabled={
                  disabled ||
                  (kind === "metadata"
                    ? false
                    : kind === "geo"
                      ? locationPending
                      : aiPending)
                }
                connected={
                  kind === "metadata"
                    ? true
                    : kind === "geo"
                      ? !!location.data
                      : !!connection.data?.connected
                }
                model={
                  kind === "metadata"
                    ? "local"
                    : kind === "geo"
                      ? "photon"
                      : ai.data?.model
                }
              />
            ))}
          </Table.Tbody>
        </Table>
      </Table.ScrollContainer>
    </section>
  );
}

function BackgroundTaskRow({
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
  const queue = useQuery({
    ...settingsQueries.queue(kind),
    refetchInterval: 2000,
  });
  const queries = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState(false);
  const detailsID = `background-task-${kind}-jobs`;
  async function command(action: "pause" | "backfill", body: unknown) {
    setBusy(true);
    setError("");
    try {
      const value = await result<QueueStatus>(
        await fetch(`/api/v1${queuePath(kind)}/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }),
      );
      queries.setQueryData([`${kind}-queue`], value);
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  const data = queue.data;
  useEffect(() => {
    // Include the final transition to idle/failed even when it stops polling.
    // Inactive (collapsed) details are only marked stale, never fetched here.
    void queries.invalidateQueries({ queryKey: [`${kind}-queue-jobs`] });
  }, [
    kind,
    queries,
    data?.queued,
    data?.failed,
    data?.running,
    data?.scanning,
  ]);
  const title = t(`${kind}Title` as MessageKey);
  // A canceled attempt can already be queued again. Keep queue-wide failures
  // (such as configuration errors) visible even without failed file jobs.
  const queueError =
    data?.error === "context canceled" && !data.failed ? "" : data?.error;
  const issue =
    error ||
    (queue.error && errorMessage(queue.error, t)) ||
    ((data?.paused || !!data?.failed) && queueError
      ? t("backgroundTaskLastFailure", {
          error: taskErrorMessage(queueError, t)!,
        })
      : "");
  const status = !data
    ? t("loading")
    : data.running
      ? t("extractionQueueRunning", { name: data.running })
      : data.paused
        ? t("backgroundTaskPaused")
        : data.scanning
          ? t("backgroundTaskScanning")
          : data.queued
            ? t("backgroundTaskWaiting")
            : t("backgroundTaskIdle");
  return (
    <Table.Tr>
      <Table.Td className="task-table-toggle-cell">
        <span className="table-row-help">
          <TreeToggle
            label={t(
              expanded ? "backgroundTaskCollapse" : "backgroundTaskExpand",
              { name: title },
            )}
            expanded={expanded}
            controls={detailsID}
            onClick={() => setExpanded(!expanded)}
          />
        </span>
      </Table.Td>
      <Table.Th scope="row">
        <div className="setting-field-heading">
          {title}
          <span className="table-row-help">
            <HelpTooltip label={title} inline>
              {t(
                kind === "geo"
                  ? "geoSettingsHint"
                  : (`${kind}BackfillHint` as MessageKey),
              )}
            </HelpTooltip>
          </span>
        </div>
      </Table.Th>
      <Table.Td colSpan={4} className="task-table-content">
        <div className="task-table-summary">
          <div className="task-table-summary-cell">
            <div className="task-table-feedback">
              <FullTextTooltip text={status}>
                <Text
                  size="sm"
                  role="status"
                  tabIndex={0}
                  className="task-table-state"
                >
                  {status}
                </Text>
              </FullTextTooltip>
              {issue && (
                <FullTextTooltip text={issue}>
                  <Text
                    size="xs"
                    c="red"
                    role="alert"
                    tabIndex={0}
                    className="task-table-message"
                  >
                    {issue}
                  </Text>
                </FullTextTooltip>
              )}
              {queue.error && (
                <Anchor
                  component="button"
                  type="button"
                  size="xs"
                  className="text-action-link"
                  onClick={() => void queue.refetch()}
                >
                  {t("retry")}
                </Anchor>
              )}
            </div>
          </div>
          <div className="task-table-summary-cell task-table-count">
            {data?.queued ?? "—"}
          </div>
          <div className="task-table-summary-cell task-table-count">
            {data?.failed ?? "—"}
          </div>
          <div className="task-table-summary-cell">
            <Group
              gap="md"
              wrap="nowrap"
              justify="flex-end"
              className="table-row-actions"
              data-busy={busy || undefined}
            >
              <Anchor
                component="button"
                type="button"
                size="xs"
                className="text-action-link"
                disabled={
                  !data ||
                  disabled ||
                  busy ||
                  !model ||
                  !connected ||
                  data.scanning
                }
                onClick={() => void command("backfill", { retry: false })}
              >
                {t("backgroundTaskBackfill")}
              </Anchor>
              <Anchor
                component="button"
                type="button"
                size="xs"
                className="text-action-link"
                disabled={
                  !data ||
                  disabled ||
                  busy ||
                  !connected ||
                  !data.failed ||
                  data.scanning
                }
                onClick={() => void command("backfill", { retry: true })}
              >
                {t("backgroundTaskRetry")}
              </Anchor>
              <Anchor
                component="button"
                type="button"
                size="xs"
                className="text-action-link"
                disabled={!data || disabled || busy}
                onClick={() => void command("pause", { paused: !data?.paused })}
              >
                {t(
                  data?.paused ? "backgroundTaskResume" : "backgroundTaskPause",
                )}
              </Anchor>
            </Group>
          </div>
        </div>
        {expanded && (
          <BackgroundTaskJobs
            kind={kind}
            title={title}
            id={detailsID}
            active={
              !!data &&
              (!!data.running ||
                (!data.paused && (data.queued > 0 || data.scanning)))
            }
          />
        )}
      </Table.Td>
    </Table.Tr>
  );
}
