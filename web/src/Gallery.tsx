import {
  useFilesets,
  FilesetEditor,
  FilesetPreview,
  primaryFile,
  boxFiles,
} from "./Filesets";
import { DialogPresence } from "./Dialog";
import type { components } from "./api-schema";
import { folderChain } from "./Folders";
import { routeURL } from "./navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button, UnstyledButton, Menu, ActionIcon } from "@mantine/core";
import { useFocusTrap } from "@mantine/hooks";
import { useQuery } from "@tanstack/react-query";
import { api, get, contentURL, type Tag, type ArchiveFile } from "./api";
import type { Filters } from "./Filters";
import { useFileResults, resultFiles } from "./fileResults";
import { FileDetail } from "./FileDetail";
import {
  Media,
  OpenMediaButton,
  usesPDFReader,
  usesTextPreview,
} from "./Media";
import { formatBytes, useI18n } from "./i18n";
import { useGalleryMotion } from "./galleryMotion";
import { errorMessage } from "./mutations";

export function Gallery({
  id,
  box,
  folder,
  filters,
  tags,
  change,
  close,
  history,
}: {
  id: string;
  box?: string;
  folder?: string;
  filters: Filters;
  tags: Tag[];
  change: (id: string) => void;
  close: () => void;
  history: (fileset?: string) => void;
}) {
  const { t, prefs } = useI18n();
  const [trigger] = useState(
    () => document.activeElement as HTMLElement | null,
  );
  const [editing, setEditing] = useState(false);
  const focusTrap = useFocusTrap(!editing);
  const alive = useRef(true);
  const currentID = useRef(id);
  currentID.current = id;
  const [details, setDetails] = useState(() => {
    try {
      return localStorage.getItem("kist.gallery.details.v1") === "open";
    } catch {
      return false;
    }
  });
  function toggleDetails() {
    const next = !details;
    setDetails(next);
    try {
      localStorage.setItem("kist.gallery.details.v1", next ? "open" : "closed");
    } catch {
      // Keep the panel usable when browser storage is unavailable.
    }
  }
  const [advancing, setAdvancing] = useState(false);
  const touch = useRef<{ x: number; y: number } | null>(null);
  const results = useFileResults(box, filters, folder);
  const files = resultFiles(results.data?.pages);
  const index = files.findIndex((file) => (file.fileset_id || file.id) === id);
  const sets = useFilesets();
  const set = sets.data?.filesets?.find((set) => set.folder_id === id);
  const combination = useQuery({
    queryKey: ["fileset", id],
    queryFn: ({ signal }) =>
      get<components["schemas"]["FilesetDetail"]>(`/filesets/${id}`, signal),
    enabled: !!set,
  });
  const selectedSet = combination.data?.fileset || set;
  const primary = selectedSet
    ? primaryFile(selectedSet, sets.data?.filesets || [])
    : id;
  const query = useQuery({
    queryKey: ["file", primary],
    queryFn: ({ signal }) => api.file(primary!, signal),
    enabled: !sets.isPending && !sets.isError && !!primary,
  });
  const data = query.data;
  const members = useQuery({
    queryKey: ["fileset-content", combination.data?.box_id],
    queryFn: ({ signal }) => boxFiles(combination.data!.box_id, signal),
    enabled: !!set && !!combination.data,
  });
  const memberFolders = useQuery({
    queryKey: ["folders", data?.file.box_id],
    queryFn: ({ signal }) => api.folders(data!.file.box_id!, signal),
    enabled: !set && !!data?.file.box_id && !!sets.data?.filesets?.length,
  });
  const owners =
    data && !set
      ? folderChain(
          memberFolders.data?.folders || [],
          data.file.folder_id,
        ).filter((folder) =>
          sets.data?.filesets?.some((set) => set.folder_id === folder.id),
        )
      : [];
  const loading =
    sets.isPending ||
    (set ? combination.isPending || members.isPending : query.isPending);
  const error =
    sets.error || (set ? combination.error || members.error : query.error);
  const retry = () => {
    void sets.refetch();
    if (set) {
      void combination.refetch();
      void members.refetch();
    } else void query.refetch();
  };

  const previewFile = data?.file || files[index];
  const motion = useGalleryMotion(
    id,
    close,
    !set &&
      !!previewFile &&
      !usesPDFReader(previewFile) &&
      !usesTextPreview(previewFile),
  );
  const setGalleryRoot = useCallback(
    (node: HTMLElement | null) => {
      motion.root.current = node;
      focusTrap(node);
    },
    [focusTrap, motion.root],
  );

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
  function openResult(file: ArchiveFile) {
    change(file.fileset_id || file.id);
  }
  function previous() {
    if (hasPrevious && !advancing) openResult(files[index - 1]);
  }
  async function next() {
    if (!hasNext || advancing) return;
    if (index + 1 < files.length) {
      openResult(files[index + 1]);
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
        openResult(loaded[position + 1]);
    } finally {
      setAdvancing(false);
    }
  }
  return (
    <>
      <section
        className={`gallery ${details ? "gallery-with-details" : ""}`}
        aria-label={t("gallery")}
        ref={setGalleryRoot}
        onKeyDown={(event) => {
          if (
            editing ||
            event.defaultPrevented ||
            event.altKey ||
            event.ctrlKey ||
            event.metaKey
          )
            return;
          if (event.key === "Escape") {
            event.preventDefault();
            void motion.dismiss();
            return;
          }
          if (
            (event.target as HTMLElement).closest(
              "input,textarea,select,video,audio,[role=combobox],[contenteditable=true],.text-reader",
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
        <header
          className="gallery-header"
          tabIndex={-1}
          data-autofocus
          aria-label={t("gallery")}
        >
          <UnstyledButton
            className="wordmark gallery-logo"
            aria-label="kist"
            onClick={() => void motion.dismiss()}
          >
            kist
            <span className="brand-dot" />
          </UnstyledButton>
          <Button onClick={() => void motion.dismiss()}>
            {t("backToResults")}
          </Button>
          <h1>
            <bdi>
              {set
                ? combination.data?.folder.name ||
                  files[index]?.name ||
                  t("filesetLabel")
                : data?.file.name || t("gallery")}
            </bdi>
          </h1>
          <div className="gallery-actions">
            {owners.map((owner) => (
              <Button key={owner.id} onClick={() => change(owner.id)}>
                {owner.name}
              </Button>
            ))}
            {!set && data && <OpenMediaButton file={data.file} />}
            {!set && data && (
              <Button component="a" href={contentURL(data.file, true)} download>
                {t("download")}
              </Button>
            )}
            {set && (
              <Menu position="bottom-end">
                <Menu.Target>
                  <ActionIcon
                    variant="default"
                    size="lg"
                    aria-label={t("moreActions")}
                  >
                    <span aria-hidden="true">⋯</span>
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item
                    disabled={!combination.data}
                    onClick={() => setEditing(true)}
                  >
                    {t("filesetConfigure")}
                  </Menu.Item>
                  <Menu.Item
                    disabled={!combination.data}
                    component="a"
                    href={
                      combination.data
                        ? routeURL({
                            files: true,
                            box: combination.data.box_id,
                            folder: id,
                          })
                        : undefined
                    }
                  >
                    {t("filesetShowMembers")}
                  </Menu.Item>
                  <Menu.Item onClick={() => history(id)}>
                    {t("filesetConfigHistory")}
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            )}
          </div>
        </header>
        <div className="gallery-body">
          <div className="gallery-stage">
            <div className="gallery-details-rail">
              <UnstyledButton
                className="gallery-details-toggle"
                aria-label={t("details")}
                aria-expanded={details}
                aria-controls="gallery-details"
                onClick={toggleDetails}
              >
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  aria-hidden="true"
                >
                  <path d={details ? "m9 5 7 7-7 7" : "m15 5-7 7 7 7"} />
                </svg>
              </UnstyledButton>
            </div>
            <div
              className="gallery-media"
              ref={motion.media}
              onTouchStart={(event) => {
                if (
                  event.touches.length !== 1 ||
                  (event.target as HTMLElement).closest(
                    "video,audio,button,a,input,.gpx-detail,.pdf-reader,.text-reader,.fileset-page-reader",
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
              {loading && <p role="status">{t("loading")}</p>}
              {error && (
                <div role="alert">
                  <p>{errorMessage(error, t)}</p>
                  <Button onClick={retry}>{t("retry")}</Button>
                </div>
              )}
              {!loading && !error && selectedSet && members.data ? (
                <div key={id} className="gallery-frame">
                  <FilesetPreview
                    key={id + JSON.stringify(selectedSet)}
                    set={selectedSet}
                    sets={sets.data?.filesets || []}
                    files={members.data}
                    open={openResult}
                  />
                </div>
              ) : !set && data ? (
                <div key={id} className="gallery-frame">
                  <Media file={data.file} detail />
                </div>
              ) : null}
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
            <aside
              id="gallery-details"
              className="gallery-details"
              aria-label={t("details")}
            >
              {query.isError && set && (
                <p role="alert">
                  {errorMessage(query.error, t)}{" "}
                  <Button onClick={() => query.refetch()}>{t("retry")}</Button>
                </p>
              )}
              {set && primary && query.isPending && (
                <p role="status">{t("loading")}</p>
              )}
              {set && !primary && combination.data && (
                <div className="file-info-panel">
                  <h3>{t("basicInfo")}</h3>
                  <dl className="detail-facts">
                    <dt>{t("name")}</dt>
                    <dd>{combination.data.folder.name}</dd>
                    <dt>{t("kind")}</dt>
                    <dd>{t(`fileset_${set.kind}` as "fileset_live_photo")}</dd>
                    <dt>{t("fileSize")}</dt>
                    <dd>
                      {formatBytes(combination.data.folder.total_size, prefs)}
                    </dd>
                  </dl>
                  <p className="supporting-text">{t("filesetIncomplete")}</p>
                </div>
              )}
              {data && (
                <FileDetail
                  key={primary}
                  data={data}
                  tags={tags}
                  history={() => history(set ? id : undefined)}
                  combination={
                    set && combination.data ? combination.data : undefined
                  }
                />
              )}
            </aside>
          )}
        </div>
      </section>
      <DialogPresence>
        {editing && combination.data && (
          <FilesetEditor
            folder={combination.data.folder}
            box={combination.data.box_id}
            sets={sets.data?.filesets || []}
            close={() => setEditing(false)}
          />
        )}
      </DialogPresence>
    </>
  );
}
