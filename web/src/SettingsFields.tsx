import {
  Anchor,
  Checkbox,
  Input,
  Text,
  type CheckboxProps,
} from "@mantine/core";
import { useEffect, type ReactNode } from "react";
import { useI18n } from "./i18n";
import { HelpTooltip } from "./HelpTooltip";
import { FullTextTooltip } from "./FullTextTooltip";

export type SettingsSectionDraft = { value: unknown; valid: boolean };

// Read metadata must not be echoed as writable configuration.
export function settingsDraftValue<
  T extends { defaults?: unknown; explicit?: unknown },
>(draft: T) {
  const { defaults: _defaults, explicit: _explicit, ...value } = draft;
  return {
    ...value,
    changed: (draft as T & { changed?: string[] }).changed || [],
  };
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
