import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, it } from "vitest";
import { CacheLimitInput } from "./CacheLimitInput";
import { hasSettingChange } from "./SettingsFields";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

it("displays localized unlimited values while keeping wire values and finite drafts", async () => {
  localStorage.setItem(storageKey, JSON.stringify({ language: "en" }));
  function Example() {
    const [value, setValue] = useState("inf");
    return (
      <>
        <CacheLimitInput
          id="cache"
          label="Cache limit"
          value={value}
          onChange={setValue}
          disabled={false}
        />
        <output aria-label="Wire value">{value}</output>
      </>
    );
  }
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <Example />
      </I18nProvider>
    </ThemeProvider>,
  );
  const user = userEvent.setup();
  const input = screen.getByRole("combobox", { name: "Cache limit" });
  expect(input).toHaveValue("No limit");
  expect(screen.getByLabelText("Wire value")).toHaveTextContent("inf");
  await user.clear(input);
  await user.type(input, "32 GB");
  expect(input).toHaveValue("32 GB");
  expect(screen.getByLabelText("Wire value")).toHaveTextContent("32 GB");
  await user.click(screen.getByRole("option", { name: "No limit" }));
  expect(input).toHaveValue("No limit");
  expect(screen.getByLabelText("Wire value")).toHaveTextContent("inf");
  localStorage.clear();
});

it("distinguishes explicit unlimited capacity from inherited limits", () => {
  const field = {
    kind: "bytes",
    value: "inf",
    explicit: true,
    unlimited: true,
  };
  expect(hasSettingChange(field, " INF ")).toBe(false);
  expect(hasSettingChange(field, null)).toBe(true);
  expect(hasSettingChange({ ...field, explicit: false }, "inf")).toBe(true);
  expect(hasSettingChange({ ...field, value: 32000000000 }, "32 GB")).toBe(
    false,
  );
});
