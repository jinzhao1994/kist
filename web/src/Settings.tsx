import { Radio, Group, ColorSwatch, DEFAULT_THEME } from "@mantine/core";
import { useId } from "react";
import { Select } from "./Select";
import { Dialog } from "./Dialog";
import { useI18n, type Preferences } from "./i18n";
import { colors, useTheme, type Appearance } from "./theme";
export function Settings({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { prefs, setPrefs, t } = useI18n();
  const { appearance, setAppearance } = useTheme();
  const id = useId();
  const field = <K extends keyof Preferences>(key: K, value: Preferences[K]) =>
    setPrefs({ ...prefs, [key]: value });
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t("settings")}>
      <div className="settings-form">
        <Radio.Group
          label={t("primaryColor")}
          value={appearance.primaryColor}
          onChange={(primaryColor) =>
            setAppearance({
              ...appearance,
              primaryColor: primaryColor as Appearance["primaryColor"],
            })
          }
        >
          <Group gap="sm" mt="xs" wrap="wrap">
            {colors.map((color) => (
              <Radio
                key={color}
                value={color}
                data-autofocus={color === appearance.primaryColor || undefined}
                color={color}
                label={
                  <Group gap={6}>
                    <ColorSwatch
                      color={DEFAULT_THEME.colors[color][6]}
                      size={16}
                    />
                    {t(color)}
                  </Group>
                }
              />
            ))}
          </Group>
        </Radio.Group>
        <Select
          id={`${id}-mode`}
          label={t("colorMode")}
          value={appearance.mode}
          onValueChange={(mode) =>
            setAppearance({ ...appearance, mode: mode as Appearance["mode"] })
          }
          options={[
            { value: "system", label: t("modeSystem"), special: true },
            { value: "light", label: t("modeLight") },
            { value: "dark", label: t("modeDark") },
          ]}
        />
        <Select
          id={`${id}-language`}
          label={t("language")}
          value={prefs.language}
          onValueChange={(language) =>
            field("language", language as Preferences["language"])
          }
          options={[
            { value: "en", label: "English" },
            { value: "zh", label: "中文" },
          ]}
        />
        <Select
          id={`${id}-locale`}
          label={t("formatLocale")}
          value={prefs.formatLocale}
          onValueChange={(locale) => field("formatLocale", locale)}
          options={localeOptions(prefs.language, prefs.formatLocale)}
          searchable
        />
        <Select
          id={`${id}-zone`}
          label={t("timezone")}
          value={prefs.timezone}
          onValueChange={(timezone) => field("timezone", timezone)}
          options={[
            ...new Set([
              "UTC",
              prefs.timezone,
              ...Intl.supportedValuesOf("timeZone"),
            ]),
          ]
            .sort()
            .map((value) => ({ value, label: value }))}
          searchable
        />
        <Select
          id={`${id}-direction`}
          label={t("direction")}
          value={prefs.direction}
          onValueChange={(direction) =>
            field("direction", direction as Preferences["direction"])
          }
          options={[
            { value: "auto", label: t("auto"), special: true },
            { value: "ltr", label: t("ltr") },
            { value: "rtl", label: t("rtl") },
          ]}
        />
      </div>
    </Dialog>
  );
}

// Common regional formats; keep existing browser preferences selectable as well.
const regionalLocales = [
  "zh-CN",
  "zh-TW",
  "zh-HK",
  "en-US",
  "en-GB",
  "en-CA",
  "en-AU",
  "en-IN",
  "en-SG",
  "ja-JP",
  "ko-KR",
  "fr-FR",
  "fr-CA",
  "de-DE",
  "de-CH",
  "es-ES",
  "es-MX",
  "es-AR",
  "pt-BR",
  "pt-PT",
  "it-IT",
  "nl-NL",
  "sv-SE",
  "nb-NO",
  "da-DK",
  "fi-FI",
  "pl-PL",
  "cs-CZ",
  "tr-TR",
  "ru-RU",
  "uk-UA",
  "ar-SA",
  "he-IL",
  "hi-IN",
  "th-TH",
  "vi-VN",
  "id-ID",
  "ms-MY",
];
function localeOptions(language: string, current: string) {
  const names = new Intl.DisplayNames([language], { type: "language" });
  return [...new Set([current, ...regionalLocales])].map((value) => ({
    value,
    label: `${names.of(new Intl.Locale(value).baseName)} · ${value}`,
  }));
}
