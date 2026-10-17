import { useI18n } from "./i18n";

export function TagLabels({
  tags,
}: {
  tags?: { id: string; name: string }[] | null;
}) {
  const { t } = useI18n();
  if (!tags?.length) return null;
  return (
    <span className="tag-chips entry-tags" role="list" aria-label={t("tags")}>
      {tags.map((tag) => (
        <span
          className="tag-chip"
          role="listitem"
          key={tag.id}
          title={tag.name}
        >
          <bdi>{tag.name}</bdi>
        </span>
      ))}
    </span>
  );
}
