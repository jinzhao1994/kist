import { SegmentedControl } from "@mantine/core";
import { useEffect, useState } from "react";
import { useI18n } from "./i18n";

export type DisplayMode = "large" | "medium" | "small" | "list";
export const displayModes: DisplayMode[] = ["large", "medium", "small", "list"];
const eventName = "kist-display-mode";
export function useDisplayMode(kind: "boxes" | "files") {
  const key = `kist.display.${kind}.v1`;
  const read = (): DisplayMode => {
    try {
      const value = localStorage.getItem(key) as DisplayMode;
      if (displayModes.includes(value)) return value;
    } catch {
      /* Preferences are optional when storage is unavailable. */
    }
    return "medium";
  };
  const [mode, setMode] = useState(read);
  useEffect(() => {
    const storage = () => setMode(read());
    const change = (event: Event) => {
      const detail = (event as CustomEvent).detail;
      if (detail.key === key && displayModes.includes(detail.mode))
        setMode(detail.mode);
    };
    window.addEventListener("storage", storage);
    window.addEventListener(eventName, change);
    return () => {
      window.removeEventListener("storage", storage);
      window.removeEventListener(eventName, change);
    };
  }, [key]);
  return [
    mode,
    (next: DisplayMode) => {
      try {
        localStorage.setItem(key, next);
      } catch {
        /* Keep the current session usable. */
      }
      setMode(next);
      window.dispatchEvent(
        new CustomEvent(eventName, { detail: { key, mode: next } }),
      );
    },
  ] as const;
}
export function DisplayModeControl({
  mode,
  change,
}: {
  mode: DisplayMode;
  change: (mode: DisplayMode) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="display-mode">
      <SegmentedControl
        aria-label={t("displayMode")}
        size="xs"
        value={mode}
        onChange={(value) => change(value as DisplayMode)}
        data={displayModes.map((value) => ({
          value,
          label: t(value === "list" ? "listView" : `${value}Icons`),
        }))}
      />
    </div>
  );
}
