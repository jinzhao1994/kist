import { Button, CopyButton, Tooltip } from "@mantine/core";
import type { components } from "./api-schema";
import type { FileDetail as FileData, Tag } from "./api";
import { TagAssignment } from "./Tags";
import { ExtractionStatus, useExtraction } from "./Extraction";
import { formatInstant, formatBytes, useI18n } from "./i18n";

// The gallery owns navigation and media; this panel contains file information.
export function FileDetail({
  data,
  tags,
  history,
  combination,
}: {
  data: FileData;
  tags: Tag[];
  history?: () => void;
  combination?: components["schemas"]["FilesetDetail"];
}) {
  const { t, prefs } = useI18n();
  const extraction = useExtraction();
  const file = data.file;
  const displayID = combination?.folder.id || file.id;
  const labels: Record<string, string> = {
    "exif:ExifImageHeight": t("imageHeight"),
    "exif:ExifImageWidth": t("imageWidth"),
    "exif:ImageHeight": t("imageHeight"),
    "exif:ImageWidth": t("imageWidth"),
    "exif:ResolutionUnit": t("resolutionUnit"),
    "exif:UserComment": t("userComment"),
    "exif:Make": t("cameraMake"),
    "exif:Model": t("cameraModel"),
    "exif:Software": t("software"),
    "file:size": t("fileSize"),
    "exif:XResolution": t("horizontalResolution"),
    "exif:YResolution": t("verticalResolution"),
    "media:mime": t("mimeType"),
    "media:kind": t("mediaKind"),
    "file:mtime": t("modified"),
  };
  const metadataValue = (key: string, value: string) => {
    if (key === "file:mtime" && /^-?\d+$/.test(value)) {
      // Import records file modification times as Unix nanoseconds.
      const date = new Date(Number(BigInt(value) / 1_000_000n));
      if (Number.isFinite(date.getTime())) {
        return formatInstant(date.toISOString(), prefs);
      }
    }
    if (key === "file:size" && Number.isSafeInteger(Number(value))) {
      return formatBytes(Number(value), prefs);
    }
    return value || "—";
  };
  const groups = new Map<string, NonNullable<FileData["metadata"]>>();
  for (const row of data.metadata || []) {
    const source = row.source || "";
    const rows = groups.get(source) || [];
    rows.push(row);
    groups.set(source, rows);
  }
  return (
    <div className="file-info-panel">
      <div className="file-info-heading">
        <h3>{t("basicInfo")}</h3>
        {history && (
          <Button variant="subtle" size="compact-xs" onClick={history}>
            {t(combination ? "filesetConfigHistory" : "versionHistory")}
          </Button>
        )}
      </div>
      <dl className="detail-facts">
        <dt>{t(combination ? "name" : "path")}</dt>
        <dd>
          <bdi>{combination?.folder.name || file.path || file.name}</bdi>
        </dd>
        <dt>{t("kind")}</dt>
        <dd>
          <bdi>
            {combination
              ? t(`fileset_${combination.fileset.kind}` as "fileset_live_photo")
              : file.media_type || t("unknown")}
          </bdi>
        </dd>
        <dt>{t("fileSize")}</dt>
        <dd>
          {file.size == null
            ? "—"
            : formatBytes(combination?.folder.total_size ?? file.size, prefs)}
        </dd>
        <dt>{t("capturedAt")}</dt>
        <dd>
          {file.captured_at ? formatInstant(file.captured_at, prefs) : "—"}
        </dd>
        <dt>{t("created")}</dt>
        <dd>
          {formatInstant(
            combination?.folder.created_at || file.created_at,
            prefs,
          )}
        </dd>
        {combination && (
          <>
            <dt>{t("filesetPrimary")}</dt>
            <dd>
              <bdi>{file.name}</bdi>
            </dd>
          </>
        )}
        <dt>{t(combination ? "filesetId" : "fileId")}</dt>
        <dd>
          <CopyButton value={displayID}>
            {({ copied, copy }) => (
              <button
                type="button"
                className="file-id-copy"
                onClick={copy}
                title={displayID}
                aria-label={t(
                  copied
                    ? "copied"
                    : combination
                      ? "copyFilesetId"
                      : "copyFileId",
                )}
              >
                <bdi>{displayID}</bdi>
                <span>{t(copied ? "copied" : "copy")}</span>
              </button>
            )}
          </CopyButton>
        </dd>
      </dl>
      <TagAssignment
        showEmpty
        collapsible
        target={file.id}
        kind="files"
        tags={tags}
        assigned={data.tags || []}
      />
      <ExtractionStatus files={[file]} state={extraction} />
      {!!data.metadata?.length && (
        <section className="file-metadata">
          <h3>{t("metadata")}</h3>
          {Array.from(groups, ([source, rows]) => (
            <section className="file-metadata-group" key={source}>
              {source && (
                <h4>
                  <bdi>
                    {source === "exif"
                      ? "EXIF"
                      : source === "file"
                        ? "File"
                        : source === "media"
                          ? "Media"
                          : source}
                  </bdi>
                </h4>
              )}
              <dl className="metadata-list">
                {rows.map((row) => (
                  <div key={row.key}>
                    <Tooltip
                      label={row.key}
                      position="bottom-start"
                      offset={8}
                      classNames={{ tooltip: "metadata-tooltip" }}
                      openDelay={0}
                      transitionProps={{ duration: 0 }}
                      events={{ hover: false, focus: true, touch: false }}
                    >
                      <dt tabIndex={0}>
                        <Tooltip.Floating
                          label={row.key}
                          position="bottom-start"
                          offset={16}
                          role="tooltip"
                          classNames={{ tooltip: "metadata-tooltip" }}
                        >
                          <bdi>{labels[row.key] || row.key}</bdi>
                        </Tooltip.Floating>
                      </dt>
                    </Tooltip>
                    <dd>
                      {row.key === "file:mtime" ? (
                        <Tooltip
                          label={row.value}
                          position="bottom-end"
                          offset={6}
                          classNames={{ tooltip: "metadata-tooltip" }}
                          openDelay={0}
                          transitionProps={{ duration: 0 }}
                          events={{ hover: false, focus: true, touch: false }}
                        >
                          <bdi tabIndex={0}>
                            <Tooltip.Floating
                              label={row.value}
                              position="bottom-start"
                              offset={16}
                              role="tooltip"
                              classNames={{ tooltip: "metadata-tooltip" }}
                            >
                              <span>{metadataValue(row.key, row.value)}</span>
                            </Tooltip.Floating>
                          </bdi>
                        </Tooltip>
                      ) : (
                        <bdi>{metadataValue(row.key, row.value)}</bdi>
                      )}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </section>
      )}
    </div>
  );
}
