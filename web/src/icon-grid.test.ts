import { expect, it } from "vitest";
import { iconGridColumns } from "./icon-grid";

it.each([
  ["large", 619.2, 4],
  ["medium", 479.2, 5],
  ["small", 330.4, 9],
] as const)(
  "accounts for gaps and keeps adding %s columns on wide screens",
  (mode, twoColumns, beyondOldLimit) => {
    expect(iconGridColumns(twoColumns - 1, mode)).toBe(1);
    expect(iconGridColumns(twoColumns + 0.01, mode)).toBe(2);
    expect(iconGridColumns(4000, mode)).toBeGreaterThanOrEqual(beyondOldLimit);
    expect(iconGridColumns(80, mode)).toBe(1);
  },
);

it("accounts for scaled spacing and keeps list view at one column", () => {
  expect(iconGridColumns(480, "medium", 20)).toBe(1);
  expect(iconGridColumns(4000, "list")).toBe(1);
});
