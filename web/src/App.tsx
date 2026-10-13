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
import { useI18n } from "./i18n";
import { Settings } from "./Settings";
import { BoxTrashDialog } from "./BoxTrashDialog";
import { BoxBrowser } from "./BoxBrowser";
import { TagResults } from "./TagResults";
import { flattenTags } from "./TagPath";
import { BoxEditor } from "./BoxEditor";
import { TagManager, TagTree } from "./Tags";
import {
  FiltersPanel,
  emptyFilters,
  emptyBoxFilters,
  type BoxFilters,
  type Filters,
} from "./Filters";
import { FileBrowser } from "./FileBrowser";
import { Gallery } from "./Gallery";
import { useNavigation } from "./navigation";
import { errorMessage } from "./mutations";
import { HistoryPage } from "./History";
export function App() {
  const { t } = useI18n();
  const { route, go } = useNavigation();
  const [settings, setSettings] = useState(false);
  const [editor, setEditor] = useState<Box | "new" | null>(null);
  const [trashing, setTrashing] = useState<Box | null>(null);
  const [manage, setManage] = useState(false);
  const [filters, setFilters] = useState<Filters>({
    ...emptyFilters,
    ...route.filters,
  });
  useEffect(() => {
    if (route.filters) setFilters({ ...emptyFilters, ...route.filters });
  }, [route.filters]);
  const [boxFilters, setBoxFilters] = useState<BoxFilters>({
    ...emptyBoxFilters,
  });
  const box = useQuery({
    queryKey: ["boxes", "detail", route.box],
    queryFn: ({ signal }) => api.box(route.box!, signal),
    enabled: !!route.box,
  });
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
          <p className="sidebar-caption">{t("archive")}</p>
          <nav aria-label={t("archive")}>
            <UnstyledButton
              className={`nav-link ${route.files && !route.box && !route.tag ? "active" : ""}`}
              onClick={() => navigate(undefined, true)}
            >
              ▦ <span>{t("allFiles")}</span>
            </UnstyledButton>
            <UnstyledButton
              className={`nav-link ${!route.files && !route.section && !route.tag ? "active" : ""}`}
              onClick={() => navigate()}
            >
              ▤ <span>{t("boxes")}</span>
            </UnstyledButton>
            {(["trash", "history"] as const).map((section) => (
              <UnstyledButton
                key={section}
                className={`nav-link ${route.section === section ? "active" : ""}`}
                onClick={() => go({ files: false, section })}
              >
                {section === "history" ? "◷" : "⌫"} <span>{t(section)}</span>
              </UnstyledButton>
            ))}
          </nav>
          <details className="sidebar-tags" open>
            <summary>{t("tags")}</summary>
            {tags.isError ? (
              <Button onClick={() => tags.refetch()}>{t("retry")}</Button>
            ) : (
              <TagTree
                tags={tree}
                choose={(id) => {
                  setFilters({ ...emptyFilters });
                  go({ files: false, tag: id });
                }}
              />
            )}
            <UnstyledButton
              className="text-button"
              onClick={() => setManage(true)}
            >
              {t("manageTags")}
            </UnstyledButton>
          </details>
          <div className="sidebar-footer">
            <Button onClick={() => setSettings(true)}>{t("settings")}</Button>
            <small>kist · {info.data?.version || "v0.1"}</small>
          </div>
        </aside>
        <main>
          <header className="page-header">
            <div>
              <span className="eyebrow">{t("archive")}</span>
              <h1>
                <bdi>
                  {route.section
                    ? t(route.section)
                    : route.tag
                      ? selectedTag?.name || t("tags")
                      : selected?.name || t(route.files ? "allFiles" : "boxes")}
                </bdi>
              </h1>
            </div>
            <div className="header-actions">
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
                  <Menu.Dropdown>
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
                    <Menu.Item onClick={() => setEditor(selected)}>
                      {t("editBox")}
                    </Menu.Item>
                    <Menu.Divider />
                    <Menu.Item
                      color="red"
                      onClick={() => setTrashing(selected)}
                    >
                      {t("moveBoxToTrash")}
                    </Menu.Item>
                  </Menu.Dropdown>
                </Menu>
              )}
              {!route.section && !selected && !route.files && !route.tag && (
                <Button variant="filled" onClick={() => setEditor("new")}>
                  {t("createBox")}
                </Button>
              )}
            </div>
          </header>
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
                key={`filters:${route.box || "all"}`}
                value={filters}
                onChange={setFilters}
                tags={tree}
                boxes={route.box ? undefined : boxes.data?.boxes || []}
                boxesLoading={boxes.isPending}
                boxesError={boxes.isError}
                retryBoxes={() => void boxes.refetch()}
              />
              <FileBrowser
                key={route.box || "all"}
                box={selected}
                paused={!!route.file}
                filters={filters}
                tags={tree}
                open={(file) => go({ ...route, file: file.id, filters })}
              />
            </>
          ) : (
            <>
              <FiltersPanel
                entity="boxes"
                value={boxFilters}
                onChange={setBoxFilters}
                tags={tree}
              />
              <BoxBrowser
                filters={boxFilters}
                open={navigate}
                paused={!!route.file}
                create={() => setEditor("new")}
              />
            </>
          )}
        </main>
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
        {settings && <Settings open={settings} onOpenChange={setSettings} />}{" "}
        {editor && (
          <BoxEditor
            box={editor === "new" ? undefined : editor}
            close={() => setEditor(null)}
          />
        )}{" "}
        {manage && (
          <TagManager tags={tree} close={() => setManage(false)} />
        )}{" "}
      </div>
      {route.file && !route.section && (
        <Gallery
          box={route.box}
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
