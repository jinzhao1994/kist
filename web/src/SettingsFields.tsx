import { Checkbox, Input, Text, type CheckboxProps } from "@mantine/core";
import type { ReactNode } from "react";
import { HelpTooltip } from "./HelpTooltip";

export function SettingsFieldHeading({
  label,
  help,
  htmlFor,
  children,
}: {
  label: string;
  help?: string;
  htmlFor?: string;
  children?: ReactNode;
}) {
  return (
    <div className="setting-field-heading">
      <Input.Label htmlFor={htmlFor}>{label}</Input.Label>
      {help && <HelpTooltip label={label}>{help}</HelpTooltip>}
      {children}
    </div>
  );
}

export function SettingsCheckbox({
  label,
  help,
  ...props
}: Omit<CheckboxProps, "label" | "description"> & {
  label: string;
  help: string;
}) {
  return (
    <div className="setting-checkbox">
      <Checkbox {...props} label={label} />
      <HelpTooltip label={label}>{help}</HelpTooltip>
    </div>
  );
}

export function SettingsHint({ children }: { children: ReactNode }) {
  return <Text className="settings-hint">{children}</Text>;
}

export function SettingsTask({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <section className="settings-task">
      <h3>{title}</h3>
      {children}
    </section>
  );
}
