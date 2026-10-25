import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get } from "./api";

// One observer per application, shared by every catalog view. Inactive lists
// are marked stale; active lists refresh in place without discarding pages.
export function useCatalogRefresh(location: string) {
  const queries = useQueryClient();
  const revision = useQuery({
    queryKey: ["catalog-revision"],
    queryFn: ({ signal }) =>
      get<{ revision: string }>("/catalog-revision", signal),
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
  });
  const previous = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!revision.data?.revision || previous.current === revision.data.revision)
      return;
    previous.current = revision.data.revision;
    // Check the initial result too: data may have been cached before mounting
    // this observer or fetched just before the first revision response arrived.
    void queries.invalidateQueries({
      predicate: (query) => query.queryKey[0] !== "catalog-revision",
    });
  }, [queries, revision.data]);
  const { refetch } = revision;
  useEffect(() => {
    void refetch();
  }, [location, refetch]);
}
