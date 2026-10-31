import {
  Anchor,
  Checkbox,
  Input,
  Text,
  type CheckboxProps,
} from "@mantine/core";
import { createContext, useContext, useEffect, type ReactNode } from "react";
import { useI18n, type MessageKey } from "./i18n";
import type { components } from "./api-schema";
import { HelpTooltip } from "./HelpTooltip";
import { FullTextTooltip } from "./FullTextTooltip";

export type SettingsSectionDraft = { value: unknown };
export type SettingsErrors = Record<
  string,
  components["schemas"]["SettingFieldError"]
>;
export const SettingsErrorsContext = createContext<{
  errors: SettingsErrors;
  clear: (key: string) => void;
}>({ errors: {}, clear: () => {} });

// Only translate server failure codes here; validation rules live in Go.
export function useSettingsError(field: string) {
  const { t } = useI18n();
  const { errors } = useContext(SettingsErrorsContext);
  const error = errors[field];
  const messages: Record<string, MessageKey> = {
    unique_id: "settingUniqueID",
    reference: "settingInvalidReference",
    number: "settingInvalidNumber",
    capability: "settingInvalidCapability",
    host: "settingInvalidHost",
    port: "settingInvalidPort",
    api_key: "settingInvalidAPIKey",
    model: "settingInvalidModel",
    duration: "settingInvalidDuration",
    size: "settingInvalidSize",
    integer: "settingInvalidInteger",
    language: "settingInvalidLanguage",
    timezone: "settingInvalidTimezone",
    listen: "settingInvalidListen",
    origin: "settingInvalidOrigin",
    origins_required: "settingOriginsRequired",
    map_url: "settingInvalidMapURL",
    endpoint: "settingInvalidEndpoint",
    proxy: "settingInvalidProxy",
    model_required: "settingModelRequired",
    choice: "settingInvalidChoice",
    link_mode: "settingInvalidLinkMode",
    interval: "settingInvalidInterval",
    required: "settingRequired",
    list: "settingInvalidList",
    text: "settingInvalidText",
  };
  return error ? t(messages[error.code] || "settingInvalidValue") : undefined;
}

// Read metadata must not be echoed as writable configuration.
export function settingsDraftValue<
  T extends {
    defaults?: unknown;
    explicit?: string[] | null;
    changed?: string[] | null;
    reset?: string[] | null;
  },
>(draft: T, saved?: T) {
  const { defaults: _defaults, explicit: _explicit, ...value } = draft;
  const changed = (draft.changed || []).filter(
    (key) =>
      !saved ||
      !saved.explicit?.includes(key) ||
      !Object.is(draft[key as keyof T], saved[key as keyof T]),
  );
  const reset = (draft.reset || []).filter(
    (key) => !saved || saved.explicit?.includes(key),
  );
  return {
    ...value,
    changed,
    reset: reset.length ? reset : undefined,
  };
}

// Compare persisted meaning while keeping the raw draft intact for typing.
// An explicitly pinned default is different from an inherited default.
export function hasSettingChange(
  field: {
    kind: string;
    value: unknown;
    explicit: boolean;
    unlimited?: boolean;
  },
  draft: unknown,
) {
  if (draft === null) return field.explicit;
  if (!field.explicit) return true;
  const normalize = (value: unknown): unknown => {
    if (
      field.unlimited &&
      typeof value === "string" &&
      value.trim().toLowerCase() === "inf"
    )
      return "inf";
    if (field.kind === "list" && Array.isArray(value))
      return value.map((v) => String(v).trim()).filter(Boolean);
    if (field.kind === "bytes" && typeof value === "string") {
      const match = /^([\d.,]+)\s*([kmgtpe]?)(i?)(b?)$/i.exec(value.trim());
      if (match && (!match[3] || match[2])) {
        const power = match[2]
          ? "kmgtpe".indexOf(match[2].toLowerCase()) + 1
          : 0;
        const bytes = Math.trunc(
          Number(match[1].replaceAll(",", "")) *
            (match[3] ? 1024 : 1000) ** power,
        );
        if (Number.isSafeInteger(bytes)) return bytes;
      }
    }
    return value;
  };
  return (
    JSON.stringify(normalize(draft)) !== JSON.stringify(normalize(field.value))
  );
}

export function SettingsFieldHeading({
  label,
  help,
  htmlFor,
  children,
  action,
}: {
  label: string;
  help?: string;
  htmlFor?: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="setting-field-heading">
      <Input.Label htmlFor={htmlFor}>{label}</Input.Label>
      {help && <HelpTooltip label={label}>{help}</HelpTooltip>}
      {children}
      {action && <div className="setting-field-action">{action}</div>}
    </div>
  );
}

export function SettingsCheckbox({
  label,
  help,
  action,
  ...props
}: Omit<CheckboxProps, "label" | "description"> & {
  label: string;
  help: string;
  action?: ReactNode;
}) {
  return (
    <div className="setting-checkbox setting-field">
      <Checkbox {...props} label={label} />
      <HelpTooltip label={label}>{help}</HelpTooltip>
      {action && <div className="setting-field-action">{action}</div>}
    </div>
  );
}

export function SettingsHint({ children }: { children: ReactNode }) {
  return <Text className="settings-hint">{children}</Text>;
}

// Keep controls in a separate task section aware of unsaved configuration.
export function useSettingsPending(
  pending: boolean,
  onChange?: (pending: boolean) => void,
) {
  useEffect(() => {
    onChange?.(pending);
  }, [pending, onChange]);
  useEffect(() => () => onChange?.(false), [onChange]);
}

export function SettingsReset({
  disabled,
  specified,
  onReset,
}: {
  disabled: boolean;
  specified: boolean;
  onReset: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="setting-source-actions">
      {specified && (
        <Anchor
          component="button"
          type="button"
          size="xs"
          className="text-action-link setting-reset"
          disabled={disabled}
          onClick={onReset}
        >
          {t("settingReset")}
        </Anchor>
      )}
      <FullTextTooltip
        text={t(specified ? "settingSpecifiedHint" : "settingInheritedHint")}
      >
        <Text
          component="span"
          className={
            specified
              ? "setting-source"
              : "setting-source setting-source--default"
          }
        >
          {t(specified ? "settingSpecified" : "settingInherited")}
        </Text>
      </FullTextTooltip>
    </div>
  );
}
