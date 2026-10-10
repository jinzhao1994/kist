import { useQuery } from "@tanstack/react-query";
import { api, contentURL, type Tag } from "./api";
import { Dialog } from "./Dialog";
import { Media } from "./Media";
import { TagAssignment } from "./Tags";
import { ExtractionStatus, useExtraction } from "./Extraction";
import { formatInstant, formatBytes, useI18n } from "./i18n";
import { errorMessage } from "./mutations";
export function FileDetail({
  id,
  tags,
  close,
  history,
}: {
  id: string;
  tags: Tag[];
  close: () => void;
  history?: () => void;
}) {
  const { t, prefs } = useI18n();
  const extraction = useExtraction();
  const query = useQuery({
    queryKey: ["file", id],
    queryFn: ({ signal }) => api.file(id, signal),
  });
  const data = query.data;
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
      title={data?.file.name || t("details")}
      className="detail-dialog"
    >
      {query.isPending && <p role="status">{t("loading")}</p>}
      {query.isError && (
        <div role="alert">
          <p>{errorMessage(query.error, t)}</p>
          <button onClick={() => query.refetch()}>{t("retry")}</button>
        </div>
      )}
      {data && (
        <>
          <div className="detail-layout">
            <div className="detail-preview">
              <Media key={id} file={data.file} detail />
              <a
                className="button-link"
                href={contentURL(data.file, true)}
                download
              >
                {t("download")}
              </a>
            </div>
            <div className="detail-facts">
              <dl>
                <dt>{t("path")}</dt>
                <dd>
                  <bdi>{data.file.path || data.file.name}</bdi>
                </dd>
                <dt>{t("kind")}</dt>
                <dd>
                  <bdi>{data.file.media_type || t("unknown")}</bdi>
                </dd>
                <dt>{t("capturedAt")}</dt>
                <dd>
                  {data.file.captured_at
                    ? formatInstant(data.file.captured_at, prefs)
                    : "—"}
                </dd>
                <dt>{t("created")}</dt>
                <dd>{formatInstant(data.file.created_at, prefs)}</dd>
                <dt>{t("fileId")}</dt>
                <dd>
                  <bdi>{id}</bdi>
                </dd>
              </dl>
              <TagAssignment
                target={id}
                kind="files"
                tags={tags}
                assigned={data.tags || []}
              />
            </div>
          </div>
          <ExtractionStatus files={[data.file]} state={extraction} />
          <h3>{t("metadata")}</h3>
          {history && <button onClick={history}>{t("versionHistory")}</button>}
          <dl className="metadata-list">
            {data.metadata?.map((row) => (
              <div key={row.key}>
                <dt>
                  <bdi>{row.key}</bdi>
                </dt>
                <dd>
                  <bdi>
                    {row.key === "file:size" &&
                    Number.isSafeInteger(Number(row.value))
                      ? formatBytes(Number(row.value), prefs)
                      : row.value}
                  </bdi>
                  <small>
                    <bdi>{row.source}</bdi>
                  </small>
                </dd>
              </div>
            ))}
          </dl>
        </>
      )}
    </Dialog>
  );
}
