import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { formatLocalDate, useI18n } from "./i18n";
import { Settings } from "./Settings";
export function App() {
  const { t, prefs } = useI18n();
  const [settings, setSettings] = useState(false);
  const boxes = useQuery({
    queryKey: ["boxes"],
    queryFn: ({ signal }) => api.boxes(signal),
  });
  const info = useQuery({
    queryKey: ["info"],
    queryFn: ({ signal }) => api.info(signal),
  });
  const client = useQueryClient();
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="wordmark" href="/">
          kist
          <span className="brand-dot" />
        </a>
        <p className="sidebar-caption">{t("archive")}</p>
        <nav aria-label={t("archive")}>
          <a className="nav-link active" href="/">
            ▤ <span>{t("boxes")}</span>
          </a>
        </nav>
        <div className="sidebar-footer">
          <button onClick={() => setSettings(true)}>{t("settings")}</button>
          <small>kist · {info.data?.version || "v0.1"}</small>
        </div>
      </aside>
      <main>
        <header className="page-header">
          <div>
            <span className="eyebrow">{t("archive")}</span>
            <h1>{t("boxes")}</h1>
          </div>
          <button onClick={() => client.invalidateQueries()}>
            {t("refresh")}
          </button>
        </header>
        {info.data?.projection_dirty && (
          <p className="notice" role="status">
            {t("projectionDirty")}
          </p>
        )}
        {boxes.isPending && <p role="status">{t("loading")}</p>}
        {boxes.isError && (
          <div role="alert">
            <p>{t("error")}</p>
            <button onClick={() => boxes.refetch()}>{t("retry")}</button>
          </div>
        )}
        {boxes.data && !boxes.data.boxes?.length && (
          <section className="empty-state">
            <span className="archive-mark" aria-hidden="true">
              ▤
            </span>
            <h2>{t("emptyBoxes")}</h2>
            <p>{t("emptyBoxesHint")}</p>
          </section>
        )}
        <div className="box-grid">
          {boxes.data?.boxes?.map((box) => (
            <article className="box-card" key={box.id}>
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
              </time>
            </article>
          ))}
        </div>
      </main>
      {settings && <Settings open={settings} onOpenChange={setSettings} />}
    </div>
  );
}
