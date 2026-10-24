import { Button, Group, Stack, Text } from "@mantine/core";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api, get } from "./api";
import { Dialog } from "./Dialog";
import { HelpTooltip } from "./HelpTooltip";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import { clientCommand } from "./offline-client";
import type { components } from "./client-api-schema";

import {
  offlineOwner,
  offlinePath,
  retainOffline,
  prepareOffline,
  type OfflineTarget,
} from "./offline-retention";
type Selection = components["schemas"]["ClientOfflineSelection"];
type Status = components["schemas"]["ClientOfflineStatus"];
export type { OfflineTarget } from "./offline-retention";
export function useClientMode() {
  return (
    useQuery({ queryKey: ["info"], queryFn: ({ signal }) => api.info(signal) })
      .data?.client === true
  );
}
export function OfflineEntry({
  target,
  close,
}: {
  target: OfflineTarget;
  close: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const owner = offlineOwner(target);
  const path = offlinePath(target);
  const selections = useQuery({
    queryKey: ["client", "offline"],
    queryFn: ({ signal }) =>
      get<Record<string, Selection>>("/client/offline", signal),
  });
  const retained = !!selections.data?.[owner];
  const status = useQuery({
    queryKey: ["client", "offline-status", owner],
    queryFn: ({ signal }) => get<Status>(`/client${path}`, signal),
    enabled: retained,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function run(remove = false) {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError("");
    try {
      if (remove) {
        await clientCommand(path, {}, "DELETE", controller.signal);
      } else {
        await retainOffline(target, controller.signal);
        // Retention is already durable even if the following transfer fails.
        await queries.invalidateQueries({ queryKey: ["client", "offline"] });
        await prepareOffline(target, controller.signal);
      }
    } catch (err) {
      if (!controller.signal.aborted) setError(errorMessage(err, t));
    } finally {
      // The first availability query can still be in flight when preparation
      // finishes. Invalidation alone joins that old read; cancel it so the
      // post-write refresh cannot publish its stale pre-download result.
      await queries.cancelQueries({
        queryKey: ["client", "offline-status", owner],
        exact: true,
      });
      active.current = null;
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["client"] });
    }
  }
  return (
    <Dialog
      open
      size="sm"
      title={t("clientKeepOffline")}
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <Stack gap="md">
        <Group gap="xs" wrap="nowrap" align="center">
          <Text size="sm" fw={600} className="offline-target-name">
            <bdi>{target.name}</bdi>
          </Text>
          <HelpTooltip label={t("clientKeepOffline")}>
            {t(target.fileset ? "clientKeepFilesetHint" : "clientKeepFileHint")}{" "}
            {t("clientRetentionScope")}
          </HelpTooltip>
        </Group>
        {selections.isPending && (
          <Text size="sm" role="status">
            {t("loading")}
          </Text>
        )}
        {selections.error && (
          <Text size="sm" role="alert">
            {errorMessage(selections.error, t)}
          </Text>
        )}
        {retained && !status.data?.ready && (
          <Text size="sm" c="dimmed">
            {t("clientRetentionSaved")}
          </Text>
        )}
        {retained && status.data && <OfflineReadiness status={status.data} />}
        {status.error && (
          <Text size="sm" role="alert">
            {errorMessage(status.error, t)}
          </Text>
        )}
        {error && (
          <Text size="sm" role="alert">
            {error}
          </Text>
        )}
        {busy && (
          <Text size="sm" role="status">
            {t("clientPreparing")}
          </Text>
        )}
        <Group component="footer" justify="space-between" gap="xs" mt="sm">
          {retained ? (
            <Button variant="subtle" disabled={busy} onClick={() => run(true)}>
              {t("clientStopKeeping")}
            </Button>
          ) : (
            <span />
          )}
          <Group gap="xs">
            {busy ? (
              <Button onClick={() => active.current?.abort()}>
                {t("cancel")}
              </Button>
            ) : (
              <Button
                disabled={selections.isPending || selections.isError}
                onClick={() => run()}
              >
                {t(retained ? "clientPrepareAgain" : "clientKeepAndPrepare")}
              </Button>
            )}
            <Button disabled={busy} onClick={close}>
              {t("close")}
            </Button>
          </Group>
        </Group>
      </Stack>
    </Dialog>
  );
}

export function OfflineReadiness({ status }: { status: Status }) {
  const { t } = useI18n();
  return (
    <Stack gap="xs">
      <Text size="sm" role="status">
        {status.ready
          ? t("clientOfflineReady")
          : t("clientOfflineMissing", {
              ready: status.available,
              total: (status.originals || []).length,
            })}
      </Text>
      {!!status.previews?.length && (
        <Text size="sm" c="dimmed">
          {t("clientOfflinePreviews", {
            ready: status.previews_available,
            total: status.previews.length,
          })}
        </Text>
      )}
    </Stack>
  );
}
