import { lazy, Suspense } from "react";
const MapPreview = lazy(() => import("./MapPreview"));
import { FloatingTooltip } from "./FloatingTooltip";
import { AISupplement } from "./AISupplement";
import { MetadataLabel } from "./MetadataLabel";
import { StructuredFields } from "./StructuredFields";
import { MetadataValue } from "./MetadataValue";
import { MetadataGroup } from "./MetadataGroup";
import { CopyTextPreview } from "./CopyTextPreview";
import { useClientMode } from "./OfflineEntry";
import { OCRText } from "./AI";
import { supportsImageAI, supportsDescriptionAI } from "./media-profile";
import { Button, CopyButton, Stack, Tooltip } from "@mantine/core";
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
  const clientMode = useClientMode();
  const ocr = data.metadata?.find((row) => row.key === "ocr:text");
  const file = data.file;
  const imageTextInput = supportsImageAI(file);
  const canOCR =
    !clientMode && !data.extracted?.includes("ocr") && !ocr && imageTextInput;
  const descriptionInput = supportsDescriptionAI(file);
  const canDescribe =
    !clientMode &&
    !data.extracted?.includes("desc") &&
    !data.metadata?.some((row) => row.key === "desc:text") &&
    descriptionInput;
  const canFacts =
    !clientMode &&
    !data.extracted?.includes("facts") &&
    !data.metadata?.some((row) => row.key === "facts:data") &&
    descriptionInput;
  const canOrient =
    !clientMode &&
    imageTextInput &&
    !data.metadata?.some((row) => row.key === "orientation:clockwise_degrees");
  const displayID = combination?.folder.id || file.id;
  const labels: Record<string, string> = {
    "geo:address": t("geoAddress"),
    "geo:provider": t("ocrProvider"),
    "orientation:clockwise_degrees": t("orientationAngle"),
    "orientation:model": t("aiModel"),
    "orientation:recipe": t("ocrRecipe"),
    "orientation:confidence": t("orientationConfidence"),
    "orientation:method": t("orientationMethod"),
    "exif:ApertureValue": t("exifApertureValue"),
    "exif:BrightnessValue": t("exifBrightnessValue"),
    "exif:ColorSpace": t("exifColorSpace"),
    "exif:ComponentsConfiguration": t("exifComponentsConfiguration"),
    "exif:CompositeImage": t("exifCompositeImage"),
    "exif:CreateDate": t("exifCreateDate"),
    "exif:CustomRendered": t("exifCustomRendered"),
    "exif:DateTimeOriginal": t("exifDateTimeOriginal"),
    "exif:DigitalZoomRatio": t("exifDigitalZoomRatio"),
    "exif:ExifVersion": t("exifExifVersion"),
    "exif:ExposureCompensation": t("exifExposureCompensation"),
    "exif:ExposureMode": t("exifExposureMode"),
    "exif:ExposureProgram": t("exifExposureProgram"),
    "exif:ExposureTime": t("exifExposureTime"),
    "exif:FNumber": t("exifFNumber"),
    "exif:Flash": t("exifFlash"),
    "exif:FlashpixVersion": t("exifFlashpixVersion"),
    "exif:FocalLength": t("exifFocalLength"),
    "exif:FocalLengthIn35mmFormat": t("exifFocalLengthIn35mmFormat"),
    "exif:FocalPlaneResolutionUnit": t("exifFocalPlaneResolutionUnit"),
    "exif:FocalPlaneXResolution": t("exifFocalPlaneXResolution"),
    "exif:FocalPlaneYResolution": t("exifFocalPlaneYResolution"),
    "exif:HostComputer": t("exifHostComputer"),
    "exif:ISO": t("exifISO"),
    "exif:LensInfo": t("exifLensInfo"),
    "exif:LensMake": t("exifLensMake"),
    "exif:LensModel": t("exifLensModel"),
    "exif:LensSerialNumber": t("exifLensSerialNumber"),
    "exif:MaxApertureValue": t("exifMaxApertureValue"),
    "exif:MeteringMode": t("exifMeteringMode"),
    "exif:ModifyDate": t("exifModifyDate"),
    "exif:OffsetTime": t("exifOffsetTime"),
    "exif:OffsetTimeDigitized": t("exifOffsetTimeDigitized"),
    "exif:OffsetTimeOriginal": t("exifOffsetTimeOriginal"),
    "exif:Orientation": t("exifOrientation"),
    "exif:RecommendedExposureIndex": t("exifRecommendedExposureIndex"),
    "exif:SceneCaptureType": t("exifSceneCaptureType"),
    "exif:SceneType": t("exifSceneType"),
    "exif:SensingMethod": t("exifSensingMethod"),
    "exif:SensitivityType": t("exifSensitivityType"),
    "exif:SerialNumber": t("exifSerialNumber"),
    "exif:ShutterSpeedValue": t("exifShutterSpeedValue"),
    "exif:SubSecTime": t("exifSubSecTime"),
    "exif:SubSecTimeDigitized": t("exifSubSecTimeDigitized"),
    "exif:SubSecTimeOriginal": t("exifSubSecTimeOriginal"),
    "exif:SubjectArea": t("exifSubjectArea"),
    "exif:TileLength": t("exifTileLength"),
    "exif:TileWidth": t("exifTileWidth"),
    "exif:WhiteBalance": t("exifWhiteBalance"),
    "exif:YCbCrPositioning": t("exifYCbCrPositioning"),
    "gps:Altitude": t("gpsAltitude"),
    "gps:AltitudeRef": t("gpsAltitudeRef"),
    "gps:DateStamp": t("gpsDateStamp"),
    "gps:DestBearing": t("gpsDestBearing"),
    "gps:DestBearingRef": t("gpsDestBearingRef"),
    "gps:HPositioningError": t("gpsHPositioningError"),
    "gps:ImgDirection": t("gpsImgDirection"),
    "gps:ImgDirectionRef": t("gpsImgDirectionRef"),
    "gps:Latitude": t("gpsLatitude"),
    "gps:LatitudeRef": t("gpsLatitudeRef"),
    "gps:Longitude": t("gpsLongitude"),
    "gps:LongitudeRef": t("gpsLongitudeRef"),
    "gps:Speed": t("gpsSpeed"),
    "gps:SpeedRef": t("gpsSpeedRef"),
    "gps:TimeStamp": t("gpsTimeStamp"),

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
    "facts:model": t("aiModel"),
    "facts:recipe": t("ocrRecipe"),
    "desc:text": t("descText"),
    "desc:model": t("aiModel"),
    "desc:recipe": t("ocrRecipe"),
    "ocr:text": t("ocrText"),
    "ocr:model": t("aiModel"),
    "ocr:recipe": t("ocrRecipe"),
  };
  const metadataLabel = (key: string) =>
    labels[key] || key.slice(key.indexOf(":") + 1);
  const metadataValue = (key: string, value: string, source: string) => {
    if (key === "orientation:clockwise_degrees") return `${value}°`;
    if (key === "orientation:method")
      return file.image_rotation_manual && source !== "user"
        ? t("orientationManual")
        : {
            overview: t("orientationOverview"),
            regions: t("orientationRegions"),
            uncertain: t("orientationUncertain"),
          }[value] || value;
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
    if (key === "geo:address")
      return value ? t("geoNearby", { address: value }) : t("geoNotFound");
    if (key === "geo:provider" && value === "photon") return "Photon";
    return value || "—";
  };
  const groups = new Map<string, NonNullable<FileData["metadata"]>>();
  for (const row of data.metadata || []) {
    // GPS is a display group; its extractor provenance stays unchanged.
    const group = row.key.startsWith("orientation:")
      ? "orientation"
      : row.key.startsWith("gps:") || row.key.startsWith("geo:")
        ? "gps"
        : row.source === "user"
          ? row.key.split(":", 1)[0]
          : row.source || "";
    const rows = groups.get(group) || [];
    rows.push(row);
    groups.set(group, rows);
  }
  if (data.gps && !groups.has("gps")) groups.set("gps", []);
  if (canOrient && !groups.has("orientation")) groups.set("orientation", []);
  if (canOCR && !groups.has("ocr")) groups.set("ocr", []);
  if (canFacts && !groups.has("facts")) groups.set("facts", []);
  if (canDescribe && !groups.has("desc")) groups.set("desc", []);
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
                className="copy-value copy-value--identifier"
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
      {(groups.size > 0 ||
        !!file.extraction_pending?.length ||
        extraction.busy ||
        extraction.failed) && (
        <section className="file-metadata">
          <h3>{t("metadata")}</h3>
          <ExtractionStatus files={[file]} state={extraction} />
          {Array.from(groups, ([source, rows]) => (
            <MetadataGroup
              key={`${file.id}/${source}`}
              fileID={file.id}
              empty={
                rows.length === 0 &&
                ["ocr", "desc", "orientation", "facts"].includes(source)
              }
              help={
                source === "gps" && data.gps ? t("geoCaptureHint") : undefined
              }
              secondary={
                source === "gps" && data.gps && !clientMode
                  ? {
                      title: t("geoTitle"),
                      extractor: "geo",
                      missing: !data.extracted?.includes("geo"),
                    }
                  : undefined
              }
              title={
                source === "exif"
                  ? "EXIF"
                  : source === "gps"
                    ? "GPS"
                    : source === "file"
                      ? "File"
                      : source === "media"
                        ? "Media"
                        : source === "ocr"
                          ? "OCR"
                          : source === "facts"
                            ? t("factsTitle")
                            : source === "desc"
                              ? t("descTitle")
                              : source === "orientation"
                                ? t("orientationTitle")
                                : source
              }
              extractor={
                !(source === "orientation" && file.image_rotation_manual) &&
                (rows.length ||
                  ["ocr", "desc", "orientation", "facts"].includes(source)) &&
                [
                  "exif",
                  "gps",
                  "media",
                  "ocr",
                  "desc",
                  "orientation",
                  "facts",
                ].includes(source) &&
                !(
                  clientMode &&
                  ["ocr", "desc", "orientation", "facts"].includes(source)
                )
                  ? source === "gps"
                    ? "exif"
                    : source
                  : undefined
              }
            >
              <Stack>
                {source === "gps" && data.gps && (
                  <>
                    <div className="metadata-map">
                      <Suspense fallback={<span>{t("mapLoading")}</span>}>
                        <MapPreview
                          point={data.gps}
                          name={t("geoMap")}
                          detail
                        />
                      </Suspense>
                    </div>
                  </>
                )}
                <dl className="metadata-list">
                  {rows.map((row) =>
                    row.key === "facts:data" ? null : row.key === "ocr:hint" ||
                      row.key === "desc:hint" ||
                      row.key === "facts:hint" ? (
                      <AISupplement
                        key={`${file.id}/${file.blob_key}/${row.key}`}
                        fileID={file.id}
                        blobKey={file.blob_key}
                        kind={
                          row.key === "ocr:hint"
                            ? "ocr"
                            : row.key === "facts:hint"
                              ? "facts"
                              : "desc"
                        }
                        row={row}
                        manual={rows.some(
                          (item) =>
                            item.key === `${source}:text` &&
                            item.source === "user",
                        )}
                        canRun={
                          !clientMode &&
                          (source === "desc" || source === "facts"
                            ? descriptionInput
                            : imageTextInput)
                        }
                      />
                    ) : (
                      <div key={row.key}>
                        <MetadataLabel
                          metadataKey={row.key}
                          label={metadataLabel(row.key)}
                        />
                        <dd>
                          <MetadataValue
                            key={`${file.id}/${file.blob_key}/${row.key}`}
                            fileID={file.id}
                            blobKey={file.blob_key}
                            row={row}
                            label={metadataLabel(row.key)}
                          >
                            {row.key === "ocr:text" ? (
                              <OCRText text={row.value} />
                            ) : row.key === "desc:text" ? (
                              <CopyTextPreview
                                text={row.value}
                                copyLabel="descCopyText"
                              />
                            ) : row.key === "file:mtime" ? (
                              <Tooltip
                                label={row.value}
                                position="bottom-end"
                                offset={6}
                                classNames={{ tooltip: "metadata-tooltip" }}
                                openDelay={0}
                                transitionProps={{ duration: 0 }}
                                events={{
                                  hover: false,
                                  focus: true,
                                  touch: false,
                                }}
                              >
                                <bdi tabIndex={0}>
                                  <FloatingTooltip
                                    label={row.value}
                                    position="bottom-start"
                                    offset={16}
                                    role="tooltip"
                                    classNames={{ tooltip: "metadata-tooltip" }}
                                  >
                                    <span>
                                      {metadataValue(
                                        row.key,
                                        row.value,
                                        row.source,
                                      )}
                                    </span>
                                  </FloatingTooltip>
                                </bdi>
                              </Tooltip>
                            ) : (
                              <bdi>
                                {metadataValue(row.key, row.value, row.source)}
                              </bdi>
                            )}
                          </MetadataValue>
                        </dd>
                      </div>
                    ),
                  )}
                </dl>
                {source === "facts" &&
                  rows
                    .filter((row) => row.key === "facts:data")
                    .map((row) => (
                      <StructuredFields
                        key={`${file.id}/${file.blob_key}/${row.key}`}
                        fileID={file.id}
                        blobKey={file.blob_key}
                        row={row}
                      />
                    ))}
                {source === "gps" &&
                  !data.gps &&
                  rows.some((row) => row.key.startsWith("geo:")) && (
                    <span className="metadata-help">
                      <a
                        href="https://www.openstreetmap.org/copyright"
                        target="_blank"
                        rel="noreferrer"
                      >
                        © OpenStreetMap contributors
                      </a>
                    </span>
                  )}
              </Stack>
            </MetadataGroup>
          ))}
        </section>
      )}
    </div>
  );
}
