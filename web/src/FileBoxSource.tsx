import { FloatingTooltip } from "./FloatingTooltip";
import type { Box } from "./api";

// Keep box names and partial-precision dates identical in cards and task lists.
export function FileBoxSource({
  box,
}: {
  box?: Pick<Box, "name" | "start_at">;
}) {
  const date = box?.start_at?.replace("T", " ") || "";
  const source = box ? [box.name, date].filter(Boolean).join(" · ") : "";
  return (
    <FloatingTooltip
      label={source}
      disabled={!source}
      multiline
      maw={360}
      position="bottom-start"
      offset={16}
      role="tooltip"
      classNames={{ tooltip: "metadata-tooltip" }}
    >
      <span className="file-card-source">
        <bdi className="file-card-source-name">{box?.name}</bdi>
        {date && (
          <time className="file-card-source-date" dateTime={box?.start_at}>
            {" · "}
            {date}
          </time>
        )}
      </span>
    </FloatingTooltip>
  );
}
