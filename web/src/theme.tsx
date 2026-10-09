import {
  createContext,
  useContext,
  useLayoutEffect,
  useState,
  type ReactNode,
} from "react";

export const themes = {
  archive: "themeArchive",
  forest: "themeForest",
  wheat: "themeWheat",
  blossom: "themeBlossom",
  slate: "themeSlate",
  sunset: "themeSunset",
} as const;
export type Appearance = {
  theme: keyof typeof themes;
  mode: "system" | "light" | "dark";
};
export const appearanceKey = "kist.appearance.v1";
export function readAppearance(): Appearance {
  try {
    const value = JSON.parse(localStorage.getItem(appearanceKey) || "null");
    return {
      theme:
        typeof value?.theme === "string" && Object.hasOwn(themes, value.theme)
          ? value.theme
          : "archive",
      mode: ["system", "light", "dark"].includes(value?.mode)
        ? value.mode
        : "system",
    };
  } catch {
    return { theme: "archive", mode: "system" };
  }
}
export function applyAppearance(value: Appearance, systemDark: boolean) {
  const mode =
    value.mode === "system" ? (systemDark ? "dark" : "light") : value.mode;
  document.documentElement.dataset.theme = value.theme;
  document.documentElement.dataset.mode = mode;
  document.documentElement.style.colorScheme = mode;
}
export function initializeAppearance() {
  applyAppearance(
    readAppearance(),
    window.matchMedia("(prefers-color-scheme: dark)").matches,
  );
}
const ThemeContext = createContext<{
  appearance: Appearance;
  setAppearance: (value: Appearance) => void;
} | null>(null);
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [appearance, setAppearance] = useState(readAppearance);
  useLayoutEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => applyAppearance(appearance, media.matches);
    apply();
    media.addEventListener("change", apply);
    try {
      localStorage.setItem(appearanceKey, JSON.stringify(appearance));
    } catch {
      // The active session still retains the preference when storage is unavailable.
    }
    return () => media.removeEventListener("change", apply);
  }, [appearance]);
  return (
    <ThemeContext value={{ appearance, setAppearance }}>
      {children}
    </ThemeContext>
  );
}
export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("ThemeProvider is missing");
  return value;
}
