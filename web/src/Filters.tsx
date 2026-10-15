import {
  ActionIcon,
  Button,
  CheckIcon,
  Menu,
  Combobox,
  PillsInput,
  UnstyledButton,
  useCombobox,
} from "@mantine/core";
import { useRef, useState, type ReactNode } from "react";
import type { FileQuery, BoxQuery, Box, Tag } from "./api";
import { flattenTags, tagOption, TagPath } from "./TagPath";
import { useI18n } from "./i18n";
export type Filters = Omit<FileQuery, "folder" | "cursor" | "limit">;
export const emptyFilters: Filters = {
  name: "",
  sort: "updated",
  asc: false,
  from: "",
  to: "",
  box: [],
  kind: [],
  tag_all: [],
  tag_any: [],
  tag_not: [],
};
export type BoxFilters = Omit<BoxQuery, "text" | "cursor" | "limit"> & {
  name?: string;
  kind?: string[] | null;
};
export const emptyBoxFilters: BoxFilters = { ...emptyFilters, sort: "updated" };
type FilterValues = Omit<Filters, "sort"> & {
  sort?: Filters["sort"] | BoxFilters["sort"];
};
type Field =
  "name" | "box" | "kind" | "from" | "to" | "tag_all" | "tag_any" | "tag_not";
