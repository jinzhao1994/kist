import { ActionIcon } from "@mantine/core";
import type { MouseEvent } from "react";

export function SidebarMenuButton({
  label,
  expanded,
  disabled,
  onClick,
}: {
  label: string;
  expanded: boolean;
  disabled?: boolean;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  return (
    <ActionIcon
      className="sidebar-row-menu"
      size="sm"
      variant="subtle"
      color="gray"
      aria-label={label}
      aria-haspopup="menu"
      aria-expanded={expanded}
      disabled={disabled}
      onClick={onClick}
    >
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <circle cx="5" cy="12" r="1.5" />
        <circle cx="12" cy="12" r="1.5" />
        <circle cx="19" cy="12" r="1.5" />
      </svg>
    </ActionIcon>
  );
}
