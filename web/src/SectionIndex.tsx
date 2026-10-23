import { Anchor, Text } from "@mantine/core";
import type { ReactNode } from "react";
import { useI18n } from "./i18n";

export function SectionIndex({
  items,
  children,
}: {
  items: { id: string; label: string }[];
  children: ReactNode;
}) {
  const { t } = useI18n();
  return (
    <div className="section-layout">
      <nav className="section-index" aria-label={t("pageContents")}>
        <Text size="xs" c="dimmed">
          {t("pageContents")}
        </Text>
        {items.map(({ id, label }) => (
          <Anchor key={id} href={`#${id}`} size="sm">
            {label}
          </Anchor>
        ))}
      </nav>
      <div className="section-body">{children}</div>
    </div>
  );
}
