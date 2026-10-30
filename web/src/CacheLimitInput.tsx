import { Autocomplete } from "@mantine/core";
import { useI18n } from "./i18n";

// Keep the wire value independent of the display language and default source.
export function CacheLimitInput({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const { t } = useI18n();
  const unlimited = t("settingNoLimit");
  return (
    <Autocomplete
      id={id}
      aria-label={label}
      disabled={disabled}
      value={value.trim().toLowerCase() === "inf" ? unlimited : value}
      data={[unlimited]}
      filter={({ options }) => options}
      onChange={(next) =>
        onChange(
          next.trim() === unlimited || next.trim().toLowerCase() === "inf"
            ? "inf"
            : next,
        )
      }
    />
  );
}
