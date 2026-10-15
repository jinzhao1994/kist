import {
  MantineProvider,
  DirectionProvider,
  createTheme,
  Button,
  Combobox,
  Modal,
} from "@mantine/core";
import {
  createContext,
  useContext,
  useLayoutEffect,
  useState,
  type ReactNode,
} from "react";

export const colors = [
  "gray",
  "green",
  "yellow",
  "pink",
  "blue",
  "orange",
] as const;
export type Appearance = {
  primaryColor: (typeof colors)[number];
  mode: "system" | "light" | "dark";
};
export const appearanceKey = "kist.appearance.v2";
const legacyColors: Record<string, Appearance["primaryColor"]> = {
  archive: "gray",
  forest: "green",
  wheat: "yellow",
  blossom: "pink",
  slate: "blue",
  sunset: "orange",
};
export function readAppearance(): Appearance {
  try {
    const raw = localStorage.getItem(appearanceKey);
    const value = JSON.parse(
      raw ?? localStorage.getItem("kist.appearance.v1") ?? "null",
    );
    const primaryColor =
      raw === null ? legacyColors[value?.theme] : value?.primaryColor;
    return {
      primaryColor: colors.includes(primaryColor) ? primaryColor : "blue",
      mode: ["system", "light", "dark"].includes(value?.mode)
        ? value.mode
        : "system",
    };
  } catch {
    return { primaryColor: "blue", mode: "system" };
  }
}
export function applyAppearance(value: Appearance, systemDark: boolean) {
  const mode =
    value.mode === "system" ? (systemDark ? "dark" : "light") : value.mode;
  delete document.documentElement.dataset.theme;
  document.documentElement.dataset.mode = mode;
  document.documentElement.dataset.mantineColorScheme = mode;
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
export function ThemeProvider({
  children,
  env = "default",
}: {
  children: ReactNode;
  env?: "default" | "test";
}) {
  const [appearance, setAppearance] = useState(readAppearance);
  const [systemDark, setSystemDark] = useState(
    () => window.matchMedia("(prefers-color-scheme: dark)").matches,
  );
  useLayoutEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useLayoutEffect(() => {
    applyAppearance(appearance, systemDark);
    try {
      localStorage.setItem(appearanceKey, JSON.stringify(appearance));
    } catch {
      /* The active session retains preferences when storage is unavailable. */
    }
  }, [appearance, systemDark]);
  const mode =
    appearance.mode === "system"
      ? systemDark
        ? "dark"
        : "light"
      : appearance.mode;
  const theme = createTheme({
    primaryColor: appearance.primaryColor,
    autoContrast: true,
    components: {
      Button: Button.extend({ defaultProps: { variant: "default" } }),
      Modal: Modal.extend({
        defaultProps: {
          shadow:
            mode === "dark"
              ? "0 4px 12px #0005, 0 20px 48px #0007"
              : "0 4px 12px #00000014, 0 20px 48px #00000033",
        },
      }),
      Combobox: Combobox.extend({
        defaultProps: {
          shadow:
            mode === "dark"
              ? "0 2px 6px #0004, 0 10px 24px #0006"
              : "0 2px 6px #0000001a, 0 10px 24px #00000024",
        },
      }),
    },
  });
  return (
    <ThemeContext value={{ appearance, setAppearance }}>
      <DirectionProvider>
        <MantineProvider theme={theme} forceColorScheme={mode} env={env}>
          {children}
        </MantineProvider>
      </DirectionProvider>
    </ThemeContext>
  );
}
export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("ThemeProvider is missing");
  return value;
}
