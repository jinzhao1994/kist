import { beforeEach, expect, it } from "vitest";
import { appearanceKey, readAppearance } from "./theme";

beforeEach(() => localStorage.clear());
it.each(["archive", "forest", "wheat", "blossom", "slate", "sunset"])(
  "keeps legacy %s mode but leaves primary color to the instance",
  (theme) => {
    localStorage.setItem(
      "kist.appearance.v1",
      JSON.stringify({ theme, mode: "dark" }),
    );
    expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "dark" });
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
  expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "light" });
  localStorage.setItem(
    appearanceKey,
    JSON.stringify({ primaryColor: "__proto__", mode: "unknown" }),
  );
  expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "system" });
});
