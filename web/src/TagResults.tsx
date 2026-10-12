import { type Tag, type Box, type ArchiveFile } from "./api";
import { BoxBrowser } from "./BoxBrowser";
import { FileBrowser } from "./FileBrowser";
import { emptyBoxFilters, emptyFilters } from "./Filters";
import { useI18n } from "./i18n";

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
  return (
    <>
      <section className="tag-results-boxes" aria-label={t("boxes")}>
        <h2>{t("boxes")}</h2>
        <BoxBrowser
          compact
          filters={{ ...emptyBoxFilters, tag_all: [tag] }}
          open={openBox}
          paused={paused}
        />
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
