import { UnstyledButton } from "@mantine/core";
import type { MouseEvent } from "react";

export function TreeToggle({
  expanded,
  label,
  controls,
  tabIndex,
  onClick,
}: {
  expanded: boolean;
  label: string;
  controls?: string;
  tabIndex?: number;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  return (
    <UnstyledButton
      className="tree-toggle"
      tabIndex={tabIndex}
      aria-label={label}
      aria-expanded={expanded}
      aria-controls={controls}
      onClick={onClick}
    >
      <svg
        width="12"
        height="12"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
        className="tree-chevron"
        data-expanded={expanded}
      >
        <path
          d="m6 3 5 5-5 5"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </UnstyledButton>
  );
}
