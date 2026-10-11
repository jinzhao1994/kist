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
it("applies selections immediately, keeps them when closed, and restores them on remount", async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  const first = mount(close);
  const originalDisplay = localStorage.getItem(storageKey);
  expect(
    screen.queryByRole("button", { name: "Save" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Cancel" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("radio", { name: "Green" }));
  expect(readAppearance()).toEqual({ primaryColor: "green", mode: "system" });
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(await screen.findByRole("option", { name: "Dark" }));
  expect(readAppearance()).toEqual({ primaryColor: "green", mode: "dark" });
  expect(document.documentElement.style.colorScheme).toBe("dark");
  expect(localStorage.getItem(storageKey)).toBe(originalDisplay);
  await user.click(screen.getByRole("button", { name: "Close" }));
  expect(close).toHaveBeenCalledWith(false);
  first.unmount();
  mount();
  expect(screen.getByRole("radio", { name: "Green" })).toBeChecked();
  expect(screen.getByRole("combobox", { name: "Appearance" })).toHaveValue(
    "Dark",
  );
});
it("searches regional formats and time zones and persists only selected options", async () => {
  const user = userEvent.setup();
  const first = mount();
  const locale = screen.getByRole("combobox", { name: "Regional format" });
  await user.click(locale);
  await user.clear(locale);
  await user.type(locale, "ja-JP", { skipClick: true });
  expect(JSON.parse(localStorage.getItem(storageKey)!).formatLocale).toBe(
    "de-DE",
  );
  await user.click(screen.getByRole("option", { name: /ja-JP/ }));
  expect(JSON.parse(localStorage.getItem(storageKey)!).formatLocale).toBe(
    "ja-JP",
  );
  const zone = screen.getByRole("combobox", { name: "Time zone" });
  await user.click(zone);
  await user.clear(zone);
  await user.type(zone, "Asia/Tokyo", { skipClick: true });
  await user.click(screen.getByRole("option", { name: "Asia/Tokyo" }));
  expect(JSON.parse(localStorage.getItem(storageKey)!).timezone).toBe(
    "Asia/Tokyo",
  );
  await user.click(screen.getByRole("combobox", { name: "Reading direction" }));
  await user.click(screen.getByRole("option", { name: "Left to right" }));
  expect(document.documentElement.dir).toBe("ltr");
  first.unmount();
  mount();
  expect(screen.getByRole("combobox", { name: "Regional format" })).toHaveValue(
    "Japanese (Japan) · ja-JP",
  );
  expect(screen.getByRole("combobox", { name: "Time zone" })).toHaveValue(
    "Asia/Tokyo",
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
  act(() => {
    media.matches = true;
    change();
  });
  expect(document.documentElement.dataset.mode).toBe("light");
  await user.click(screen.getByRole("combobox", { name: "Appearance" }));
  await user.click(screen.getByRole("option", { name: "Follow system" }));
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
  expect(document.documentElement.dataset.mode).toBe("dark");
});
