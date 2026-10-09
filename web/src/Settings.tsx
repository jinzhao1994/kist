import { useId, useState } from "react";
import { Dialog } from "./Dialog";
import { useI18n, validFormats, type Preferences } from "./i18n";
import { themes, useTheme, type Appearance } from "./theme";
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
        <label htmlFor={`${id}-theme`}>{t("theme")}</label>
        <select
          id={`${id}-theme`}
          value={look.theme}
          onChange={(e) =>
            setLook({ ...look, theme: e.target.value as Appearance["theme"] })
          }
        >
          {Object.entries(themes).map(([key, label]) => (
            <option key={key} value={key}>
              {t(label)}
            </option>
          ))}
        </select>
        <label htmlFor={`${id}-mode`}>{t("colorMode")}</label>
        <select
          id={`${id}-mode`}
          value={look.mode}
          onChange={(e) =>
            setLook({ ...look, mode: e.target.value as Appearance["mode"] })
          }
        >
          <option value="system">{t("modeSystem")}</option>
          <option value="light">{t("modeLight")}</option>
          <option value="dark">{t("modeDark")}</option>
        </select>
        <label htmlFor={`${id}-language`}>{t("language")}</label>
        <select
          id={`${id}-language`}
          value={draft.language}
          onChange={(e) =>
            field("language", e.target.value as Preferences["language"])
          }
        >
          <option value="en">English</option>
          <option value="zh">中文</option>
        </select>
        <label htmlFor={`${id}-locale`}>{t("formatLocale")}</label>
        <input
          id={`${id}-locale`}
          value={draft.formatLocale}
          onChange={(e) => field("formatLocale", e.target.value)}
          placeholder="zh-CN / en-US / de-DE"
          required
        />
        <label htmlFor={`${id}-zone`}>{t("timezone")}</label>
        <input
          id={`${id}-zone`}
          value={draft.timezone}
          onChange={(e) => field("timezone", e.target.value)}
          placeholder="Asia/Shanghai"
          required
        />
        <label htmlFor={`${id}-direction`}>{t("direction")}</label>
        <select
          id={`${id}-direction`}
          value={draft.direction}
          onChange={(e) =>
            field("direction", e.target.value as Preferences["direction"])
          }
        >
          <option value="auto">{t("auto")}</option>
          <option value="ltr">{t("ltr")}</option>
          <option value="rtl">{t("rtl")}</option>
        </select>
        {error && <p role="alert">{t("invalidSettings")}</p>}
        <div className="actions">
          <button type="button" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </button>
          <button className="primary">{t("save")}</button>
        </div>
      </form>
    </Dialog>
  );
}
