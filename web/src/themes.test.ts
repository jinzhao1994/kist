import { beforeEach, expect, it } from "vitest";
import { appearanceKey, readAppearance } from "./theme";

beforeEach(() => localStorage.clear());
it.each([
  ["archive", "gray"],
  ["forest", "green"],
  ["wheat", "yellow"],
  ["blossom", "pink"],
  ["slate", "blue"],
  ["sunset", "orange"],
])(
  "migrates legacy %s to %s without losing dark mode",
  (theme, primaryColor) => {
    localStorage.setItem(
      "kist.appearance.v1",
      JSON.stringify({ theme, mode: "dark" }),
    );
    expect(readAppearance()).toEqual({ primaryColor, mode: "dark" });
  },
);
it("prefers new preferences over a legacy skin and validates unknown colors", () => {
  localStorage.setItem(
    "kist.appearance.v1",
    JSON.stringify({ theme: "forest", mode: "dark" }),
  );
  localStorage.setItem(
    appearanceKey,
    JSON.stringify({ primaryColor: "orange", mode: "light" }),
  );
  expect(readAppearance()).toEqual({ primaryColor: "orange", mode: "light" });
  localStorage.setItem(
    appearanceKey,
    JSON.stringify({ primaryColor: "__proto__", mode: "unknown" }),
  );
  expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "system" });
});
