import {
  Select as MantineSelect,
  Tooltip,
  CheckIcon,
  Combobox,
  InputBase,
  ScrollArea,
  useCombobox,
  type SelectProps,
} from "@mantine/core";
import { useState } from "react";
import { useI18n } from "./i18n";

export function Select({
  id,
  "aria-label": ariaLabel,
  "data-autofocus": autofocus,
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
  "data-autofocus"?: boolean;
  value: string;
  onValueChange: (value: string) => void;
  options: {
    value: string;
    label: string;
    special?: boolean;
    disabled?: boolean;
  }[];
  disabled?: boolean;
  label?: string;
  searchable?: boolean;
  renderOption?: SelectProps["renderOption"];
  nothingFoundMessage?: string;
}) {
  const { t } = useI18n();
  const [search, setSearch] = useState("");
  const store = useCombobox({
    onDropdownOpen: () => store.focusSearchInput(),
    onDropdownClose: () => {
      setSearch("");
      store.resetSelectedOption();
    },
  });
  const renderItem: NonNullable<SelectProps["renderOption"]> = (item) => (
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
          options.find((option) => option.value === item.option.value)?.special
            ? "select-special-option"
            : undefined
        }
      >
        {renderOption ? renderOption(item) : item.option.label}
      </span>
    </>
  );
  if (searchable) {
    const selected = options.find((option) => option.value === value);
    const visible = options.filter((option) =>
      option.label
        .toLocaleLowerCase()
        .includes(search.trim().toLocaleLowerCase()),
    );
    return (
      <Combobox
        store={store}
        keepMounted={false}
        withinPortal
        returnFocus
        onOptionSubmit={(next) => {
          onValueChange(next);
          store.closeDropdown();
          store.focusTarget();
        }}
      >
        <Combobox.Target targetType="button" withExpandedAttribute>
          <InputBase
            data-autofocus={autofocus}
            id={id}
            aria-label={ariaLabel}
            label={label}
            component="button"
            type="button"
            role="combobox"
            disabled={disabled}
            pointer
            rightSection={<Combobox.Chevron />}
            rightSectionPointerEvents="none"
            classNames={{
              root: "searchable-select",
              input: "tag-select-input",
            }}
            onClick={() => store.toggleDropdown()}
          >
            <Tooltip.Floating
              label={selected?.label}
              disabled={!selected?.label}
              position="bottom-start"
              offset={16}
              role="tooltip"
              multiline
              maw={360}
              classNames={{ tooltip: "metadata-tooltip" }}
            >
              <span className="select-value">{selected?.label}</span>
            </Tooltip.Floating>
          </InputBase>
        </Combobox.Target>
        <Combobox.Dropdown>
          <Combobox.Search
            value={search}
            onChange={(event) => {
              setSearch(event.currentTarget.value);
              store.updateSelectedOptionIndex();
            }}
            onKeyDown={(event) => {
              if (event.key === "Tab") {
                store.closeDropdown();
                store.focusTarget();
              }
            }}
            placeholder={t("searchOptions")}
            aria-label={t("searchOptions")}
          />
          <Combobox.Options>
            <ScrollArea.Autosize mah={240} type="scroll">
              {visible.length ? (
                visible.map((option) => (
                  <Combobox.Option
                    key={option.value}
                    value={option.value}
                    aria-label={option.label}
                    active={option.value === value}
                    disabled={option.disabled}
                  >
                    {renderItem({ option, checked: option.value === value })}
                  </Combobox.Option>
                ))
              ) : (
                <Combobox.Empty>
                  {nothingFoundMessage || t("noFilterOptions")}
                </Combobox.Empty>
              )}
            </ScrollArea.Autosize>
          </Combobox.Options>
        </Combobox.Dropdown>
      </Combobox>
    );
  }
  return (
    <MantineSelect
      data-autofocus={autofocus}
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
      renderOption={renderItem}
      nothingFoundMessage={nothingFoundMessage}
      allowDeselect={false}
    />
  );
}
