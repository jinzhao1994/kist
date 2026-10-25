import { ActionIcon, Menu, Tooltip } from "@mantine/core";
import type { ReactNode } from "react";
import { useI18n } from "./i18n";

export function MoreActionsMenu({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  return (
    <Menu position="bottom-end" withinPortal>
      <Menu.Target>
        <Tooltip
          label={t("moreActions")}
          events={{ hover: true, focus: true, touch: false }}
        >
          <ActionIcon
            size="lg"
            variant="subtle"
            color="gray"
            aria-label={t("moreActions")}
          >
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="currentColor"
              aria-hidden="true"
            >
              <circle cx="5" cy="12" r="1.7" />
              <circle cx="12" cy="12" r="1.7" />
              <circle cx="19" cy="12" r="1.7" />
            </svg>
          </ActionIcon>
        </Tooltip>
      </Menu.Target>
      <Menu.Dropdown>{children}</Menu.Dropdown>
    </Menu>
  );
}
