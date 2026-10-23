import { Tooltip, UnstyledButton } from "@mantine/core";
import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, type MediaJob } from "./api";
import type { components } from "./client-api-schema";
import { useI18n } from "./i18n";

export type ConnectionStatus = components["schemas"]["ClientConnectionStatus"];
export const connectionStatusKey = ["client", "connection-status"];

export function useConnectionStatus() {
  return useQuery({
    queryKey: connectionStatusKey,
    queryFn: ({ signal }) =>
      get<ConnectionStatus>("/client/connection-status", signal),
    retry: false,
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
  });
}

export function ClientConnectionStatus({ openSync }: { openSync: () => void }) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useConnectionStatus();
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
  const label = t(
    state === "connected"
      ? "remoteConnected"
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
          <span className="connection-counts">
            <span>
              {t("clientDownloadCount", {
                count:
                  !query.isError &&
                  state === "connected" &&
                  query.data?.pending_download != null
                    ? query.data.pending_download
                    : "—",
              })}
            </span>
            <span>
              {t("clientUploadCount", {
                count: query.isError
                  ? "—"
                  : (query.data?.pending_upload ?? "—"),
              })}
            </span>
          </span>
        </span>
      </UnstyledButton>
    </Tooltip>
  );
}
