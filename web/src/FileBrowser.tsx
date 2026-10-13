import { useEffect, useRef, useState } from "react";
import { Button } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { api, type Box, type Tag, type Folder, type ArchiveFile } from "./api";
import type { Filters } from "./Filters";
import { useFileResults, resultFiles } from "./fileResults";
import { FolderCards, FolderEditor } from "./Folders";
import { FileGrid } from "./FileGrid";
import { BoxTags } from "./Tags";
import { Upload } from "./Upload";
import { ExtractionStatus, pendingFiles, useExtraction } from "./Extraction";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
export function FileBrowser({
  box,
  folder,
  uploadFolder,
  folders = [],
  chooseFolder,
  searching,
  filters,
  tags,
  open,
  paused,
  emptyHint,
}: {
  box?: Box;
  folder?: string;
  uploadFolder?: string;
  folders?: Folder[];
  chooseFolder?: (id: string) => void;
  searching?: boolean;
  emptyHint?: string;
  paused?: boolean;
  filters: Filters;
  tags: Tag[];
  open: (file: ArchiveFile) => void;
}) {
  const { t } = useI18n();
  const [creating, setCreating] = useState(false);
  const extraction = useExtraction();
  const info = useQuery({
    queryKey: ["info"],
    queryFn: ({ signal }) => api.info(signal),
  });
  const query = useFileResults(box?.id, filters, folder);
  const end = useRef<HTMLDivElement>(null);
  const { hasNextPage, isFetching, isError, fetchNextPage, dataUpdatedAt } =
    query;
  useEffect(() => {
    const element = end.current;
    if (!element || !hasNextPage || isFetching || isError || paused) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) void fetchNextPage({ cancelRefetch: false });
      },
      { rootMargin: "160px 0px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [hasNextPage, isFetching, isError, paused, fetchNextPage, dataUpdatedAt]);
  const files = resultFiles(query.data?.pages);
  return (
    <>
      {box && (
        <>
          <p className="box-description">
            <bdi>{box.description}</bdi>
          </p>
          <BoxTags box={box.id} tags={tags} />
          {info.data && (
            <Upload
              key={`${box.id}:${uploadFolder || ""}`}
              box={box.id}
              folder={uploadFolder}
              maxFiles={info.data.max_upload_files}
              maxBytes={info.data.max_upload_bytes}
              onComplete={async () =>
                extraction.run(await pendingFiles(box.id))
              }
            />
          )}
        </>
      )}
      {box && uploadFolder && (
        <div className="folder-toolbar">
          <Button onClick={() => setCreating(true)}>{t("createFolder")}</Button>
          {searching && <span className="hint">{t("searchWholeBox")}</span>}
        </div>
      )}
      {creating && box && uploadFolder && (
        <FolderEditor
          box={box.id}
          parent={uploadFolder}
          close={() => setCreating(false)}
        />
      )}
      {!!folders.length && chooseFolder && (
        <FolderCards folders={folders} choose={chooseFolder} />
      )}
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && !query.isFetchNextPageError && (
        <div role="alert">
          <p>{errorMessage(query.error, t)}</p>
          <Button onClick={() => query.refetch()}>{t("retry")}</Button>
        </div>
      )}
      {query.data && (
        <>
          <div className="file-count" role="status">
            {t("loadedFiles", { count: files.length })}
          </div>
          <ExtractionStatus files={files} state={extraction} />
          {files.length ? (
            <FileGrid files={files} open={open} paused={paused} />
          ) : !folders.length ? (
            <section className="empty-state">
              <span className="archive-mark" aria-hidden="true">
                ▤
              </span>
              <h2>{t(box && !searching ? "emptyFolder" : "noResults")}</h2>
              <p>
                {emptyHint || (box ? t("uploadToFolder") : t("clearFilters"))}
              </p>
            </section>
          ) : null}
          <div className="load-more" ref={end}>
            {query.isFetchNextPageError && !query.isFetchingNextPage ? (
              <div role="alert">
                <p>{errorMessage(query.error, t)}</p>
                <Button
                  onClick={() => query.fetchNextPage({ cancelRefetch: false })}
                >
                  {t("retry")}
                </Button>
              </div>
            ) : (
              <p role="status">
                {query.isFetchingNextPage
                  ? t("loading")
                  : !query.hasNextPage && files.length > 0
                    ? t("allFilesShown", { count: files.length })
                    : null}
              </p>
            )}
          </div>
        </>
      )}
    </>
  );
}
