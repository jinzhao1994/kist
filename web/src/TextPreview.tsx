import { Button, NativeSelect } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, useState } from "react";
import { get, type ArchiveFile, type TextPreviewResult } from "./api";
import { useI18n } from "./i18n";

const TextReader = lazy(() => import("./TextReader"));

export function TextPreview({
  file,
  detail,
  active,
  paused,
}: {
  file: ArchiveFile;
  detail: boolean;
  active: boolean;
  paused: boolean;
}) {
  const { t } = useI18n();
  const [encoding, setEncoding] = useState("auto");
  const query = useQuery({
    queryKey: ["text-preview", file.id, file.blob_key, detail, encoding],
    enabled: active && !paused,
    queryFn: ({ signal }) =>
      get<TextPreviewResult>(
        `/files/${encodeURIComponent(file.id)}/text-preview?detail=${detail}&encoding=${encoding}`,
        signal,
      ),
  });
  const data = query.data;
  const choice = detail && (
    <NativeSelect
      aria-label={t("textEncoding")}
      value={encoding}
      onChange={(event) => setEncoding(event.currentTarget.value)}
      data={[
        { value: "auto", label: t("textEncodingAuto") },
        { value: "gb18030", label: "GB18030" },
      ]}
    />
  );
  if (detail && data?.supported)
    return (
      <Suspense
        fallback={<div className="media-placeholder">{t("loading")}</div>}
      >
        <TextReader
          key={`${file.id}:${file.blob_key}:${encoding}`}
          text={data.text}
          name={file.name}
          encoding={data.encoding}
          truncated={data.truncated}
          encodingControl={choice}
        />
      </Suspense>
    );
  if (!detail && data?.supported)
    return (
      <div className="text-thumbnail" aria-label={t("textPreview")}>
        <pre aria-hidden="true">{data.text || t("emptyText")}</pre>
      </div>
    );
  return (
    <div className="media-placeholder">
      <span aria-hidden="true">▤</span>
      {detail && (
        <>
          <p role="status">
            {query.isError
              ? t("textLoadFailed")
              : !data
                ? t("loading")
                : data.reason === "encoding"
                  ? t("textEncodingUnknown")
                  : t("previewUnavailable")}
          </p>
          {(!data || data.reason !== "binary") && choice}
          {query.isError && (
            <Button onClick={() => void query.refetch()}>{t("retry")}</Button>
          )}
        </>
      )}
    </div>
  );
}
