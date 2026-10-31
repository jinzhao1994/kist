import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { get } from "./api";
import type { components } from "./api-schema";

type Schema = components["schemas"];
const read = <T>(key: string, path: string) =>
  queryOptions({
    queryKey: [key],
    queryFn: ({ signal }) => get<T>(path, signal),
    staleTime: 10_000,
    retry: false,
    retryOnMount: false,
  });

export type MetadataTask =
  "metadata" | "ocr" | "desc" | "facts" | "orientation" | "geo";
export const queuePath = (kind: MetadataTask) =>
  kind === "geo" ? "/geo" : kind === "metadata" ? "/metadata" : `/ai/${kind}`;
export const settingsQueries = {
  storage: read<Schema["StorageUsage"]>(
    "instance-storage-usage",
    "/settings/storage",
  ),
  credentials: {
    ...read<Schema["SyncCredential"][]>(
      "sync-credentials",
      "/settings/credentials",
    ),
    // This section is always prepared by loadSettingsPage. A catalog check
    // between preloading and mounting must not repeat that just-finished read.
    refetchOnMount: false,
  },
  connection: read<Schema["AIStatus"]>("ai-connection", "/ai"),
  ai: read<Schema["OCRSettings"]>("ai-settings", "/ai/settings"),
  location: read<Schema["LocationSettings"]>(
    "location-settings",
    "/geo/settings",
  ),
  queue: (kind: MetadataTask) =>
    read<Schema["OCRQueueStatus"]>(`${kind}-queue`, queuePath(kind)),
};

// Gather local first-screen data before exposing the settings sections. Child
// observers share these queries, so mounting them does not repeat the requests.
// Models come from a remote service and deliberately remain independent.
export async function loadSettingsPage(
  client: QueryClient,
  signal?: AbortSignal,
  refresh = false,
) {
  const fresh = refresh ? { staleTime: 0 } : {};
  // Storage accounting may scan a large archive. Its fixed summary placeholders
  // can update independently without holding up configuration controls.
  void client.prefetchQuery({ ...settingsQueries.storage, ...fresh });
  const pending = [
    client.prefetchQuery({ ...settingsQueries.credentials, ...fresh }),
  ];
  const settings = await get<Schema["SettingsResponse"]>("/settings", signal);
  if (settings.mode === "library") {
    pending.push(
      client.prefetchQuery({ ...settingsQueries.queue("metadata"), ...fresh }),
    );
    pending.push(
      client.prefetchQuery({ ...settingsQueries.location, ...fresh }),
    );
    pending.push(
      client.prefetchQuery({ ...settingsQueries.queue("geo"), ...fresh }),
    );
    const connection = await client
      .fetchQuery({ ...settingsQueries.connection, ...fresh })
      .catch(() => undefined);
    // Preserve the local-only AI boundary; unsupported instances do not expose
    // AI settings or queue endpoints.
    if (connection?.supported) {
      pending.push(client.prefetchQuery({ ...settingsQueries.ai, ...fresh }));
      for (const kind of ["orientation", "ocr", "desc", "facts"] as const)
        pending.push(
          client.prefetchQuery({ ...settingsQueries.queue(kind), ...fresh }),
        );
    }
  }
  await Promise.all(pending);
  return settings;
}