const fields = [
  ["name", "filename"],
  ["box", "boxes"],
  ["kind", "kind"],
  ["tag_all", "tagAll"],
  ["tag_any", "tagAny"],
  ["tag_not", "tagNot"],
  ["from", "from"],
  ["to", "to"],
] as const;
const kinds = ["image", "video", "audio", "document", "other"] as const;
function validDate(value: string) {
  if (!/^\d{4}(-\d{2}(-\d{2})?)?$/.test(value)) return false;
  const [year, month = 1, day = 1] = value.split("-").map(Number);
  if (!year || month < 1 || month > 12 || day < 1) return false;
  const date = new Date(0);
  date.setUTCFullYear(year, month, 0);
  return day <= date.getUTCDate();
}
export function FiltersPanel<T extends FilterValues>({
  value,
  onChange,
  tags,
  entity = "files",
  boxes,
  boxesLoading = false,
  boxesError = false,
  retryBoxes,
  saveSearch,
  toolbar,
  hideSort = false,
}: {
  value: T;
  onChange: (value: T) => void;
  tags: Tag[];
  entity?: "files" | "boxes";
  boxes?: Box[];
  boxesLoading?: boolean;
  boxesError?: boolean;
  retryBoxes?: () => void;
  saveSearch?: ReactNode;
  toolbar?: ReactNode;
  hideSort?: boolean;
}) {
  const { t } = useI18n();
  const [field, setField] = useState<Field | null>(null);
  const [draft, setDraft] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const combo = useCombobox({
    onDropdownClose: () => combo.resetSelectedOption(),
  });
  const flat = flattenTags(tags);
  const change = (patch: Partial<FilterValues>) =>
    onChange({ ...value, ...patch });
  const visibleFields = fields.filter(
    ([key]) =>
      (entity !== "boxes" || (key !== "kind" && key !== "box")) &&
      (key !== "box" || boxes !== undefined),
  );
  const title = (key: Field) =>
    key === "name"
      ? t(entity === "boxes" ? "searchBoxes" : "filename")
      : t(fields.find(([id]) => id === key)![1]);
  const reset = () => {
    setField(null);
    setDraft("");
    combo.resetSelectedOption();
  };
  const edit = (next: Field) => {
    setField(next);
    setDraft(
      next === "name" || next === "from" || next === "to"
        ? value[next] || ""
        : "",
    );
    combo.resetSelectedOption();
    combo.openDropdown();
    input.current?.focus();
  };
  const describe = (key: Field) => {
    const selected = value[key];
    if (!Array.isArray(selected)) return selected || "";
    return selected
      .map((item) =>
        key === "kind"
          ? t(item as (typeof kinds)[number])
          : key === "box"
            ? boxes?.find((box) => box.id === item)?.name || item
            : flat.map(tagOption).find((tag) => tag.value === item)?.label ||
              item,
      )
      .join(", ");
  };
  const needle = draft.trim().toLocaleLowerCase();
  const matches = (text: string) => text.toLocaleLowerCase().includes(needle);
  const options =
    field === "kind"
      ? kinds.map((kind) => ({ value: kind, label: t(kind) }))
      : field === "box"
        ? (boxes || []).map((box) => ({ value: box.id, label: box.name }))
        : flat.map(tagOption);
  const scalar = field === "name" || field === "from" || field === "to";
  const invalid =
    (field === "from" || field === "to") &&
    !!draft.trim() &&
    !validDate(draft.trim());
  const commitText = () => {
    if (invalid || !draft.trim()) return;
    change({ [field || "name"]: draft.trim() });
    reset();
    combo.openDropdown();
  };
  const submit = (option: string) => {
    if (option.startsWith("field:")) {
      edit(option.slice(6) as Field);
      return;
    }
    if (option === "apply") {
      commitText();
      return;
    }
    if (!field || scalar) return;
    const item = option.slice(6);
    const selected = value[field] || [];
    change({
      [field]: selected.includes(item)
        ? selected.filter((id) => id !== item)
        : [...selected, item],
    });
    reset();
    combo.openDropdown();
    input.current?.focus();
  };
  const active = visibleFields.filter(([key]) => !!value[key]?.length);
  return (
    <section className="filters-bar" aria-label={t("filters")}>
      <div className="filter-search">
        <Combobox
          store={combo}
          onOptionSubmit={submit}
          position="bottom-start"
          withinPortal
        >
          <Combobox.DropdownTarget>
            <PillsInput
              className="filter-search-input"
              onClick={() => input.current?.focus()}
            >
              <div className="filter-input-content">
                {active.map(([key]) => (
                  <div className="filter-token" key={key}>
                    <UnstyledButton
                      className="filter-token-label"
                      onClick={(event) => {
                        event.stopPropagation();
                        edit(key);
                      }}
                    >
                      {title(key)}: {describe(key)}
                    </UnstyledButton>
                    <ActionIcon
                      size="sm"
                      variant="subtle"
                      aria-label={`${t("remove")}: ${title(key)}`}
                      onClick={() =>
                        change({
                          [key]:
                            key === "name" || key === "from" || key === "to"
                              ? ""
                              : [],
                        })
                      }
                    >
                      ×
                    </ActionIcon>
                  </div>
                ))}
                {field && (
                  <span className="filter-field-label">{title(field)}:</span>
                )}
                <Combobox.EventsTarget withExpandedAttribute>
                  <PillsInput.Field
                    role="combobox"
                    ref={input}
                    aria-label={field ? title(field) : t("searchConditions")}
                    placeholder={
                      field ? t("filterValue") : t("searchConditions")
                    }
                    value={draft}
                    onFocus={() => combo.openDropdown()}
                    onClick={() => combo.openDropdown()}
                    onBlur={() => combo.closeDropdown()}
                    onChange={(event) => {
                      setDraft(event.currentTarget.value);
                      combo.resetSelectedOption();
                      combo.openDropdown();
                    }}
                    onKeyDown={(event) => {
                      if (event.nativeEvent.isComposing) return;
                      if (event.key === "Escape") {
                        reset();
                        combo.closeDropdown();
                      }
                      if (event.key === "Backspace" && !draft && field) reset();
                      if (
                        event.key === "Enter" &&
                        combo.getSelectedOptionIndex() === -1 &&
                        (!field || scalar)
                      ) {
                        event.preventDefault();
                        commitText();
                      }
                    }}
                  />
                </Combobox.EventsTarget>
                {active.length > 0 && (
                  <ActionIcon
                    variant="subtle"
                    color="gray"
                    aria-label={t("clearFilters")}
                    onClick={() => {
                      onChange({
                        ...value,
                        ...emptyFilters,
                        sort: value.sort,
                        asc: value.asc,
                      });
                      reset();
                    }}
                  >
                    ×
                  </ActionIcon>
                )}
              </div>
            </PillsInput>
          </Combobox.DropdownTarget>
          <Combobox.Dropdown className="filter-editor">
            <Combobox.Header>
              {field ? title(field) : t("chooseFilter")}
            </Combobox.Header>
            <Combobox.Options className="filter-options">
              {!field ? (
                <>
                  {visibleFields
                    .filter(([key]) => matches(title(key)) || matches(key))
                    .map(([key]) => (
                      <Combobox.Option key={key} value={`field:${key}`}>
                        {title(key)}
                      </Combobox.Option>
                    ))}
                  {draft.trim() && (
                    <Combobox.Option value="apply">
                      {t("searchText", { value: draft.trim() })}
                    </Combobox.Option>
                  )}
                </>
              ) : scalar ? (
                <>
                  {(field === "from" || field === "to") && (
                    <Combobox.Empty>
                      {t(entity === "boxes" ? "boxDateHint" : "filterDateHint")}
                    </Combobox.Empty>
                  )}
                  {invalid && <p role="alert">{t("invalidDate")}</p>}
                  <Combobox.Option
                    value="apply"
                    disabled={!draft.trim() || invalid}
                  >
                    {t("applyFilter")}
                    {draft.trim() ? `: ${draft.trim()}` : ""}
                  </Combobox.Option>
                </>
              ) : (
                <>
                  {field === "box" && boxesLoading && (
                    <Combobox.Empty>{t("loading")}</Combobox.Empty>
                  )}
                  {field === "box" && boxesError && (
                    <Combobox.Empty>
                      <Button variant="subtle" onClick={retryBoxes}>
                        {t("retry")}
                      </Button>
                    </Combobox.Empty>
                  )}
                  {options
                    .filter((option) => matches(option.label))
                    .map((option) => {
                      const tag = field.startsWith("tag_")
                        ? flat.find((tag) => tag.id === option.value)
                        : undefined;
                      const checked = value[field]?.includes(option.value);
                      return (
                        <Combobox.Option
                          value={`value:${option.value}`}
                          key={option.value}
                          active={checked}
                        >
                          <span className="filter-option-value">
                            {checked && <CheckIcon size={12} aria-hidden />}
                            {tag ? (
                              <TagPath option={tagOption(tag)} />
                            ) : (
                              option.label
                            )}
                          </span>
                        </Combobox.Option>
                      );
                    })}
                  {!options.some((option) => matches(option.label)) &&
                    !(field === "box" && (boxesLoading || boxesError)) && (
                      <Combobox.Empty>{t("noFilterOptions")}</Combobox.Empty>
                    )}
                </>
              )}
            </Combobox.Options>
            {field && (
              <Combobox.Footer>
                <Button
                  size="compact-sm"
                  variant="subtle"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => {
                    reset();
                    input.current?.focus();
                  }}
                >
                  {t("chooseFilter")}
                </Button>
              </Combobox.Footer>
            )}
          </Combobox.Dropdown>
        </Combobox>
      </div>
      {saveSearch}
      {!hideSort && (
        <Menu position="bottom-end">
          <Menu.Target>
            <ActionIcon
              size="lg"
              variant="default"
              aria-label={t("sort")}
              title={t("sort")}
            >
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                aria-hidden="true"
              >
                <path d="M4 6h12M4 12h8M4 18h4M18 9v11m-3-3 3 3 3-3" />
              </svg>
            </ActionIcon>
          </Menu.Target>
          <Menu.Dropdown>
            {(
              (entity === "boxes"
                ? [
                    ["activity", "activityDate"],
                    ["name", "name"],
                    ["updated", "updated"],
                  ]
                : [
                    ["updated", "updated"],
                    ["captured", "captured"],
                    ["imported", "imported"],
                    ["name", "filename"],
                    ["type", "kind"],
                    ["size", "fileSize"],
                  ]) as [
                T["sort"],
                (
                  | "activityDate"
                  | "name"
                  | "updated"
                  | "captured"
                  | "imported"
                  | "filename"
                  | "kind"
                  | "fileSize"
                ),
              ][]
            ).map(([sort, label]) => (
              <Menu.Item
                key={sort}

                data-checked={value.sort === sort}
                rightSection={value.sort === sort ? "✓" : null}
                onClick={() => change({ sort })}
              >
                {t(label)}
              </Menu.Item>
            ))}
            <Menu.Divider />
            <Menu.Item onClick={() => change({ asc: !value.asc })}>
              {t(value.asc ? "ascending" : "descending")}{" "}
              {value.asc ? "↑" : "↓"}
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      )}
      {toolbar}
    </section>
  );
}
