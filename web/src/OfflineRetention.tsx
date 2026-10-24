import { EmptyHint } from "./EmptyHint";
import { Anchor, Button, Group, Stack, Table, Text } from "@mantine/core";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { routeURL } from "./navigation";
import { api, get } from "./api";
import type { components } from "./client-api-schema";
import type { components as Archive } from "./api-schema";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import { clientCommand } from "./offline-client";

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
                <Table.Th>{t("type")}</Table.Th>
                <Table.Th>{t("clientOfflineStatus")}</Table.Th>
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
  const singleFile =
    (selection.files || []).length === 1 && !selection.filesets?.length;
  const singleSet =
    (selection.filesets || []).length === 1 && !selection.files?.length;
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
    enabled: true,
    refetchOnWindowFocus: false,
  });
  return (
    <Table.Tr>
      <Table.Td>
        {singleFile || singleSet ? (
          <Anchor
            size="sm"
            className="credential-note"
            href={routeURL({
              files: true,
              file: singleFile ? files[0] : filesets[0],
            })}
          >
            <bdi>{name.data || owner}</bdi>
          </Anchor>
        ) : (
          <Text size="sm" className="credential-note">
            <bdi>{owner}</bdi>
          </Text>
        )}
        {name.error && (
          <Text size="xs" c="dimmed">
            {errorMessage(name.error, t)}
          </Text>
        )}
      </Table.Td>
      <Table.Td>
        <Text size="sm" c="dimmed">
          {singleSet
            ? t("filesetLabel")
            : singleFile
              ? t("clientRetainedFile")
              : t("clientRetainedCount", {
                  files: files.length,
                  filesets: filesets.length,
                })}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" role={status.isError ? "alert" : "status"}>
          {status.isError
            ? errorMessage(status.error, t)
            : status.isPending
              ? t("clientOfflineChecking")
              : status.data.ready
                ? t("clientOfflineAvailable")
                : t("clientOfflineIncomplete")}
        </Text>
      </Table.Td>
      <Table.Td>
        <Group gap="xs" wrap="nowrap" justify="flex-end">
          {!status.data?.ready && (
            <Button
              variant="subtle"
              size="compact-sm"
              disabled={busy}
              onClick={() =>
                run(async (signal) => {
                  try {
                    await clientCommand(`${path}/prepare`, {}, "POST", signal);
                  } finally {
                    // Cancel stale reads before refreshing the prepared state.
                    await queries.cancelQueries({
                      queryKey: ["client", "offline-status", owner],
                      exact: true,
                    });
                    await queries.invalidateQueries({
                      queryKey: ["client", "offline-status", owner],
                      exact: true,
                    });
                  }
                })
              }
            >
              {t("clientPrepareOffline")}
            </Button>
          )}
          <Button
            variant="subtle"
            size="compact-sm"
            disabled={busy}
            onClick={() =>
              run(async (signal) => {
                await clientCommand(path, {}, "DELETE", signal);
              })
            }
          >
            {t("clientCancelRetention")}
          </Button>
        </Group>
      </Table.Td>
    </Table.Tr>
  );
}
