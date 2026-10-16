import { describe, expect, it, vi } from "vitest";
import {
  parse,
  TYPE,
  type MessageFormatElement,
} from "@formatjs/icu-messageformat-parser";
import {
  en,
  zh,
  translator,
  formatLocalDate,
  formatInstant,
  formatNumber,
  initialPreferences,
  initializeLanguage,
  storageKey,
  type Preferences,
} from "./i18n";
const prefs: Preferences = {
  language: "en",
  formatLocale: "en-US",
  timezone: "Pacific/Honolulu",
  direction: "auto",
};
function variables(nodes: MessageFormatElement[]): string[] {
  const names = new Set<string>();
  for (const node of nodes) {
    if ("value" in node && node.type !== TYPE.literal && node.type !== TYPE.tag)
      names.add(node.value);
    if (node.type === TYPE.plural || node.type === TYPE.select) {
      expect(node.options.other).toBeDefined();
      for (const option of Object.values(node.options))
        for (const name of variables(option.value)) names.add(name);
    }
  }
  return [...names].sort();
}
describe("message contract", () => {
  it("keeps keys, placeholders and plural fallbacks consistent in every catalog", () => {
    expect(Object.keys(zh).sort()).toEqual(Object.keys(en).sort());
    for (const key of Object.keys(en) as (keyof typeof en)[])
      expect(variables(parse(zh[key])), key).toEqual(variables(parse(en[key])));
  });
  it("formats plural messages without concatenation", () => {
    expect(translator(prefs)("fileCount", { count: 1 })).toBe("1 file");
    expect(translator(prefs)("fileCount", { count: 2 })).toBe("2 files");
    expect(
      translator({ ...prefs, language: "zh" })("fileCount", { count: 2 }),
    ).toBe("2 个文件");
  });
  it("preserves LocalDate precision across zones", () => {
    expect(formatLocalDate("2020", prefs)).toBe("2020");
    expect(formatLocalDate("2020-02", prefs)).toBe("February 2020");
    expect(formatLocalDate("2020-02-29", prefs)).toBe("February 29, 2020");
    expect(
      formatLocalDate("2020-02-29", { ...prefs, timezone: "Asia/Shanghai" }),
    ).toBe(formatLocalDate("2020-02-29", prefs));
    expect(formatInstant("2020-03-01T00:00:00Z", prefs)).toContain("Feb 29");
  });
  it("keeps regional formatting independent from message language", () => {
    expect(
      translator({ ...prefs, formatLocale: "de-DE" })("fileCount", {
        count: 1234,
      }),
    ).toBe("1.234 files");
    expect(formatNumber(1234.5, { ...prefs, formatLocale: "de-DE" })).toBe(
      "1.234,5",
    );
    expect(translator({ ...prefs, formatLocale: "de-DE" })("boxes")).toBe(
      "Boxes",
    );
  });
  it("rejects malformed persisted preferences", () => {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify({
        language: "en",
        direction: "rtl",
        formatLocale: "not_a_locale",
        timezone: "missing",
      }),
    );
    expect(initialPreferences().formatLocale).not.toBe("not_a_locale");
    window.localStorage.removeItem(storageKey);
  });
});

describe("document language initialization", () => {
  it("applies saved message language and direction before React mounts", () => {
    localStorage.setItem(
      storageKey,
      JSON.stringify({ ...prefs, language: "zh", direction: "rtl" }),
    );
    initializeLanguage();
    expect(document.documentElement.lang).toBe("zh");
    expect(document.documentElement.dir).toBe("rtl");
    localStorage.removeItem(storageKey);
  });
  it("uses browser language when saved preferences cannot be read", () => {
    const languages = vi
      .spyOn(navigator, "languages", "get")
      .mockReturnValue(["zh-CN"]);
    const storage = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(() => {
        throw new Error("Storage is unavailable");
      });
    try {
      initializeLanguage();
      expect(document.documentElement.lang).toBe("zh");
      expect(document.documentElement.dir).toBe("ltr");
      languages.mockReturnValue(["en-US"]);
      initializeLanguage();
      expect(document.documentElement.lang).toBe("en");
    } finally {
      languages.mockRestore();
      storage.mockRestore();
    }
  });
});
