import { DialogPresence } from "./Dialog";
import {
  UnstyledButton,
  Button,
  ActionIcon,
  Menu,
  Tooltip,
} from "@mantine/core";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Box } from "./api";
import { FolderBreadcrumbs, folderChain } from "./Folders";
import { formatLocalDate, useI18n } from "./i18n";
import { Settings } from "./Settings";
import { BoxTrashDialog } from "./BoxTrashDialog";
import { DisplayModeControl, useDisplayMode } from "./DisplayMode";
import { BoxBrowser } from "./BoxBrowser";
import { TagResults } from "./TagResults";
import { flattenTags } from "./TagPath";
import { BoxEditor } from "./BoxEditor";
import { TagManager, TagTree } from "./Tags";
import {
  FiltersPanel,
  emptyFilters,
  emptyBoxFilters,
  searchConditions,
  type BoxFilters,
  type Filters,
} from "./Filters";
import { FileBrowser } from "./FileBrowser";
import { Gallery } from "./Gallery";
import { useNavigation } from "./navigation";
import { errorMessage } from "./mutations";
import { HistoryPage } from "./History";
import { NavigationIcon } from "./NavigationIcon";
import {
  SavedSearches,
  SaveSearchButton,
  searchDefinition,
} from "./SavedSearches";
export function App() {
  const { t, prefs } = useI18n();
  const { route, go } = useNavigation();
  const [actionsHost, setActionsHost] = useState<HTMLSpanElement | null>(null);
  const [infoHost, setInfoHost] = useState<HTMLDivElement | null>(null);
  const [fileToolbarHost, setFileToolbarHost] =
    useState<HTMLSpanElement | null>(null);
  const [boxMode, setBoxMode] = useDisplayMode("boxes");
  const [settings, setSettings] = useState(false);
  const [editor, setEditor] = useState<Box | "new" | null>(null);
  const [trashing, setTrashing] = useState<Box | null>(null);
  const [manage, setManage] = useState(false);
  const [filters, setFilters] = useState<Filters>({
    ...emptyFilters,
    ...route.filters,
  });
  useEffect(() => {
    setFilters({ ...emptyFilters, ...route.filters });
  }, [route.filters]);
  const [boxFilters, setBoxFilters] = useState<BoxFilters>({
    ...emptyBoxFilters,
    ...route.boxFilters,
  });
  useEffect(() => {
    setBoxFilters({ ...emptyBoxFilters, ...route.boxFilters });
  }, [route.boxFilters]);
  const box = useQuery({
    queryKey: ["boxes", "detail", route.box],
    queryFn: ({ signal }) => api.box(route.box!, signal),
    enabled: !!route.box,
  });
  const folders = useQuery({
    queryKey: ["folders", route.box],
    queryFn: ({ signal }) => api.folders(route.box!, signal),
    enabled: !!route.box && route.files && !route.section,
  });
  const folderList = folders.data?.folders || [];
  const currentFolder =
    route.folder || folderList.find((folder) => !folder.parent_id)?.id;
  const searching = searchConditions(filters).length > 0;
  const queryFolder = route.box ? currentFolder : undefined;
  const currentSearch =
    !route.section && !route.tag
      ? searchDefinition(
          route.files ? "files" : "boxes",
          route.files ? filters : boxFilters,
          route.files ? route.box : undefined,
          route.box ? currentFolder : undefined,
        )
      : undefined;
  const chooseFolder = (folder: string) => {
    setFilters({ ...emptyFilters });
    go({ box: route.box, files: true, folder });
  };
  const boxes = useQuery({
    queryKey: ["boxes"],
    queryFn: ({ signal }) => api.boxes(signal),
    enabled: !!route.section || (route.files && !route.box),
  });
  const info = useQuery({
    queryKey: ["info"],
    queryFn: ({ signal }) => api.info(signal),
  });
  const tags = useQuery({
    queryKey: ["tags"],
    queryFn: ({ signal }) => api.tags(signal),
  });
  const client = useQueryClient();
  const selected = box.data;
  const tree = tags.data?.tags || [];
  const selectedTag = flattenTags(tree).find((tag) => tag.id === route.tag);
  const navigate = (box?: Box, files = false) => {
    setFilters({ ...emptyFilters });
    go({ box: box?.id, files: !!box || files });
  };
  const boxManagementActions = selected && (
    <>
      <Menu.Item
        onClick={() =>
          go({
            files: false,
            section: "history",
            box: selected.id,
          })
        }
      >
        {t("history")}
      </Menu.Item>
      <Menu.Item onClick={() => setEditor(selected)}>{t("editBox")}</Menu.Item>
      <Menu.Divider />
      <Menu.Item color="red" onClick={() => setTrashing(selected)}>
        {t("moveBoxToTrash")}
      </Menu.Item>
    </>
  );
  return (
    <>
      <div
        className="app-shell"
        inert={!!route.file && !route.section}
        aria-hidden={(!!route.file && !route.section) || undefined}
      >
        <aside className="sidebar">
          <a
            className="wordmark"
            href="/"
            onClick={(event) => {
              event.preventDefault();
              navigate();
            }}
          >
            kist
            <span className="brand-dot" />
          </a>
          <nav aria-label={t("archive")}>
            <UnstyledButton
              className={`nav-link ${route.files && !route.box && !route.tag ? "active" : ""}`}
              onClick={() => navigate(undefined, true)}
            >
              <NavigationIcon kind="files" /> <span>{t("allFiles")}</span>
            </UnstyledButton>
            <UnstyledButton
              className={`nav-link ${!route.files && !route.section && !route.tag ? "active" : ""}`}
              onClick={() => navigate()}
            >
              <NavigationIcon kind="boxes" /> <span>{t("boxes")}</span>
            </UnstyledButton>
            {(["trash", "history"] as const).map((section) => (
              <UnstyledButton
                key={section}
                className={`nav-link ${route.section === section ? "active" : ""}`}
                onClick={() => go({ files: false, section })}
              >
                <NavigationIcon kind={section} /> <span>{t(section)}</span>
              </UnstyledButton>
            ))}
          </nav>
          <SavedSearches
            current={currentSearch}
            open={(next) => {
              if (next.files) setFilters({ ...emptyFilters, ...next.filters });
              else setBoxFilters({ ...emptyBoxFilters, ...next.boxFilters });
              go(next);
            }}
          />
          <div className="sidebar-tag-section">
            <details className="sidebar-tags" open>
              <summary>
                <svg
                  className="sidebar-tags-chevron"
                  width="12"
                  height="12"
                  viewBox="0 0 16 16"
                  fill="none"
                  aria-hidden="true"
                >
                  <path
                    d="m6 3 5 5-5 5"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
                {t("allTags")}
                <Tooltip
                  label={t("manageTags")}
                  events={{ hover: true, focus: true, touch: false }}
                >
                  <ActionIcon
                    className="manage-tags-button"
                    variant="subtle"
                    color="gray"
                    size="sm"
                    aria-label={t("manageTags")}
                    onClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      setManage(true);
                    }}
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      aria-hidden="true"
                    >
                      <path d="M4 7h6m4 0h6M4 17h10m4 0h2" />
                      <circle cx="12" cy="7" r="2" />
                      <circle cx="16" cy="17" r="2" />
                    </svg>
                  </ActionIcon>
                </Tooltip>
              </summary>
              {tags.isError ? (
                <Button onClick={() => tags.refetch()}>{t("retry")}</Button>
              ) : (
                <TagTree
                  tags={tree}
                  choose={(id, destination = "tag") => {
                    if (destination === "files") {
                      const next = { ...emptyFilters, tag_all: [id] };
                      setFilters(next);
                      go({ files: true, filters: next });
                    } else if (destination === "boxes") {
                      const next = { ...emptyBoxFilters, tag_all: [id] };
                      setBoxFilters(next);
                      go({ files: false, boxFilters: next });
                    } else {
                      setFilters({ ...emptyFilters });
                      go({ files: false, tag: id });
                    }
                  }}
                />
              )}
            </details>
          </div>
          <div className="sidebar-footer">
            <small>kist · {info.data?.version || "v0.1"}</small>
            <UnstyledButton onClick={() => setSettings(true)}>
              {t("settings")}
            </UnstyledButton>
          </div>
        </aside>
        <main
          className={
            !route.section && !selected && !route.files && !route.tag
              ? "boxes-page"
              : route.files
                ? "files-page"
                : undefined
          }
        >
          <header className="page-header">
            <div>
              <div className="box-title-line">
                <h1>
                  {selected && route.files && !route.section ? (
                    <UnstyledButton
                      className="box-root-link"
                      onClick={() => {
                        setFilters({ ...emptyFilters });
                        go({ box: route.box, files: true });
                      }}
                    >
                      <bdi>{selected.name}</bdi>
                    </UnstyledButton>
                  ) : (
                    <bdi>
                      {route.section
                        ? t(route.section)
                        : route.tag
                          ? selectedTag?.name || t("tags")
                          : selected?.name ||
                            t(route.files ? "allFiles" : "boxes")}
                    </bdi>
                  )}
                </h1>
                {selected &&
                  !route.section &&
                  (selected.start_at || selected.end_at) && (
                    <span className="box-title-date">
                      {selected.start_at
                        ? formatLocalDate(selected.start_at, prefs)
                        : ""}
                      {selected.end_at
                        ? ` — ${formatLocalDate(selected.end_at, prefs)}`
                        : ""}
                    </span>
                  )}
              </div>
              {route.files && <div ref={setInfoHost} />}
            </div>
            <div className="header-actions">
              {!route.section && !selected && !route.files && !route.tag && (
                <ActionIcon
                  variant="default"
                  size="lg"
                  aria-label={t("createBox")}
                  title={t("createBox")}
                  onClick={() => setEditor("new")}
                >
                  <span aria-hidden="true">＋</span>
                </ActionIcon>
              )}
              {route.files && selected && <span ref={setActionsHost} />}
              <Tooltip
                label={t("refresh")}
                events={{ hover: true, focus: true, touch: false }}
              >
                <ActionIcon
                  size="lg"
                  variant="subtle"
                  color="gray"
                  aria-label={t("refresh")}
                  onClick={() => client.invalidateQueries()}
                >
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    aria-hidden="true"
                  >
                    <path d="M20 7v5h-5M4 17v-5h5" />
                    <path d="M6.1 7a7 7 0 0 1 11.55-.9L20 12M4 12l2.35 5.9A7 7 0 0 0 17.9 17" />
                  </svg>
                </ActionIcon>
              </Tooltip>
              {!route.section && selected && (
                <Menu position="bottom-end" withinPortal>
                  <Menu.Target>
                    <Tooltip
                      label={t("moreActions")}
                      events={{ hover: true, focus: true, touch: false }}
                    >
                      <ActionIcon
                        size="lg"
                        variant="subtle"
                        color="gray"
                        aria-label={t("moreActions")}
                      >
                        <svg
                          width="20"
                          height="20"
                          viewBox="0 0 24 24"
                          fill="currentColor"
                          aria-hidden="true"
                        >
                          <circle cx="5" cy="12" r="1.7" />
                          <circle cx="12" cy="12" r="1.7" />
                          <circle cx="19" cy="12" r="1.7" />
                        </svg>
                      </ActionIcon>
                    </Tooltip>
                  </Menu.Target>
                  <Menu.Dropdown>{boxManagementActions}</Menu.Dropdown>
                </Menu>
              )}
            </div>
          </header>
          {route.box &&
            route.files &&
            currentFolder &&
            folderChain(folderList, currentFolder).length > 0 && (
              <FolderBreadcrumbs
                box={route.box}
                goRoot={() => navigate()}
                rootLabel={selected?.name}
                folders={folderList}
                current={currentFolder}
                choose={chooseFolder}
              />
            )}
          {info.data?.projection_dirty && (
            <p className="notice" role="status">
              {t("projectionDirty")}
            </p>
          )}
          {route.section && boxes.isPending && (
            <p role="status">{t("loading")}</p>
          )}
          {route.section && boxes.isError && (
            <div role="alert">
              <p>{errorMessage(boxes.error, t)}</p>
              <Button onClick={() => boxes.refetch()}>{t("retry")}</Button>
            </div>
          )}
          {route.section ? (
            <HistoryPage
              key={`${route.section}:${route.box || ""}:${route.file || ""}`}
              box={route.box}
              file={route.file}
              trash={route.section === "trash"}
              boxes={boxes.data?.boxes || []}
              scope={(box, file) =>
                go({
                  files: false,
                  section: file ? "history" : route.section,
                  box,
                  file,
                })
              }
            />
          ) : route.tag ? (
            tags.isPending ? (
              <p role="status">{t("loading")}</p>
            ) : tags.isError ? (
              <p role="alert">{errorMessage(tags.error, t)}</p>
            ) : !selectedTag ? (
              <p role="alert">{t("notFound")}</p>
            ) : (
              <TagResults
                key={route.tag}
                tag={route.tag}
                tags={tree}
                paused={!!route.file}
                openBox={navigate}
                openFile={(file) =>
                  go({
                    ...route,
                    file: file.id,
                    filters: { ...emptyFilters, tag_all: [route.tag!] },
                  })
                }
              />
            )
          ) : route.box && box.isPending ? (
            <p role="status">{t("loading")}</p>
          ) : route.box && box.isError ? (
            <div role="alert">
              <p>{errorMessage(box.error, t)}</p>
              <Button onClick={() => box.refetch()}>{t("retry")}</Button>
            </div>
          ) : route.files && (!route.box || selected) ? (
            <>
              <FiltersPanel
                toolbar={<span ref={setFileToolbarHost} />}
                saveSearch={
                  currentSearch && (
                    <SaveSearchButton definition={currentSearch} />
                  )
                }
                key={`filters:${route.box || "all"}`}
                value={filters}
                onChange={(next) => {
                  setFilters(next);
                  go({ ...route, filters: next }, true);
                }}
                tags={tree}
                boxes={route.box ? undefined : boxes.data?.boxes || []}
                boxesLoading={boxes.isPending}
                boxesError={boxes.isError}
                retryBoxes={() => void boxes.refetch()}
              />
              {route.box && folders.isPending ? (
                <p role="status">{t("loading")}</p>
              ) : route.box && folders.isError ? (
                <div role="alert">
                  <p>{errorMessage(folders.error, t)}</p>
                  <Button onClick={() => folders.refetch()}>
                    {t("retry")}
                  </Button>
                </div>
              ) : route.box &&
                (!currentFolder ||
                  !folderChain(folderList, currentFolder).length) ? (
                <p role="alert">{t("notFound")}</p>
              ) : (
                <FileBrowser
                  boxActions={boxManagementActions}
                  onSort={(sort) => {
                    const next = {
                      ...filters,
                      sort,
                      asc:
                        filters.sort === sort
                          ? !filters.asc
                          : sort === "name" || sort === "type",
                    };
                    setFilters(next);
                    go({ ...route, filters: next }, true);
                  }}
                  toolbarHost={fileToolbarHost}
                  actionsHost={actionsHost}
                  infoHost={infoHost}
                  key={route.box || "all"}
                  box={selected}
                  folder={queryFolder}
                  uploadFolder={currentFolder}
                  folders={
                    searching
                      ? []
                      : folderList.filter(
                          (folder) => folder.parent_id === currentFolder,
                        )
                  }
                  chooseFolder={chooseFolder}
                  searching={searching && !!route.box}
                  searchingSubfolder={
                    !!folderList.find((folder) => folder.id === currentFolder)
                      ?.parent_id
                  }
                  paused={!!route.file}
                  filters={filters}
                  tags={tree}
                  open={(file) => go({ ...route, file: file.id, filters })}
                />
              )}
            </>
          ) : (
            <>
              <FiltersPanel
                saveSearch={
                  currentSearch && (
                    <SaveSearchButton definition={currentSearch} />
                  )
                }
                entity="boxes"
                hideSort={boxMode === "list"}
                toolbar={
                  <DisplayModeControl
                    mode={boxMode}
                    change={setBoxMode}
                    dropdown
                  />
                }
                value={boxFilters}
                onChange={(next) => {
                  setBoxFilters(next);
                  go({ ...route, boxFilters: next }, true);
                }}
                tags={tree}
              />
              <BoxBrowser
                filters={boxFilters}
                onSort={(sort) => {
                  const next = {
                    ...boxFilters,
                    sort,
                    asc:
                      boxFilters.sort === sort
                        ? !boxFilters.asc
                        : sort === "name",
                  };
                  setBoxFilters(next);
                  go({ ...route, boxFilters: next }, true);
                }}
                open={navigate}
                paused={!!route.file}
                create={() => setEditor("new")}
              />
            </>
          )}
        </main>
        <DialogPresence>
          {trashing && (
            <BoxTrashDialog
              box={trashing}
              close={() => setTrashing(null)}
              done={() => {
                setTrashing(null);
                navigate();
              }}
            />
          )}
        </DialogPresence>
        <DialogPresence>
          {settings && <Settings open={settings} onOpenChange={setSettings} />}
        </DialogPresence>{" "}
        <DialogPresence>
          {editor && (
            <BoxEditor
              box={editor === "new" ? undefined : editor}
              close={() => setEditor(null)}
            />
          )}
        </DialogPresence>{" "}
        <DialogPresence>
          {manage && <TagManager tags={tree} close={() => setManage(false)} />}
        </DialogPresence>{" "}
      </div>
      {route.file && !route.section && (
        <Gallery
          box={route.box}
          folder={queryFolder}
          filters={
            route.tag ? { ...emptyFilters, tag_all: [route.tag] } : filters
          }
          change={(file) => go({ ...route, file }, true)}
          id={route.file}
          tags={tree}
          close={() => {
            if (window.history.state?.kistGallery) window.history.back();
            else go({ ...route, file: undefined }, true);
          }}
          history={() =>
            go({
              files: false,
              section: "history",
              file: route.file,
              box: route.box,
            })
          }
        />
      )}
    </>
  );
}
