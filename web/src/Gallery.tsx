import { useEffect, useRef, useState } from "react";
import { Button } from "@mantine/core";
import { useFocusTrap } from "@mantine/hooks";
import { useQuery } from "@tanstack/react-query";
import { api, contentURL, type Tag } from "./api";
import type { Filters } from "./Filters";
import { useFileResults, resultFiles } from "./fileResults";
import { FileDetail } from "./FileDetail";
import { Media } from "./Media";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

export function Gallery({
  id,
  box,
  filters,
  tags,
  change,
  close,
  history,
}: {
  id: string;
  box?: string;
  filters: Filters;
  tags: Tag[];
  change: (id: string) => void;
  close: () => void;
  history: () => void;
}) {
  const { t } = useI18n();
  const [trigger] = useState(
    () => document.activeElement as HTMLElement | null,
  );
  const focusTrap = useFocusTrap();
  const alive = useRef(true);
  const currentID = useRef(id);
  currentID.current = id;
  const [details, setDetails] = useState(
    () => !window.matchMedia("(max-width: 680px)").matches,
  );
  const [advancing, setAdvancing] = useState(false);
  const touch = useRef<{ x: number; y: number } | null>(null);
  const results = useFileResults(box, filters);
  const files = resultFiles(results.data?.pages);
  const index = files.findIndex((file) => file.id === id);
  const query = useQuery({
    queryKey: ["file", id],
    queryFn: ({ signal }) => api.file(id, signal),
  });
  const data = query.data;
  const hasPrevious = index > 0;
  const hasNext =
    index >= 0 && (index + 1 < files.length || !!results.hasNextPage);
  const { hasNextPage, isFetching, isError, fetchNextPage } = results;
  useEffect(() => {
    if (
      index >= 0 &&
      index >= files.length - 3 &&
      hasNextPage &&
      !isFetching &&
      !isError
    )
      void fetchNextPage({ cancelRefetch: false });
  }, [index, files.length, hasNextPage, isFetching, isError, fetchNextPage]);
  useEffect(() => {
    alive.current = true;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = overflow;
      alive.current = false;
      if (trigger?.isConnected) trigger.focus({ preventScroll: true });
    };
  }, []);
  function previous() {
    if (hasPrevious && !advancing) change(files[index - 1].id);
  }
  async function next() {
    if (!hasNext || advancing) return;
    if (index + 1 < files.length) {
      change(files[index + 1].id);
      return;
    }
    setAdvancing(true);
    try {
      const page = await results.fetchNextPage({ cancelRefetch: false });
      const loaded = resultFiles(page.data?.pages);
      const position = loaded.findIndex((file) => file.id === id);
      if (
        alive.current &&
        currentID.current === id &&
        position >= 0 &&
        loaded[position + 1]
      )
        change(loaded[position + 1].id);
    } finally {
      setAdvancing(false);
    }
  }
  return (
    <section
      className={`gallery ${details ? "gallery-with-details" : ""}`}
      aria-label={t("gallery")}
      ref={focusTrap}
      onKeyDown={(event) => {
        if (
          event.defaultPrevented ||
          event.altKey ||
          event.ctrlKey ||
          event.metaKey
        )
          return;
        if (event.key === "Escape") {
          event.preventDefault();
          close();
          return;
        }
        if (
          (event.target as HTMLElement).closest(
            "input,textarea,select,video,audio,[role=combobox],[contenteditable=true]",
          )
        )
          return;
        if (event.key === "ArrowLeft") {
          event.preventDefault();
          previous();
        }
        if (event.key === "ArrowRight") {
          event.preventDefault();
          void next();
        }
      }}
    >
      <header className="gallery-header">
        <Button data-autofocus onClick={close}>
          {t("backToResults")}
        </Button>
        <h1>
          <bdi>{data?.file.name || t("gallery")}</bdi>
        </h1>
        <div className="gallery-actions">
          {data && (
            <a
              className="button-link"
              href={contentURL(data.file, true)}
              download
            >
              {t("download")}
            </a>
          )}
          <Button aria-expanded={details} onClick={() => setDetails(!details)}>
            {t("details")}
          </Button>
        </div>
      </header>
      <div className="gallery-body">
        <div className="gallery-stage">
          <div
            className="gallery-media"
            onTouchStart={(event) => {
              if (
                event.touches.length !== 1 ||
                (event.target as HTMLElement).closest(
                  "video,audio,button,a,input",
                )
              ) {
                touch.current = null;
                return;
              }
              touch.current = {
                x: event.touches[0].clientX,
                y: event.touches[0].clientY,
              };
            }}
            onTouchCancel={() => {
              touch.current = null;
            }}
            onTouchEnd={(event) => {
              const start = touch.current;
              touch.current = null;
              const end = event.changedTouches[0];
              if (!start || !end || event.touches.length) return;
              const dx = end.clientX - start.x,
                dy = end.clientY - start.y;
              if (Math.abs(dx) > 70 && Math.abs(dx) > Math.abs(dy) * 1.5) {
                if (dx > 0) previous();
                else void next();
              }
            }}
          >
            {query.isPending && <p role="status">{t("loading")}</p>}
            {query.isError && (
              <div role="alert">
                <p>{errorMessage(query.error, t)}</p>
                <Button onClick={() => query.refetch()}>{t("retry")}</Button>
              </div>
            )}
            {data && <Media key={id} file={data.file} detail />}
          </div>
          <nav
            className="gallery-navigation"
            aria-label={t("galleryNavigation")}
          >
            <Button disabled={!hasPrevious || advancing} onClick={previous}>
              {t("previousFile")}
            </Button>
            <span role="status">
              {index >= 0
                ? `${index + 1} / ${files.length}${results.hasNextPage ? "+" : ""}`
                : t("singleFile")}
            </span>
            <Button
              disabled={!hasNext || advancing}
              onClick={() => void next()}
            >
              {advancing ? t("loading") : t("nextFile")}
            </Button>
          </nav>
          {results.isFetchingNextPage && (
            <p className="gallery-loading" role="status">
              {t("loading")}
            </p>
          )}
          {results.isError && (
            <div className="gallery-load-error" role="alert">
              <p>{errorMessage(results.error, t)}</p>
              <Button
                onClick={() =>
                  results.isFetchNextPageError
                    ? results.fetchNextPage({ cancelRefetch: false })
                    : results.refetch()
                }
              >
                {t("retry")}
              </Button>
            </div>
          )}
        </div>
        {details && (
          <aside className="gallery-details" aria-label={t("details")}>
            {data && (
              <FileDetail key={id} data={data} tags={tags} history={history} />
            )}
          </aside>
        )}
      </div>
    </section>
  );
}
