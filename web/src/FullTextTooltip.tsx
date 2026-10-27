import { Tooltip } from "@mantine/core";
import type { ReactElement } from "react";
import { FloatingTooltip } from "./FloatingTooltip";

export function FullTextTooltip({
  text,
  children,
}: {
  text: string;
  children: ReactElement;
}) {
  return (
    <FloatingTooltip
      label={text}
      multiline
      position="bottom-start"
      offset={16}
      role="tooltip"
      classNames={{ tooltip: "metadata-tooltip metadata-tooltip--text" }}
    >
      <Tooltip
        label={text}
        multiline
        interactive
        position="bottom-start"
        offset={8}
        classNames={{ tooltip: "metadata-tooltip metadata-tooltip--text" }}
        openDelay={0}
        closeDelay={100}
        transitionProps={{ duration: 0 }}
        events={{ hover: false, focus: true, touch: true }}
      >
        {children}
      </Tooltip>
    </FloatingTooltip>
  );
}
