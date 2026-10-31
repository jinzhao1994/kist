import { FloatingTooltip } from "./FloatingTooltip";
import {
  Select as MantineSelect,
  CheckIcon,
  Combobox,
  Input,
  InputBase,
  ScrollArea,
  useCombobox,
  type SelectProps,
} from "@mantine/core";
import { useRef, useState, type ReactNode } from "react";
import { useI18n } from "./i18n";
import { InputChevron } from "./InputChevron";

export function SelectOptionContent({
  selected,
  special = false,
  children,
}: {
  selected: boolean;
  special?: boolean;
  children: ReactNode;
}) {
  return (
    <span
      className="select-option-content"
      data-selected={selected || undefined}
    >
      <span
        className={`select-option-label${special ? " select-special-option" : ""}`}
      >
        {children}
      </span>
      <span className="select-option-check" aria-hidden="true">
        {selected && <CheckIcon size={12} />}
      </span>
    </span>
  );
}

export function Select({
  id,
  "aria-label": ariaLabel,
  "data-autofocus": autofocus,
  value,
  onValueChange,
  options,
  disabled,
  error,
  label,
  searchable = false,
  renderOption,
  nothingFoundMessage,
  placeholder,
  getCreateLabel,
  onBeforeOpen,
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
  error?: string;
  label?: string;
  searchable?: boolean;
  renderOption?: SelectProps["renderOption"];
  nothingFoundMessage?: string;
  placeholder?: string;
  getCreateLabel?: (value: string) => string;
  onBeforeOpen?: () => boolean | Promise<boolean>;
}) {
  const { t } = useI18n();
  const [search, setSearch] = useState("");
  const openAttempt = useRef(0);
  const store = useCombobox({
    onDropdownOpen: () => store.focusSearchInput(),
    onDropdownClose: () => {
      setSearch("");
      store.resetSelectedOption();
    },
  });
  async function requestOpen() {
    const attempt = ++openAttempt.current;
    try {
      if (onBeforeOpen && !(await onBeforeOpen())) return;
    } catch {
      // The caller owns loading and error feedback for deferred options.
      return;
    }
    const target = store.targetRef.current;
    if (
      attempt !== openAttempt.current ||
      !target ||
      target.matches(":disabled") ||
      document.activeElement !== target ||
      !document.hasFocus()
    )
      return;
    store.openDropdown();
  }
  const dropdownStore: typeof store = {
    ...store,
    openDropdown: () => {
      void requestOpen();
    },
    closeDropdown: () => {
      openAttempt.current++;
      store.closeDropdown();
    },
    toggleDropdown: () => {
      if (store.dropdownOpened) dropdownStore.closeDropdown();
      else void requestOpen();
    },
  };
  const renderItem: NonNullable<SelectProps["renderOption"]> = (item) => (
    <SelectOptionContent
      selected={!!item.checked}
      special={
        item.option.value === "" ||
        options.find((option) => option.value === item.option.value)?.special
      }
    >
      {renderOption ? renderOption(item) : item.option.label}
    </SelectOptionContent>
  );
  if (searchable) {
    const selected = options.find((option) => option.value === value);
    const customValue =
      getCreateLabel &&
      search.trim() &&
      !options.some((option) => option.value === search.trim())
        ? search.trim()
        : undefined;
    const visible = options.filter((option) =>
      option.label
        .toLocaleLowerCase()
        .includes(search.trim().toLocaleLowerCase()),
    );
    return (
      <Combobox
        store={dropdownStore}
        keepMounted={false}
        withinPortal
        returnFocus
        onOptionSubmit={(next) => {
          onValueChange(next);
          dropdownStore.closeDropdown();
          store.focusTarget();
        }}
      >
        <Combobox.Target targetType="button" withExpandedAttribute>
          <InputBase
            data-autofocus={autofocus}
            id={id}
            aria-label={ariaLabel}
            label={label}
            error={error}
            component="button"
            type="button"
            role="combobox"
            disabled={disabled}
            pointer
            onBlur={() => {
              openAttempt.current++;
            }}
            rightSection={<InputChevron error={!!error} />}
            rightSectionPointerEvents="none"
            classNames={{
              root: "searchable-select",
              input: "tag-select-input",
            }}
            onClick={() => dropdownStore.toggleDropdown()}
          >
            <FloatingTooltip
              label={selected?.label}
              disabled={!selected?.label}
              position="bottom-start"
              offset={16}
              role="tooltip"
              multiline
              maw={360}
              classNames={{ tooltip: "metadata-tooltip" }}
            >
              <span className="select-value">
                {selected?.label ?? (
                  <Input.Placeholder>{placeholder}</Input.Placeholder>
                )}
              </span>
            </FloatingTooltip>
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
                dropdownStore.closeDropdown();
                store.focusTarget();
              }
            }}
            placeholder={t("searchOptions")}
            aria-label={t("searchOptions")}
          />
          <Combobox.Options>
            <ScrollArea.Autosize mah={240} type="scroll">
              {visible.map((option) => (
                <Combobox.Option
                  key={option.value}
                  value={option.value}
                  aria-label={option.label}
                  active={option.value === value}
                  aria-selected={option.value === value}
                  disabled={option.disabled}
                >
                  {renderItem({ option, checked: option.value === value })}
                </Combobox.Option>
              ))}
              {customValue && (
                <Combobox.Option value={customValue}>
                  <SelectOptionContent selected={false}>
                    {getCreateLabel!(customValue)}
                  </SelectOptionContent>
                </Combobox.Option>
              )}
              {!visible.length && !customValue && (
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
      error={error}
      placeholder={placeholder}
      value={value}
      rightSection={<InputChevron error={!!error} />}
      rightSectionPointerEvents="none"
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
