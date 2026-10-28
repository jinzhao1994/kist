import { Group, Tooltip } from "@mantine/core";
import type { ReactNode } from "react";
import { FloatingTooltip } from "./FloatingTooltip";
import { HelpTooltip } from "./HelpTooltip";

export function MetadataLabel({
  metadataKey,
  label,
  help,
}: {
  metadataKey: string;
  label: string;
  help?: ReactNode;
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
        {help ? (
          <Group gap="xs">
            {text}
            <HelpTooltip label={label} muted inline>
              {help}
            </HelpTooltip>
          </Group>
        ) : (
          text
        )}
      </dt>
    </Tooltip>
  );
}
