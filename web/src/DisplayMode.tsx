import { ActionIcon, Menu, SegmentedControl } from "@mantine/core";
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
  dropdown = false,
}: {
  mode: DisplayMode;
  change: (mode: DisplayMode) => void;
  dropdown?: boolean;
}) {
  const { t } = useI18n();
  if (dropdown)
    return (
      <Menu position="bottom-end">
        <Menu.Target>
          <ActionIcon
            size="lg"
            variant="default"
            aria-label={t("displayMode")}
            title={t("displayMode")}
          >
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              aria-hidden="true"
            >
              {mode === "list" ? (
                <path d="M4 6h16M4 12h16M4 18h16" />
              ) : (
                <>
                  <rect x="4" y="4" width="16" height="16" rx="1" />
                  <path d="M12 4v16M4 12h16" />
                </>
              )}
            </svg>
          </ActionIcon>
        </Menu.Target>
        <Menu.Dropdown>
          {displayModes.map((value) => (
            <Menu.Item
              key={value}

              data-checked={mode === value}
              rightSection={mode === value ? "✓" : null}
              onClick={() => change(value)}
            >
              {t(value === "list" ? "listView" : `${value}Icons`)}
            </Menu.Item>
          ))}
        </Menu.Dropdown>
      </Menu>
    );
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
