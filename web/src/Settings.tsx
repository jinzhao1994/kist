import {
  TextInput,
  Button,
  Radio,
  Group,
  ColorSwatch,
  DEFAULT_THEME,
} from "@mantine/core";
import { useId, useState } from "react";
import { Select } from "./Select";
import { Dialog } from "./Dialog";
import { useI18n, validFormats, type Preferences } from "./i18n";
import { colors, useTheme, type Appearance } from "./theme";
export function Settings({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { prefs, setPrefs, t } = useI18n();
  const [draft, setDraft] = useState(prefs);
  const { appearance, setAppearance } = useTheme();
  const [look, setLook] = useState(appearance);
  const [error, setError] = useState(false);
  const id = useId();
  const field = <K extends keyof Preferences>(key: K, value: Preferences[K]) =>
    setDraft({ ...draft, [key]: value });
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t("settings")}>
      <form
        className="settings-form"
        onSubmit={(event) => {
          event.preventDefault();
          if (!validFormats(draft.formatLocale, draft.timezone)) {
            setError(true);
            return;
          }
          setPrefs(draft);
          setAppearance(look);
          onOpenChange(false);
        }}
      >
        <Radio.Group
          label={t("primaryColor")}
          value={look.primaryColor}
          onChange={(primaryColor) =>
            setLook({
              ...look,
              primaryColor: primaryColor as Appearance["primaryColor"],
            })
          }
        >
          <Group gap="sm" mt="xs" wrap="wrap">
            {colors.map((color) => (
              <Radio
                key={color}
                value={color}
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
          value={look.mode}
          onValueChange={(mode) =>
            setLook({ ...look, mode: mode as Appearance["mode"] })
          }
          options={[
            { value: "system", label: t("modeSystem") },
            { value: "light", label: t("modeLight") },
            { value: "dark", label: t("modeDark") },
          ]}
        />
        <Select
          id={`${id}-language`}
          label={t("language")}
          value={draft.language}
          onValueChange={(language) =>
            field("language", language as Preferences["language"])
          }
          options={[
            { value: "en", label: "English" },
            { value: "zh", label: "中文" },
          ]}
        />
        <label htmlFor={`${id}-locale`}>{t("formatLocale")}</label>
        <TextInput
          id={`${id}-locale`}
          value={draft.formatLocale}
          onChange={(e) => field("formatLocale", e.target.value)}
          placeholder="zh-CN / en-US / de-DE"
          required
        />
        <label htmlFor={`${id}-zone`}>{t("timezone")}</label>
        <TextInput
          id={`${id}-zone`}
          value={draft.timezone}
          onChange={(e) => field("timezone", e.target.value)}
          placeholder="Asia/Shanghai"
          required
        />
        <Select
          id={`${id}-direction`}
          label={t("direction")}
          value={draft.direction}
          onValueChange={(direction) =>
            field("direction", direction as Preferences["direction"])
          }
          options={[
            { value: "auto", label: t("auto") },
            { value: "ltr", label: t("ltr") },
            { value: "rtl", label: t("rtl") },
          ]}
        />
        {error && <p role="alert">{t("invalidSettings")}</p>}
        <div className="actions">
          <Button type="button" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button variant="filled" type="submit">
            {t("save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
