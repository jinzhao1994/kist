import {
  Select as MantineSelect,
  CheckIcon,
  type SelectProps,
} from "@mantine/core";
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
  options: { value: string; label: string; special?: boolean }[];
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
      renderOption={(item) => (
        <>
          {!renderOption && item.checked && (
            <CheckIcon
              size={12}
              aria-hidden="true"
              style={{ flexShrink: 0, opacity: 0.4 }}
            />
          )}
          <span
            className={
              item.option.value === "" ||
              options.find((option) => option.value === item.option.value)
                ?.special
                ? "select-special-option"
                : undefined
            }
          >
            {renderOption ? renderOption(item) : item.option.label}
          </span>
        </>
      )}
      nothingFoundMessage={nothingFoundMessage}
      allowDeselect={false}
    />
  );
}
