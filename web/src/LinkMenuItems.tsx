import { Menu } from "@mantine/core";
import { useState } from "react";
import { useI18n } from "./i18n";

export function LinkMenuItems({
  href,
  copy = true,
}: {
  href: string;
  copy?: boolean;
}) {
  const { t } = useI18n();
  const [status, setStatus] = useState<"copied" | "copyLinkFailed" | null>(
    null,
  );
  return (
    <>
      <Menu.Item
        component="a"
        href={href}
        target="_blank"
        rel="noopener noreferrer"
      >
        {t("openNewTab")}
      </Menu.Item>
      {copy && (
        <Menu.Item
          closeMenuOnClick={false}
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(
                new URL(href, window.location.origin).href,
              );
              setStatus("copied");
            } catch {
              setStatus("copyLinkFailed");
            }
          }}
        >
          {t(status || "copyLink")}
        </Menu.Item>
      )}
    </>
  );
}
