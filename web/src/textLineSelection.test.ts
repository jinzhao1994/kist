import { expect, it } from "vitest";
import { lineHash, parseLineHash } from "./textLineSelection";

it("reads single lines and normalized forward or backward ranges", () => {
  expect(parseLineHash("#L3", 80)).toEqual({ from: 3, to: 3 });
  expect(parseLineHash("#L3-7", 80)).toEqual({ from: 3, to: 7 });
  expect(parseLineHash("#L7-L3", 80)).toEqual({ from: 3, to: 7 });
  expect(lineHash({ from: 3, to: 3 })).toBe("#L3");
  expect(lineHash({ from: 3, to: 7 })).toBe("#L3-7");
});
it("bounds links to loaded text and rejects invalid or unrelated fragments", () => {
  expect(parseLineHash("#L3-200", 10)).toEqual({ from: 3, to: 10 });
  for (const hash of [
    "#L20",
    "#L0",
    "#L-1",
    "#L1-0",
    "#L1.5",
    "#L99999999999999999999",
    "#chapter",
    "",
  ]) {
    expect(parseLineHash(hash, 10)).toBeNull();
  }
});
