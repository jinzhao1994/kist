import { useEffect, useRef } from "react";
import { useDebouncedValue } from "@mantine/hooks";
import { Button } from "@mantine/core";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api, type Box } from "./api";
import type { BoxFilters } from "./Filters";
import { BoxGrid } from "./BoxGrid";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

export function BoxBrowser({
  filters,
  open,
  paused,
  create,
  compact = false,
  onSort,
}: {
  filters: BoxFilters;
  open: (box: Box) => void;
  paused?: boolean;
  create?: () => void;
  compact?: boolean;
  onSort?: (sort: NonNullable<BoxFilters["sort"]>) => void;
}) {
  const { t } = useI18n();
  const [text] = useDebouncedValue(filters.name || "", 250);
  const search = { ...filters, name: text };
  const query = useInfiniteQuery({
    queryKey: ["boxes", "search", search],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) => {
      const { name, kind: _kind, ...rest } = search;
      return api.searchBoxes(
        { ...rest, text: name, limit: 100, cursor: pageParam },
        signal,
      );
    },
    getNextPageParam: (page) => page.cursor || undefined,
  });
  const end = useRef<HTMLDivElement>(null);
  const { hasNextPage, isFetching, isError, fetchNextPage, dataUpdatedAt } =
    query;
  useEffect(() => {
    if (!end.current || !hasNextPage || isFetching || isError || paused) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) void fetchNextPage({ cancelRefetch: false });
      },
      { rootMargin: "160px 0px" },
    );
    observer.observe(end.current);
    return () => observer.disconnect();
  }, [hasNextPage, isFetching, isError, paused, fetchNextPage, dataUpdatedAt]);
  const boxes = Array.from(
    new Map(
      (query.data?.pages.flatMap((page) => page.boxes || []) || []).map(
        (box) => [box.id, box],
      ),
    ).values(),
  );
  const filtered = !!(
    filters.name ||
    filters.from ||
    filters.to ||
    filters.tag_all?.length ||
    filters.tag_any?.length ||
    filters.tag_not?.length
  );
  return (
    <>
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && (
        <div role="alert">
          <p>{errorMessage(query.error, t)}</p>
          <Button
            onClick={() =>
              query.isFetchNextPageError
                ? query.fetchNextPage({ cancelRefetch: false })
                : query.refetch()
            }
          >
            {t("retry")}
          </Button>
        </div>
      )}
      {query.data && (
        <>
          {boxes.length ? (
            <BoxGrid
              boxes={boxes}
              open={open}
              filters={filters}
              onSort={onSort}
            />
          ) : compact ? (
            <p>{t("noMatchingBoxes")}</p>
          ) : (
            <section className="empty-state">
              <span className="archive-mark" aria-hidden="true">
                ▤
              </span>
              <h2>{t(filtered ? "noMatchingBoxes" : "emptyBoxes")}</h2>
              <p>{t(filtered ? "clearFilters" : "emptyBoxesHint")}</p>
              {!filtered && create && (
                <Button variant="filled" onClick={create}>
                  {t("createBox")}
                </Button>
              )}
            </section>
          )}
          {boxes.length > 0 && (
            <div className="load-more" ref={end} role="status">
              {query.isFetchingNextPage
                ? t("loading")
                : !query.hasNextPage && !query.isError && boxes.length > 0
                  ? t("allBoxesShown", { count: boxes.length })
                  : null}
            </div>
          )}
        </>
      )}
    </>
  );
}
