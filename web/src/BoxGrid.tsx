import { UnstyledButton } from "@mantine/core";
import type { Box } from "./api";
import { formatLocalDate, useI18n } from "./i18n";

export function BoxGrid({
  boxes,
  open,
}: {
  boxes: Box[];
  open: (box: Box) => void;
}) {
  const { prefs } = useI18n();
  return (
    <div className="box-grid">
      {boxes.map((box) => (
        <UnstyledButton
          className="box-card"
          key={box.id}
          onClick={() => open(box)}
        >
          <span className="box-icon" aria-hidden="true">
            ▤
          </span>
          <h2>
            <bdi>{box.name}</bdi>
          </h2>
          <p>
            <bdi>{box.description}</bdi>
          </p>
          <time>
            {box.start_at ? formatLocalDate(box.start_at, prefs) : ""}
            {box.end_at ? ` — ${formatLocalDate(box.end_at, prefs)}` : ""}
          </time>
        </UnstyledButton>
      ))}
    </div>
  );
}
