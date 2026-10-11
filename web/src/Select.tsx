import { Select as MantineSelect } from "@mantine/core";
export function Select({
  id,
  value,
  onValueChange,
  options,
  disabled,
  label,
  searchable = false,
}: {
  id?: string;
  value: string;
  onValueChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
  label?: string;
  searchable?: boolean;
}) {
  return (
    <MantineSelect
      id={id}
      label={label}
      value={value}
      onChange={(next) => {
        if (next !== null) onValueChange(next);
      }}
      data={options}
      disabled={disabled}
      searchable={searchable}
      allowDeselect={false}
    />
  );
}
