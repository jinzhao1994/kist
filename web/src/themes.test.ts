import { readFileSync } from "node:fs";
import { expect, it } from "vitest";
import { themes } from "./theme";

const css = readFileSync("src/themes.css", "utf8");
const components = readFileSync("src/style.css", "utf8");
const rules = Array.from(
  css.matchAll(/(:root[^{}]*)\{([^}]+)\}/g),
  ([, selector, body]) => ({
    selector: selector.trim(),
    values: Object.fromEntries(
      Array.from(
        body.matchAll(/--([\w-]+):\s*(#[\da-f]+);/g),
        ([, key, value]) => [key, value],
      ),
    ),
  }),
);
function luminance(hex: string) {
  const rgb = [1, 3, 5].map(
    (offset) => parseInt(hex.slice(offset, offset + 2), 16) / 255,
  );
  return rgb
    .map((v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
    .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
}
for (const theme of Object.keys(themes)) {
  for (const mode of ["light", "dark"]) {
    it(`${theme}/${mode} supplies component tokens and readable text contrast`, () => {
      if (theme !== "archive") {
        expect(
          rules.some((r) => r.selector === `:root[data-theme="${theme}"]`),
        ).toBe(true);
        expect(
          rules.some(
            (r) =>
              r.selector === `:root[data-theme="${theme}"][data-mode="dark"]`,
          ),
        ).toBe(true);
      }
      const palette = Object.assign(
        {},
        ...rules
          .filter(
            ({ selector }) =>
              (!selector.includes("data-theme") ||
                selector.includes(`data-theme="${theme}"`)) &&
              (!selector.includes("data-mode") ||
                selector.includes(`data-mode="${mode}"`)),
          )
          .map((r) => r.values),
      ) as Record<string, string>;
      for (const [, token] of components.matchAll(/var\(--([\w-]+)\)/g))
        expect(palette[token], token).toBeDefined();
      for (const [fg, bg] of [
        ["text", "canvas"],
        ["text", "surface"],
        ["muted", "surface"],
        ["muted", "sidebar"],
        ["muted", "media"],
        ["on-accent", "accent"],
        ["danger", "surface"],
        ["selected-text", "selected"],
        ["text", "warning"],
      ]) {
        const [a, b] = [luminance(palette[fg]), luminance(palette[bg])].sort(
          (x, y) => x - y,
        );
        expect(
          (b + 0.05) / (a + 0.05),
          `${fg} on ${bg}`,
        ).toBeGreaterThanOrEqual(4.5);
      }
    });
  }
}
