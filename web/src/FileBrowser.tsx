import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Dialog, DialogPresence } from "./Dialog";
import { Button, ActionIcon, Menu } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { api, type Box, type Tag, type Folder, type ArchiveFile } from "./api";
import type { Filters } from "./Filters";
import { useFileResults, resultFiles } from "./fileResults";
import { FolderEditor } from "./Folders";
import { FileGrid } from "./FileGrid";
import { BoxTags } from "./Tags";
import { Upload, type UploadHandle } from "./Upload";
import { ExtractionStatus, pendingFiles, useExtraction } from "./Extraction";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
export function FileBrowser({
  toolbarHost,
  actionsHost,
  infoHost,
  box,
  folder,
  uploadFolder,
  folders = [],
  chooseFolder,
  searching,
  filters,
  onSort,
  tags,
  open,
  paused,
  emptyHint,
}: {
  toolbarHost?: HTMLElement | null;
  actionsHost?: HTMLElement | null;
  infoHost?: HTMLElement | null;
  box?: Box;
  folder?: string;
  uploadFolder?: string;
  folders?: Folder[];
  chooseFolder?: (id: string) => void;
  searching?: boolean;
  emptyHint?: string;
  paused?: boolean;
  filters: Filters;
  onSort?: (sort: NonNullable<Filters["sort"]>) => void;
  tags: Tag[];
  open: (file: ArchiveFile) => void;
}) {
  const { t } = useI18n();
  const picker = useRef<UploadHandle>(null);
  const [addingTag, setAddingTag] = useState(false);
  const [creating, setCreating] = useState(false);
  const [contextParent, setContextParent] = useState<string>();
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
  const content = (
    <>
      {searching && box && <p className="hint">{t("searchWholeBox")}</p>}
      <DialogPresence>
        {(creating || contextParent) &&
          box &&
          (contextParent || uploadFolder) && (
            <FolderEditor
              box={box.id}
              parent={contextParent || uploadFolder!}
              close={() => {
                setCreating(false);
                setContextParent(undefined);
              }}
            />
          )}
      </DialogPresence>
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && !query.isFetchNextPageError && (
        <div role="alert">
          <p>{errorMessage(query.error, t)}</p>
          <Button onClick={() => query.refetch()}>{t("retry")}</Button>
        </div>
      )}
      {query.data && (
        <>
          <span className="file-count sr-only" role="status">
            {t("loadedFiles", { count: files.length })}
          </span>
          <ExtractionStatus files={files} state={extraction} />
          {files.length || folders.length ? (
            <FileGrid
              files={files}
              boxID={box?.id}
              folders={
                filters.sort === "type" && !filters.asc && query.hasNextPage
                  ? []
                  : folders
              }
              chooseFolder={chooseFolder}
              createFolder={box ? setContextParent : undefined}
              filters={filters}
              onSort={onSort}
              open={open}
              paused={paused}
              toolbarHost={toolbarHost}
            />
          ) : !folders.length ? (
            <section className="empty-state file-empty-state">
              <span className="archive-mark" aria-hidden="true">
                ▤
              </span>
              <h2>{t(box && !searching ? "emptyFolder" : "noResults")}</h2>
              <p>
                {emptyHint || (box ? t("uploadToFolder") : t("clearFilters"))}
              </p>
              {box && !searching && info.data && uploadFolder && (
                <Button
                  variant="default"
                  onClick={() => picker.current?.open()}
                >
                  {t("chooseFiles")}
                </Button>
              )}
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
  const actions = box && (
    <Menu position="bottom-end">
      <Menu.Target>
        <ActionIcon
          variant="default"
          size="lg"
          aria-label={t("addItems")}
          title={t("addItems")}
        >
          <span aria-hidden="true">＋</span>
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Item
          disabled={!info.data || !uploadFolder}
          onClick={() => picker.current?.open()}
        >
          {t("chooseFiles")}
        </Menu.Item>
        <Menu.Item disabled={!uploadFolder} onClick={() => setCreating(true)}>
          {t("createFolder")}
        </Menu.Item>
        <Menu.Item onClick={() => setAddingTag(true)}>{t("addTag")}</Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
  const details = box && (
    <>
      {box.description && (
        <p className="file-page-description">
          <bdi>{box.description}</bdi>
        </p>
      )}
      <BoxTags box={box.id} tags={tags} compact editing={false} />
    </>
  );
  return (
    <>
      {actionsHost ? createPortal(actions, actionsHost) : actions}
      {infoHost ? createPortal(details, infoHost) : details}
      <DialogPresence>
        {addingTag && box && (
          <Dialog open onOpenChange={setAddingTag} title={t("addTag")}>
            <BoxTags box={box.id} tags={tags} />
          </Dialog>
        )}
      </DialogPresence>
      {box &&
      info.data &&
      (!info.data.resumable_upload_version || uploadFolder) ? (
        <Upload
          key={`${box.id}:${uploadFolder || ""}`}
          compact
          pickerRef={picker}
          box={box.id}
          folder={uploadFolder}
          maxFiles={info.data.max_upload_files}
          maxBytes={info.data.max_upload_bytes}
          resumableVersion={info.data.resumable_upload_version}
          chunkBytes={info.data.upload_chunk_bytes}
          onComplete={async () => extraction.run(await pendingFiles(box.id))}
        >
          {content}
        </Upload>
      ) : (
        content
      )}
    </>
  );
}
