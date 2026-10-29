import {
  infiniteQueryOptions,
  queryOptions,
  type QueryClient,
} from "@tanstack/react-query";
import { get } from "./api";
import type { components } from "./client-api-schema";
import { offlineClient } from "./offline-client";

type Schema = components["schemas"];
type HistoryPage = {
  entries: Schema["ClientLocalHistoryEntry"][];
  before?: string;
};
const shared = { staleTime: 10_000, retryOnMount: false };
export const clientQueries = {
  cache: queryOptions({
    ...shared,
    queryKey: ["client", "cache"],
    queryFn: ({ signal }) => offlineClient.cache(signal),
  }),
  storage: queryOptions({
    ...shared,
    queryKey: ["client", "storage"],
    queryFn: ({ signal }) => offlineClient.storage(signal),
  }),
  offline: queryOptions({
    ...shared,
    queryKey: ["client", "offline"],
    queryFn: ({ signal }) =>
      get<Record<string, Schema["ClientOfflineSelection"]>>(
        "/client/offline",
        signal,
      ),
  }),
  history: infiniteQueryOptions({
    ...shared,
    queryKey: ["client", "history"],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) =>
      get<HistoryPage>(
        `/client/history?limit=25&before=${encodeURIComponent(pageParam)}`,
        signal,
      ),
    getNextPageParam: (page) => page.before || undefined,
  }),
};

// These first-screen sections are independent of sync status. Start them
// together and reuse their caches when the page's child components mount.
export async function loadClientPage(
  client: QueryClient,
  signal?: AbortSignal,
) {
  const pending = [
    client.prefetchQuery(clientQueries.offline),
    client.prefetchInfiniteQuery(clientQueries.history),
  ];
  const status = await offlineClient.status(signal);
  await Promise.all(pending);
  return status;
}
