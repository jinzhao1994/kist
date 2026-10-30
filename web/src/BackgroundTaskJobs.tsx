import { Anchor, Group, Table, Text } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { get } from "./api";
import type { components } from "./api-schema";
import { FileBoxSource } from "./FileBoxSource";
import { FullTextTooltip } from "./FullTextTooltip";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import { routeURL } from "./navigation";
import { queuePath, type MetadataTask } from "./settingsQueries";
import { TaskError, TaskStatus, taskErrorMessage } from "./TaskStatus";

type QueuePage = components["schemas"]["ExtractionQueuePage"];

// This observer only exists while expanded; only the visible page is refreshed.
export function BackgroundTaskJobs({
  kind,
  title,
  id,
  active,
}: {
  kind: MetadataTask;
  title: string;
  id: string;
  active: boolean;
}) {
  const { t } = useI18n();
  const [cursors, setCursors] = useState([""]);
  const cursor = cursors[cursors.length - 1];
  const jobs = useQuery({
    queryKey: [`${kind}-queue-jobs`, cursor],
    queryFn: ({ signal }) =>
      get<QueuePage>(
        `${queuePath(kind)}/jobs?limit=20&cursor=${encodeURIComponent(cursor)}`,
        signal,
      ),
    refetchInterval: (query) =>
      active ? (query.state.error ? 15000 : 2000) : false,
    retry: false,
  });
  const items = jobs.data?.items;
  return (
    <div id={id} className="task-details">
      {jobs.isPending && <TaskStatus>{t("loading")}</TaskStatus>}
      {jobs.error && (
        <Group gap="sm">
          <TaskError>{errorMessage(jobs.error, t)}</TaskError>
          <Anchor
            component="button"
            type="button"
            size="xs"
            className="text-action-link"
            onClick={() => void jobs.refetch()}
          >
            {t("retry")}
          </Anchor>
        </Group>
      )}
      {items && items.length === 0 && (
        <TaskStatus>{t("backgroundTaskEmpty")}</TaskStatus>
      )}
      {!!items?.length && (
        <Table
          verticalSpacing="xs"
          horizontalSpacing="sm"
          className="settings-table settings-table--dense task-details-table"
          aria-label={t("backgroundTaskList", { name: title })}
        >
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t("name")}</Table.Th>
              <Table.Th>{t("backgroundTaskBox")}</Table.Th>
              <Table.Th>{t("backgroundTaskState")}</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items?.map((job) => (
              <Table.Tr key={job.request_id}>
                <Table.Td>
                  <FullTextTooltip text={job.name}>
                    {job.file_available ? (
                      <Anchor
                        href={routeURL({ files: true, file: job.file_id })}
                        className="task-details-name"
                      >
                        {job.name}
                      </Anchor>
                    ) : (
                      <Text
                        className="task-details-name"
                        c="dimmed"
                        tabIndex={0}
                      >
                        {job.name}
                      </Text>
                    )}
                  </FullTextTooltip>
                </Table.Td>
                <Table.Td>
                  <FileBoxSource box={job.box} />
                </Table.Td>
                <Table.Td>
                  <div className="task-table-feedback">
                    <Text
                      className="task-table-state"
                      c={job.state === "failed" ? "red" : "dimmed"}
                    >
                      {t(
                        job.state === "failed"
                          ? "backgroundTaskFailed"
                          : "backgroundTaskQueued",
                      )}
                    </Text>
                    {job.error && (
                      <FullTextTooltip text={taskErrorMessage(job.error, t)!}>
                        <Text
                          size="xs"
                          c="red"
                          className="task-table-message"
                          tabIndex={0}
                        >
                          {taskErrorMessage(job.error, t)}
                        </Text>
                      </FullTextTooltip>
                    )}
                  </div>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
      {(cursors.length > 1 || jobs.data?.next_cursor) && (
        <Group justify="flex-end" gap="md" className="task-details-pagination">
          <Anchor
            component="button"
            type="button"
            size="xs"
            className="text-action-link"
            disabled={cursors.length === 1 || jobs.isFetching}
            onClick={() => setCursors(cursors.slice(0, -1))}
          >
            {t("backgroundTaskPrevious")}
          </Anchor>
          <Anchor
            component="button"
            type="button"
            size="xs"
            className="text-action-link"
            disabled={!jobs.data?.next_cursor || jobs.isFetching}
            onClick={() => setCursors([...cursors, jobs.data!.next_cursor])}
          >
            {t("backgroundTaskNext")}
          </Anchor>
        </Group>
      )}
    </div>
  );
}
