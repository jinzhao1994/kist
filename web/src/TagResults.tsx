import { Button } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { api, type Tag, type Box, type ArchiveFile } from "./api";
import { BoxGrid } from "./BoxGrid";
import { FileBrowser } from "./FileBrowser";
import { emptyFilters } from "./Filters";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

export function TagResults({
  tag,
  tags,
  paused,
  openBox,
  openFile,
}: {
  tag: string;
  tags: Tag[];
  paused: boolean;
  openBox: (box: Box) => void;
  openFile: (file: ArchiveFile) => void;
}) {
  const { t } = useI18n();
  const boxes = useQuery({
    queryKey: ["boxes", "tag", tag],
    queryFn: ({ signal }) => api.boxesByTag(tag, signal),
  });
  return (
    <>
      <section className="tag-results-boxes" aria-label={t("boxes")}>
        <h2>{t("boxes")}</h2>
        {boxes.isPending && <p role="status">{t("loading")}</p>}
        {boxes.isError && (
          <div role="alert">
            <p>{errorMessage(boxes.error, t)}</p>
            <Button onClick={() => boxes.refetch()}>{t("retry")}</Button>
          </div>
        )}
        {boxes.data &&
          (boxes.data.boxes?.length ? (
            <BoxGrid boxes={boxes.data.boxes} open={openBox} />
          ) : (
            <p>{t("noMatchingBoxes")}</p>
          ))}
      </section>
      <section>
        <h2>{t("files")}</h2>
        <FileBrowser
          filters={{ ...emptyFilters, tag_all: [tag] }}
          tags={tags}
          paused={paused}
          emptyHint={t("noTaggedFilesHint")}
          open={openFile}
        />
      </section>
    </>
  );
}
