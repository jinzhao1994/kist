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
let listeners: Set<(event: { matches: boolean }) => void>;
const change = () => {
  for (const listener of listeners) listener({ matches: media.matches });
};
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
  listeners = new Set();
  media = {
    matches: false,
    addEventListener: vi.fn((_event, callback) => {
      listeners.add(callback);
    }),
    removeEventListener: vi.fn((_event, callback) => {
      listeners.delete(callback);
    }),
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
    <ThemeProvider env="test">
      <I18nProvider>
        <Settings open onOpenChange={close} />
      </I18nProvider>
    </ThemeProvider>,
  );
}
it("saves independent primary color and mode, restores them on remount, and leaves cancelled drafts unchanged", async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  const first = mount(close);
  const originalDisplay = localStorage.getItem(storageKey);
  await user.click(screen.getByRole("radio", { name: "Blue" }));
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(await screen.findByRole("option", { name: "Dark" }));
  expect(document.documentElement.dataset.mode).toBe("light");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(close).toHaveBeenCalledWith(false);
  expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "system" });
  first.unmount();
  const second = mount();
  await user.click(screen.getByRole("radio", { name: "Green" }));
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(await screen.findByRole("option", { name: "Dark" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(readAppearance()).toEqual({ primaryColor: "green", mode: "dark" });
  expect(localStorage.getItem(storageKey)).toBe(originalDisplay);
  expect(document.documentElement.dir).toBe("rtl");
  expect(document.documentElement.style.colorScheme).toBe("dark");
  second.unmount();
  mount();
  expect(screen.getByRole("radio", { name: "Green" })).toBeChecked();
  expect(screen.getByRole("combobox", { name: "Appearance" })).toHaveValue(
    "Dark",
  );
});
it("tracks system changes only in system mode and removes its listener on unmount", async () => {
  const user = userEvent.setup();
  const view = mount();
  act(() => {
    media.matches = true;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("dark");
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(await screen.findByRole("option", { name: "Light" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  act(() => {
    media.matches = true;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("light");
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(screen.getByRole("option", { name: "Follow system" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(document.documentElement.dataset.mode).toBe("dark");
  act(() => {
    media.matches = false;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("light");
  view.unmount();
  expect(listeners.size).toBe(0);
});
it("applies persisted appearance before mounting and tolerates corrupt or inaccessible storage", async () => {
  localStorage.setItem(
    appearanceKey,
    JSON.stringify({ primaryColor: "blue", mode: "dark" }),
  );
  initializeAppearance();
  expect(document.documentElement.dataset.theme).toBeUndefined();
  expect(document.documentElement.dataset.mode).toBe("dark");
  for (const raw of ["{", "null", '{"theme":"__proto__","mode":"unknown"}']) {
    localStorage.setItem(appearanceKey, raw);
    expect(readAppearance()).toEqual({ primaryColor: "blue", mode: "system" });
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
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(await screen.findByRole("option", { name: "Dark" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(document.documentElement.dataset.mode).toBe("dark");
});
