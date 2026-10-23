import { Anchor } from "@mantine/core";
import { useI18n } from "./i18n";

export function SectionIndex({
  items,
}: {
  items: { id: string; label: string }[];
}) {
  const { t } = useI18n();
  return (
    <nav className="section-index" aria-label={t("pageContents")}>
      {items.map(({ id, label }) => (
        <Anchor key={id} href={`#${id}`} size="sm">
          {label}
        </Anchor>
      ))}
    </nav>
  );
}
