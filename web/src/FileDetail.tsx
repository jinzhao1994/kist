import { CopyTextPreview } from "./CopyTextPreview";
import { useClientMode } from "./OfflineEntry";
import { OCRAction, OCRText } from "./AI";
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
  // A filename is only a UI hint until basic metadata is available. The
  // backend validates image bytes before sending anything to a provider.
  const imageTextInput = file.media_type
    ? ["image/jpeg", "image/png", "image/webp"].includes(file.media_type)
    : /\.(?:jpe?g|png|webp)$/i.test(file.name);
  const canOCR =
    !clientMode && !data.extracted?.includes("ocr") && !ocr && imageTextInput;
  const canDescribe =
    !clientMode &&
    !data.extracted?.includes("desc") &&
    !data.metadata?.some((row) => row.key === "desc:text") &&
    imageTextInput;
  const displayID = combination?.folder.id || file.id;
  const labels: Record<string, string> = {
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
    "desc:text": t("descText"),
    "desc:model": t("aiModel"),
    "desc:provider": t("ocrProvider"),
    "desc:recipe": t("ocrRecipe"),
    "desc:input_sha256": t("ocrInputHash"),
    "desc:language": t("descLanguage"),
    "ocr:text": t("ocrText"),
    "ocr:model": t("aiModel"),
    "ocr:provider": t("ocrProvider"),
    "ocr:recipe": t("ocrRecipe"),
    "ocr:input_sha256": t("ocrInputHash"),
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
    if (
      (key === "ocr:provider" || key === "desc:provider") &&
      value === "openai"
    )
      return "OpenAI";
    return value || "—";
  };
  const groups = new Map<string, NonNullable<FileData["metadata"]>>();
  for (const row of data.metadata || []) {
    const source = row.source || "";
    const rows = groups.get(source) || [];
    rows.push(row);
    groups.set(source, rows);
  }
  if (canOCR && !groups.has("ocr")) groups.set("ocr", []);
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
                          : source === "ocr"
                            ? "OCR"
                            : source === "desc"
                              ? t("descTitle")
                              : source}
                  </bdi>
                </h4>
              )}
              <Stack>
                {source === "ocr" && canOCR && (
                  <OCRAction key={file.id} fileID={file.id} />
                )}
                {source === "desc" && canDescribe && (
                  <OCRAction key={file.id} fileID={file.id} kind="desc" />
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
                        {row.key === "ocr:text" ? (
                          <OCRText text={row.value} />
                        ) : row.key === "desc:text" ? (
                          <CopyTextPreview
                            text={row.value}
                            copyLabel="descCopyText"
                          />
                        ) : ["ocr:input_sha256", "desc:input_sha256"].includes(
                            row.key,
                          ) ? (
                          <CopyButton value={row.value}>
                            {({ copied, copy }) => (
                              <button
                                type="button"
                                className="copy-value copy-value--identifier"
                                onClick={copy}
                                title={row.value}
                                aria-label={t(
                                  copied ? "copied" : "ocrCopyHash",
                                )}
                              >
                                <bdi>{row.value}</bdi>
                                <span>{t(copied ? "copied" : "copy")}</span>
                              </button>
                            )}
                          </CopyButton>
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
              </Stack>
            </section>
          ))}
        </section>
      )}
    </div>
  );
}
