import { Group, Tooltip } from "@mantine/core";
import type { ReactNode } from "react";
import { FloatingTooltip } from "./FloatingTooltip";

export function MetadataLabel({
  metadataKey,
  label,
  children,
}: {
  metadataKey: string;
  label: string;
  children?: ReactNode;
}) {
  const text = (
    <FloatingTooltip
      label={metadataKey}
      position="bottom-start"
      offset={16}
      role="tooltip"
      classNames={{ tooltip: "metadata-tooltip" }}
    >
      <bdi>{label}</bdi>
    </FloatingTooltip>
  );
  return (
    <Tooltip
      label={metadataKey}
      position="bottom-start"
      offset={8}
      classNames={{ tooltip: "metadata-tooltip" }}
      openDelay={0}
      transitionProps={{ duration: 0 }}
      events={{ hover: false, focus: true, touch: false }}
    >
      <dt tabIndex={0}>
        {children ? (
          <Group gap="xs">
            {text}
            {children}
          </Group>
        ) : (
          text
        )}
      </dt>
    </Tooltip>
  );
}
