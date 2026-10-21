import { get } from "./api";
import { clientCommand } from "./offline-client";
import type { components } from "./client-api-schema";

export type OfflineTarget = { id: string; name: string; fileset: boolean };
export type OfflineStatus = components["schemas"]["ClientOfflineStatus"];
export const offlineOwner = (target: OfflineTarget) =>
  `${target.fileset ? "fileset" : "file"}:${target.id}`;
export const offlinePath = (target: OfflineTarget) =>
  `/offline/${encodeURIComponent(offlineOwner(target))}`;

export async function retainOffline(
  target: OfflineTarget,
  signal: AbortSignal,
) {
  await clientCommand(
    offlinePath(target),
    {
      files: target.fileset ? [] : [target.id],
      filesets: target.fileset ? [target.id] : [],
    },
    "PUT",
    signal,
  );
}

export async function prepareOffline(
  target: OfflineTarget,
  signal: AbortSignal,
) {
  const path = offlinePath(target);
  const status = await get<OfflineStatus>(`/client${path}`, signal);
  return status.ready
    ? status
    : clientCommand<OfflineStatus>(`${path}/prepare`, {}, "POST", signal);
}
