import { expect, it } from "vitest";
import { APIError } from "./api";
import { translator, type Preferences } from "./i18n";
import { errorMessage } from "./mutations";

it("preserves unknown API diagnostics while localizing the surrounding message", () => {
  const error = new APIError(500, {
    code: "future_storage_error",
    class: "internal",
    message: "Storage is temporarily unavailable",
  });
  for (const language of ["en", "zh"] as const) {
    const prefs: Preferences = {
      language,
      formatLocale: "de-DE",
      timezone: "UTC",
      direction: "rtl",
    };
    const t = translator(prefs);
    const message = errorMessage(error, t);
    expect(message).toContain(error.body.code);
    expect(message).toContain(error.body.message);
    expect(message).toContain(
      language === "zh" ? "操作未完成" : "Something went wrong",
    );
    expect(errorMessage(new TypeError("Failed to fetch"), t)).toBe(t("error"));
    expect(
      errorMessage(new APIError(409, { ...error.body, code: "tag_cycle" }), t),
    ).toBe(t("tagCycle"));
  }
});
