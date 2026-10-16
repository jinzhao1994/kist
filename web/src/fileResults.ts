import { useInfiniteQuery } from "@tanstack/react-query";
import { api, type ArchiveFile } from "./api";
import { searchConditions, type Filters } from "./Filters";

export function useFileResults(
  box: string | undefined,
  filters: Filters,
  folder?: string,
) {
  return useInfiniteQuery({
    queryKey: ["files", box, filters, ...(folder ? [folder] : [])],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) =>
      api.files(
        {
          ...filters,
          ...(folder
            ? { folder, recursive: searchConditions(filters).length > 0 }
            : {}),
          box: box ? [box] : filters.box?.length ? filters.box : undefined,
          limit: 100,
          cursor: pageParam,
        },
        signal,
      ),
    getNextPageParam: (page) => page.cursor || undefined,
    refetchOnMount: false,
  });
}
export function resultFiles(
  pages: { files: ArchiveFile[] | null }[] | undefined,
) {
  return Array.from(
    new Map(
      (pages?.flatMap((page) => page.files || []) || []).map((file) => [
        file.id,
        file,
      ]),
    ).values(),
  );
}
