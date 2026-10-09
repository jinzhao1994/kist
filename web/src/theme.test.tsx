import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider, storageKey } from "./i18n";
import { Settings } from "./Settings";
import {
  ThemeProvider,
  appearanceKey,
  initializeAppearance,
  readAppearance,
} from "./theme";

let media: {
  matches: boolean;
  addEventListener: ReturnType<typeof vi.fn>;
  removeEventListener: ReturnType<typeof vi.fn>;
};
let change: () => void;
beforeEach(() => {
  localStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "de-DE",
      timezone: "UTC",
      direction: "rtl",
    }),
  );
  media = {
    matches: false,
    addEventListener: vi.fn((_event, callback) => {
      change = callback;
    }),
    removeEventListener: vi.fn(),
  };
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => media),
  );
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.clear();
});
function mount(close = vi.fn()) {
  return render(
    <ThemeProvider>
      <I18nProvider>
        <Settings open onOpenChange={close} />
      </I18nProvider>
    </ThemeProvider>,
  );
}
it("saves independent theme and mode, restores them on remount, and leaves cancelled drafts unchanged", async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  const first = mount(close);
  const originalDisplay = localStorage.getItem(storageKey);
  await user.selectOptions(screen.getByLabelText("Theme"), "slate");
  await user.selectOptions(screen.getByLabelText("Appearance"), "dark");
  expect(document.documentElement.dataset.mode).toBe("light");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(close).toHaveBeenCalledWith(false);
  expect(readAppearance()).toEqual({ theme: "archive", mode: "system" });
  first.unmount();
  const second = mount();
  await user.selectOptions(screen.getByLabelText("Theme"), "slate");
  await user.selectOptions(screen.getByLabelText("Appearance"), "dark");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(readAppearance()).toEqual({ theme: "slate", mode: "dark" });
  expect(localStorage.getItem(storageKey)).toBe(originalDisplay);
  expect(document.documentElement.dir).toBe("rtl");
  expect(document.documentElement.style.colorScheme).toBe("dark");
  second.unmount();
  mount();
  expect(screen.getByLabelText("Theme")).toHaveValue("slate");
  expect(screen.getByLabelText("Appearance")).toHaveValue("dark");
});
it("tracks system changes only in system mode and removes its listener on unmount", async () => {
  const user = userEvent.setup();
  const view = mount();
  act(() => {
    media.matches = true;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("dark");
  await user.selectOptions(screen.getByLabelText("Appearance"), "light");
  await user.click(screen.getByRole("button", { name: "Save" }));
  act(() => {
    media.matches = true;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("light");
  await user.selectOptions(screen.getByLabelText("Appearance"), "system");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(document.documentElement.dataset.mode).toBe("dark");
  act(() => {
    media.matches = false;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("light");
  view.unmount();
  expect(media.removeEventListener).toHaveBeenLastCalledWith("change", change);
});
it("applies persisted appearance before mounting and tolerates corrupt or inaccessible storage", async () => {
  localStorage.setItem(
    appearanceKey,
    JSON.stringify({ theme: "slate", mode: "dark" }),
  );
  initializeAppearance();
  expect(document.documentElement.dataset.theme).toBe("slate");
  expect(document.documentElement.dataset.mode).toBe("dark");
  for (const raw of ["{", "null", '{"theme":"__proto__","mode":"unknown"}']) {
    localStorage.setItem(appearanceKey, raw);
    expect(readAppearance()).toEqual({ theme: "archive", mode: "system" });
  }
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
    throw new Error("denied");
  });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("denied");
  });
  expect(() => initializeAppearance()).not.toThrow();
  mount();
  const user = userEvent.setup();
  await user.selectOptions(screen.getByLabelText("Appearance"), "dark");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(document.documentElement.dataset.mode).toBe("dark");
});
