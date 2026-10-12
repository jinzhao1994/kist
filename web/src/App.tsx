import { UnstyledButton, Button } from "@mantine/core";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Box } from "./api";
import { formatLocalDate, useI18n } from "./i18n";
import { Settings } from "./Settings";
import { BoxTrashDialog } from "./BoxTrashDialog";
import { BoxEditor } from "./BoxEditor";
import { TagManager, TagTree } from "./Tags";
import { FiltersPanel, emptyFilters, type Filters } from "./Filters";
import { FileBrowser } from "./FileBrowser";
import { FileDetail } from "./FileDetail";
import { useNavigation } from "./navigation";
import { errorMessage } from "./mutations";
import { HistoryPage } from "./History";
export function App() {
  const { t, prefs } = useI18n();
  const { route, go } = useNavigation();
  const [settings, setSettings] = useState(false);
  const [editor, setEditor] = useState<Box | "new" | null>(null);
  const [trashing, setTrashing] = useState<Box | null>(null);
  const [manage, setManage] = useState(false);
  const [filters, setFilters] = useState<Filters>({ ...emptyFilters });
  const boxes = useQuery({
    queryKey: ["boxes"],
    queryFn: ({ signal }) => api.boxes(signal),
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
  const selected = boxes.data?.boxes?.find((box) => box.id === route.box);
  const tree = tags.data?.tags || [];
  const navigate = (box?: Box, files = false) => {
    setFilters({ ...emptyFilters });
    go({ box: box?.id, files: !!box || files });
  };
  return (
    <div className="app-shell">
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
            className={`nav-link ${route.files && !route.box ? "active" : ""}`}
            onClick={() => navigate(undefined, true)}
          >
            ▦ <span>{t("allFiles")}</span>
          </UnstyledButton>
          <UnstyledButton
            className={`nav-link ${!route.files && !route.section ? "active" : ""}`}
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
                setFilters({ ...emptyFilters, tag_all: [id] });
                go({ files: true, box: route.section ? undefined : route.box });
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
                  : selected?.name || t(route.files ? "allFiles" : "boxes")}
              </bdi>
            </h1>
          </div>
          <div className="header-actions">
            <Button onClick={() => client.invalidateQueries()}>
              {t("refresh")}
            </Button>
            {!route.section && selected && (
              <Button
                onClick={() =>
                  go({ files: false, section: "history", box: selected.id })
                }
              >
                {t("history")}
              </Button>
            )}
            {!route.section &&
              (selected ? (
                <>
                  <Button onClick={() => setEditor(selected)}>
                    {t("editBox")}
                  </Button>
                  <Button
                    color="red"
                    variant="subtle"
                    onClick={() => setTrashing(selected)}
                  >
                    {t("moveBoxToTrash")}
                  </Button>
                </>
              ) : (
                !route.files && (
                  <Button variant="filled" onClick={() => setEditor("new")}>
                    {t("createBox")}
                  </Button>
                )
              ))}
          </div>
        </header>
        {info.data?.projection_dirty && (
          <p className="notice" role="status">
            {t("projectionDirty")}
          </p>
        )}
        {boxes.isPending && <p role="status">{t("loading")}</p>}
        {boxes.isError && (
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
        ) : route.box && boxes.data && !selected ? (
          <p role="alert">{t("notFound")}</p>
        ) : route.files && (!route.box || selected) ? (
          <>
            <FiltersPanel value={filters} onChange={setFilters} tags={tree} />
            <FileBrowser
              key={route.box || "all"}
              box={selected}
              paused={!!route.file}
              filters={filters}
              tags={tree}
              open={(file) => go({ ...route, file: file.id })}
            />
          </>
        ) : (
          <>
            {boxes.data && !boxes.data.boxes?.length && (
              <section className="empty-state">
                <span className="archive-mark" aria-hidden="true">
                  ▤
                </span>
                <h2>{t("emptyBoxes")}</h2>
                <p>{t("emptyBoxesHint")}</p>
                <Button variant="filled" onClick={() => setEditor("new")}>
                  {t("createBox")}
                </Button>
              </section>
            )}
            <div className="box-grid">
              {boxes.data?.boxes?.map((box) => (
                <UnstyledButton
                  className="box-card"
                  key={box.id}
                  onClick={() => navigate(box)}
                >
                  <span className="box-icon" aria-hidden="true">
                    ▤
                  </span>
                  <h2>
                    <bdi>{box.name}</bdi>
                  </h2>
                  <p>
                    <bdi>{box.description}</bdi>
                  </p>
                  <time>
                    {box.start_at ? formatLocalDate(box.start_at, prefs) : ""}
                    {box.end_at
                      ? ` — ${formatLocalDate(box.end_at, prefs)}`
                      : ""}
                  </time>
                </UnstyledButton>
              ))}
            </div>
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
      {manage && <TagManager tags={tree} close={() => setManage(false)} />}{" "}
      {route.file && !route.section && (
        <FileDetail
          key={route.file}
          id={route.file}
          tags={tree}
          close={() => go({ ...route, file: undefined })}
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
    </div>
  );
}
