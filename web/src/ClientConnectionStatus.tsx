import { Tooltip, UnstyledButton } from "@mantine/core";
import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, type MediaJob } from "./api";
import type { components } from "./client-api-schema";
import { useI18n, type Translate } from "./i18n";
import { clientCommand } from "./offline-client";

export type ConnectionStatus = components["schemas"]["ClientConnectionStatus"];
export const connectionStatusKey = ["client", "connection-status"];

export function useConnectionStatus() {
  return useQuery({
    queryKey: connectionStatusKey,
    queryFn: async ({ signal }) => {
      const status = await get<ConnectionStatus>(
        "/client/connection-status",
        signal,
      );
      if (
        status.state === "connected" &&
        !status.syncing &&
        !status.manual_required &&
        ((status.pending_download != null &&
          (status.pending_download > 0 || status.relay_updates) &&
          status.pending_upload === 0) ||
          (status.pending_download === 0 && status.pending_upload > 0))
      ) {
        // The worker rechecks both histories. A failed wake is retried by its timer.
        await clientCommand("/sync/automatic", {}, "POST", signal).catch(
          () => undefined,
        );
      }
      return status;
    },
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.syncing ||
      query.state.data?.pending_upload ||
      query.state.data?.pending_download
        ? 2000
        : 30000,
    refetchIntervalInBackground: false,
  });
}

export function syncProgressLabel(
  status: ConnectionStatus | undefined,
  t: Translate,
): string {
  if (status?.progress && status.progress.count > 0) {
    return t(
      status.progress.direction === "download"
        ? "clientSyncDownloading"
        : "clientSyncUploading",
      { count: status.progress.count },
    );
  }
  return t("clientSyncing");
}

export function connectionStatusLabel(
  data: ConnectionStatus | undefined,
  isError: boolean,
  t: Translate,
): string {
  const state = isError ? "unreachable" : data?.state;
  return data?.syncing && !isError
    ? syncProgressLabel(data, t)
    : t(
        state === "connected"
          ? data?.manual_required
            ? "clientManualSyncRequired"
            : data?.catalog_complete &&
                data.pending_download === 0 &&
                data.pending_upload === 0 &&
                !data.awaiting_root &&
                !data.relay_updates
              ? "clientSynced"
              : "remoteConnected"
          : state === "unauthorized"
            ? "remoteUnauthorized"
            : state === "incompatible"
              ? "remoteIncompatible"
              : state === "unconfigured"
                ? "remoteUnconfigured"
                : state === "unreachable"
                  ? "remoteDisconnected"
                  : "remoteChecking",
      );
}

export function ClientConnectionStatus({ openSync }: { openSync: () => void }) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useConnectionStatus();
  const catalog = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!query.data) return;
    const version = `${query.data.catalog_op_id}:${query.data.pending_upload}:${query.data.relay_sequence}:${query.data.awaiting_root}`;
    const previous = catalog.current;
    catalog.current = version;
    if (previous !== undefined && previous !== version) {
      void queries.invalidateQueries({
        predicate: (q) =>
          !(
            q.queryKey[0] === "client" && q.queryKey[1] === "connection-status"
          ),
      });
    }
  }, [queries, query.data]);
  const state = query.isError ? "unreachable" : query.data?.state;
  useEffect(() => {
    if (state !== "connected") return;
    void queries.invalidateQueries({
      queryKey: ["media"],
      predicate: (q) => {
        const job = q.state.data as MediaJob | undefined;
        return job?.state === "blocked" && job.reason === "not_cached";
      },
    });
  }, [queries, state, query.dataUpdatedAt]);
  const label = connectionStatusLabel(query.data, query.isError, t);
  return (
    <Tooltip label={query.data?.server || label}>
      <UnstyledButton className="client-connection-status" onClick={openSync}>
        <span
          className="connection-dot"
          data-connected={state === "connected"}
          aria-hidden="true"
        />
        <span role="status">
          <span className="connection-label">{label}</span>
          {query.data && !query.data.syncing && (
            <span className="connection-counts">
              {!query.isError &&
                state === "connected" &&
                (query.data.pending_download ?? 0) > 0 && (
                  <span>
                    {t("clientDownloadCount", {
                      count: query.data.pending_download!,
                    })}
                  </span>
                )}
              {query.data?.relay_updates && (
                <span>{t("clientRelayUpdates")}</span>
              )}
              {!!query.data?.awaiting_root && (
                <span>
                  {t("clientAwaitingRootCount", {
                    count: query.data.awaiting_root,
                  })}
                </span>
              )}
              {query.data?.pending_upload !== 0 && (
                <span>
                  {t("clientUploadCount", {
                    count: query.isError
                      ? "—"
                      : (query.data?.pending_upload ?? "—"),
                  })}
                </span>
              )}
            </span>
          )}
        </span>
      </UnstyledButton>
    </Tooltip>
  );
}
