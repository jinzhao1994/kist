import { Button } from "@mantine/core";
import type { FileDetail as FileData, Tag } from "./api";
import { TagAssignment } from "./Tags";
import { ExtractionStatus, useExtraction } from "./Extraction";
import { formatInstant, formatBytes, useI18n } from "./i18n";

// The gallery owns navigation and media; this panel contains file information.
export function FileDetail({
  data,
  tags,
  history,
}: {
  data: FileData;
  tags: Tag[];
  history?: () => void;
}) {
  const { t, prefs } = useI18n();
  const extraction = useExtraction();
  const file = data.file;
  return (
    <>
      <dl className="detail-facts">
        <dt>{t("path")}</dt>
        <dd>
          <bdi>{file.path || file.name}</bdi>
        </dd>
        <dt>{t("kind")}</dt>
        <dd>
          <bdi>{file.media_type || t("unknown")}</bdi>
        </dd>
        <dt>{t("fileSize")}</dt>
        <dd>{file.size == null ? "—" : formatBytes(file.size, prefs)}</dd>
        <dt>{t("capturedAt")}</dt>
        <dd>
          {file.captured_at ? formatInstant(file.captured_at, prefs) : "—"}
        </dd>
        <dt>{t("created")}</dt>
        <dd>{formatInstant(file.created_at, prefs)}</dd>
        <dt>{t("fileId")}</dt>
        <dd>
          <bdi>{file.id}</bdi>
        </dd>
      </dl>
      {history && <Button onClick={history}>{t("versionHistory")}</Button>}
      <TagAssignment
        target={file.id}
        kind="files"
        tags={tags}
        assigned={data.tags || []}
      />
      <ExtractionStatus files={[file]} state={extraction} />
      <h3>{t("metadata")}</h3>
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
  );
}
