import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SecretInput } from "./SecretInput";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

beforeEach(() => {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  vi.stubGlobal("CSS", { ...globalThis.CSS, supports: vi.fn(() => true) });
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
function mount(disabled = false) {
  function Field() {
    const [value, setValue] = useState("");
    return (
      <SecretInput
        label="API Key"
        value={value}
        onChange={(e) => setValue(e.currentTarget.value)}
        disabled={disabled}
        placeholder="Configured; leave empty to keep"
      />
    );
  }
  return render(
    <ThemeProvider env="test">
      <I18nProvider>
        <Field />
      </I18nProvider>
    </ThemeProvider>,
  );
}
it("keeps the key out of native password fields while masking, pasting and revealing", async () => {
  const { container } = mount();
  const input = screen.getByLabelText("API Key", { selector: "input" });
  expect(input).toHaveAttribute("type", "text");
  expect(input).toHaveAttribute("autocomplete", "off");
  expect(input).toHaveAttribute("spellcheck", "false");
  expect(container.querySelector("[data-masked-secret]")).not.toBeNull();
  const user = userEvent.setup();
  await user.click(input);
  await user.paste("synthetic-api-key");
  expect(input).toHaveValue("synthetic-api-key");
  const reveal = screen.getByRole("button", { name: "Show secret" });
  expect(reveal).toHaveAttribute("aria-pressed", "false");
  await user.click(reveal);
  expect(screen.getByRole("button", { name: "Hide secret" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(container.querySelector("[data-masked-secret]")).toBeNull();
  expect(input).toHaveValue("synthetic-api-key");
  await user.click(screen.getByRole("button", { name: "Hide secret" }));
  expect(container.querySelector("[data-masked-secret]")).not.toBeNull();
  expect(container.querySelector('input[type="password"]')).toBeNull();
  await user.clear(input);
  expect(input).toHaveValue("");
  expect(input).toHaveAttribute(
    "placeholder",
    "Configured; leave empty to keep",
  );
});
it("falls back to native masking when masked text is unsupported", async () => {
  vi.stubGlobal("CSS", { ...globalThis.CSS, supports: vi.fn(() => false) });
  mount();
  const input = screen.getByLabelText("API Key", { selector: "input" });
  expect(input).toHaveAttribute("type", "password");
  await userEvent.click(screen.getByRole("button", { name: "Show secret" }));
  expect(input).toHaveAttribute("type", "text");
  await userEvent.click(screen.getByRole("button", { name: "Hide secret" }));
  expect(input).toHaveAttribute("type", "password");
});
it("preserves disabled input and visibility controls", async () => {
  mount(true);
  expect(
    screen.getByLabelText("API Key", { selector: "input" }),
  ).toBeDisabled();
  const button = screen.getByRole("button", { name: "Show secret" });
  expect(button).toBeDisabled();
  await userEvent.click(button);
  expect(button).toHaveAttribute("aria-pressed", "false");
});
