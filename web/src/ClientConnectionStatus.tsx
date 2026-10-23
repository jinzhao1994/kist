import { Tooltip, UnstyledButton } from "@mantine/core";
import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, type MediaJob } from "./api";
import type { components } from "./client-api-schema";
import { useI18n } from "./i18n";

export type ConnectionStatus = components["schemas"]["ClientConnectionStatus"];
export const connectionStatusKey = ["client", "connection-status"];

export function ClientConnectionStatus({
  openSettings,
}: {
  openSettings: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useQuery({
    queryKey: connectionStatusKey,
    queryFn: ({ signal }) =>
      get<ConnectionStatus>("/client/connection-status", signal),
    retry: false,
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
  });
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
      <UnstyledButton
        className="client-connection-status"
        onClick={openSettings}
      >
        <span
          className="connection-dot"
          data-connected={state === "connected"}
          aria-hidden="true"
        />
        <span role="status">{label}</span>
      </UnstyledButton>
    </Tooltip>
  );
}
