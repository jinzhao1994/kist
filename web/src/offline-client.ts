import { get, result } from "./api";
import type { components } from "./client-api-schema";
export type ClientStatus = components["schemas"]["ClientStatus"];
export type CacheStatus = components["schemas"]["ClientCacheStatus"];
export type ProtectedStorage = components["schemas"]["ClientProtectedStorage"];
export type SyncResponse = components["schemas"]["ClientSyncResponse"];
export async function clientCommand<T>(
  path: string,
  body: unknown,
  method = "POST",
  signal?: AbortSignal,
): Promise<T> {
  return result<T>(
    await fetch(`/api/v1/client${path}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    }),
  );
}
export const offlineClient = {
  status: (signal?: AbortSignal) => get<ClientStatus>("/client/status", signal),
  cache: (signal?: AbortSignal) => get<CacheStatus>("/client/cache", signal),
  storage: (signal?: AbortSignal) =>
    get<ProtectedStorage>("/client/protected-storage", signal),
};
