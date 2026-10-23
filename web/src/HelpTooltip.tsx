import { ActionIcon, Tooltip } from "@mantine/core";
import type { ReactNode } from "react";

export function HelpTooltip({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <Tooltip
      label={children}
      multiline
      maw={360}
      position="bottom-start"
      offset={8}
      classNames={{ tooltip: "metadata-tooltip" }}
      openDelay={0}
      transitionProps={{ duration: 0 }}
      events={{ hover: false, focus: true, touch: true }}
    >
      <ActionIcon
        type="button"
        variant="subtle"
        color="gray"
        size="sm"
        aria-label={label}
      >
        <Tooltip.Floating
          label={children}
          position="bottom-start"
          offset={16}
          role="tooltip"
          multiline
          maw={360}
          classNames={{ tooltip: "metadata-tooltip" }}
        >
          <span style={{ display: "inline-flex" }}>
            <svg
              width="16"
              height="16"
              viewBox="0 0 20 20"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.4"
              aria-hidden="true"
            >
              <circle cx="10" cy="10" r="7.5" />
              <path d="M10 9v5" />
              <circle cx="10" cy="6" r=".7" fill="currentColor" stroke="none" />
            </svg>
          </span>
        </Tooltip.Floating>
      </ActionIcon>
    </Tooltip>
  );
}
