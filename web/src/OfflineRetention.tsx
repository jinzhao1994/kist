import { EmptyHint } from "./EmptyHint";
import { Button, Group, Stack, Table, Text } from "@mantine/core";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, get } from "./api";
import type { components } from "./client-api-schema";
import type { components as Archive } from "./api-schema";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import { clientCommand } from "./offline-client";
import { OfflineReadiness } from "./OfflineEntry";

type Selection = components["schemas"]["ClientOfflineSelection"];
type Status = components["schemas"]["ClientOfflineStatus"];
type Props = {
  active: boolean;
  busy: boolean;
  run: (command: (signal: AbortSignal) => Promise<void>) => Promise<void>;
};

export function OfflineRetention({ active, busy, run }: Props) {
  const { t } = useI18n();
  const selections = useQuery({
    queryKey: ["client", "offline"],
    queryFn: ({ signal }) =>
      get<Record<string, Selection>>("/client/offline", signal),
    enabled: active,
  });
  return (
    <Stack gap="sm">
      <Text size="sm" fw={600}>
        {t("clientRetainedItems")}
      </Text>
      {selections.isPending && <Text role="status">{t("loading")}</Text>}
      {selections.error && (
        <Text role="alert">{errorMessage(selections.error, t)}</Text>
      )}
      {selections.data && Object.keys(selections.data).length === 0 && (
        <EmptyHint>{t("clientRetainedEmpty")}</EmptyHint>
      )}
      {active && !!Object.keys(selections.data || {}).length && (
        <Table.ScrollContainer minWidth={640}>
          <Table
            highlightOnHover
            verticalSpacing="xs"
            horizontalSpacing="sm"
            className="settings-table retention-table"
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t("name")}</Table.Th>
                <Table.Th>{t("clientRetainedContents")}</Table.Th>
                <Table.Th>
                  <span className="sr-only">
                    {t("settingCredentialActions")}
                  </span>
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {active &&
                Object.entries(selections.data || {})
                  .sort(([a], [b]) => a.localeCompare(b))
                  .map(([owner, selection]) => (
                    <RetentionItem
                      key={owner}
                      owner={owner}
                      selection={selection}
                      busy={busy}
                      run={run}
                    />
                  ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Stack>
  );
}

function RetentionItem({
  owner,
  selection,
  busy,
  run,
}: Omit<Props, "active"> & {
  owner: string;
  selection: Selection;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const [expanded, setExpanded] = useState(false);
  const files = selection.files || [];
  const filesets = selection.filesets || [];
  const path = `/offline/${encodeURIComponent(owner)}`;
  const standardFile =
    files.length === 1 && !filesets.length && owner === `file:${files[0]}`;
  const standardSet =
    filesets.length === 1 &&
    !files.length &&
    owner === `fileset:${filesets[0]}`;
  const name = useQuery({
    queryKey: ["client", "offline-name", owner, files, filesets],
    queryFn: async ({ signal }) =>
      standardFile
        ? (await api.file(files[0], signal)).file.name
        : (
            await get<Archive["schemas"]["FilesetDetail"]>(
              `/filesets/${encodeURIComponent(filesets[0])}`,
              signal,
            )
          ).folder.name,
    enabled: standardFile || standardSet,
    retry: false,
  });
  const status = useQuery({
    queryKey: ["client", "offline-status", owner],
    queryFn: ({ signal }) => get<Status>(`/client${path}`, signal),
    enabled: expanded,
    refetchOnWindowFocus: false,
  });
  return (
    <>
      <Table.Tr>
        <Table.Td>
          <Text size="sm" className="credential-note">
            <bdi>{name.data || owner}</bdi>
          </Text>
        </Table.Td>
        <Table.Td>
          <Text size="sm" c="dimmed">
            {t("clientRetainedCount", {
              files: files.length,
              filesets: filesets.length,
            })}
          </Text>
        </Table.Td>
        <Table.Td>
          {name.error && (
            <Text size="xs" c="dimmed">
              {errorMessage(name.error, t)}
            </Text>
          )}
          <Group gap="xs" wrap="nowrap" justify="flex-end">
            <Button
              variant="subtle"
              size="compact-sm"
              disabled={busy}
              aria-expanded={expanded}
              onClick={() => {
                setExpanded(true);
                if (expanded) void status.refetch();
              }}
            >
              {t("clientRetainedDetails")}
            </Button>
            <Button
              variant="subtle"
              size="compact-sm"
              disabled={busy}
              onClick={() =>
                run(async (signal) => {
                  try {
                    await clientCommand(`${path}/prepare`, {}, "POST", signal);
                  } finally {
                    // Do not let a pre-download availability check win the refresh.
                    await queries.cancelQueries({
                      queryKey: ["client", "offline-status", owner],
                      exact: true,
                    });
                  }
                  setExpanded(true);
                })
              }
            >
              {t("clientPrepareAgain")}
            </Button>
            <Button
              variant="subtle"
              size="compact-sm"
              color="red"
              disabled={busy}
              onClick={() =>
                run(async (signal) => {
                  await clientCommand(path, {}, "DELETE", signal);
                })
              }
            >
              {t("clientStopKeeping")}
            </Button>
          </Group>
        </Table.Td>
      </Table.Tr>
      {expanded && (
        <Table.Tr>
          <Table.Td colSpan={3}>
            <Stack gap="xs">
              {status.isFetching && <Text role="status">{t("loading")}</Text>}
              {status.error && (
                <Text role="alert">{errorMessage(status.error, t)}</Text>
              )}
              {status.data && !status.isError && (
                <OfflineReadiness status={status.data} />
              )}
              <Text size="xs" c="dimmed">
                {t("clientRetentionScope")}
              </Text>
            </Stack>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}
