import type { CSSProperties } from "react";
import type { DisplayMode } from "./DisplayMode";

const layouts = {
  large: { minWidth: 300, gapRem: 1.2 },
  medium: { minWidth: 230, gapRem: 1.2 },
  small: { minWidth: 160, gapRem: 0.65 },
};

export function iconGridStyle(mode: DisplayMode): CSSProperties {
  const layout = layouts[mode === "list" ? "medium" : mode];
  return {
    "--icon-grid-min-width": `${layout.minWidth}px`,
    "--icon-grid-gap": `${layout.gapRem}rem`,
  } as CSSProperties;
}

export function iconGridColumns(
  width: number,
  mode: DisplayMode,
  rootFontSize = 16,
) {
  if (mode === "list") return 1;
  const { minWidth, gapRem } = layouts[mode];
  const gap = gapRem * rootFontSize;
  return Math.max(1, Math.floor((width + gap) / (minWidth + gap)));
}
