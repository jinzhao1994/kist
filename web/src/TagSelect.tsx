import { Combobox, InputBase, ScrollArea, useCombobox } from "@mantine/core";
import { useState } from "react";
import { TagPath, type TagOption } from "./TagPath";
import { useI18n } from "./i18n";

// A button target preserves rich path labels when closed; search stays in
// the dropdown so it never replaces the selected label with indented text.
export function TagSelect({
  id,
  value,
  onValueChange,
  options,
  disabled,
}: {
  id: string;
  value: string;
  onValueChange: (value: string) => void;
  options: TagOption[];
  disabled?: boolean;
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
  const selected = options.find((option) => option.value === value);
  const visible = options.filter((option) =>
    option.label
      .toLocaleLowerCase()
      .includes(search.trim().toLocaleLowerCase()),
  );
  return (
    <Combobox
      store={store}
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
          id={id}
          component="button"
          type="button"
          role="combobox"
          disabled={disabled}
          pointer
          rightSection={<Combobox.Chevron />}
          rightSectionPointerEvents="none"
          classNames={{ input: "tag-select-input" }}
          onClick={() => store.toggleDropdown()}
        >
          {selected && <TagPath option={selected} />}
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
          placeholder={t("searchTags")}
          aria-label={t("searchTags")}
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
                >
                  <TagPath option={option} />
                </Combobox.Option>
              ))
            ) : (
              <Combobox.Empty>{t("noFilterOptions")}</Combobox.Empty>
            )}
          </ScrollArea.Autosize>
        </Combobox.Options>
      </Combobox.Dropdown>
    </Combobox>
  );
}
