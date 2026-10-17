import { Select as MantineSelect, type SelectProps } from "@mantine/core";
export function Select({
  id,
  "aria-label": ariaLabel,
  value,
  onValueChange,
  options,
  disabled,
  label,
  searchable = false,
  renderOption,
  nothingFoundMessage,
}: {
  id?: string;
  "aria-label"?: string;
  value: string;
  onValueChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
  label?: string;
  searchable?: boolean;
  renderOption?: SelectProps["renderOption"];
  nothingFoundMessage?: string;
}) {
  return (
    <MantineSelect
      id={id}
      aria-label={ariaLabel}
      label={label}
      value={value}
      onChange={(next) => {
        if (next !== null) onValueChange(next);
      }}
      data={options}
      disabled={disabled}
      searchable={searchable}
      renderOption={renderOption}
      nothingFoundMessage={nothingFoundMessage}
      allowDeselect={false}
    />
  );
}
