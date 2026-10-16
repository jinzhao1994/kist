import {
  ActionIcon,
  Button,
  CheckIcon,
  Menu,
  Combobox,
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
  keyword: [],
  name_contains: "",
  name_equals: "",
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
  | "name_contains"
  | "name_equals"
  | "box"
  | "kind"
  | "from"
  | "to"
  | "tag_all"
  | "tag_any"
  | "tag_not";
const fields = [
  ["name_contains", "name"],
  ["box", "boxes"],
  ["kind", "kind"],
  ["tag_all", "tags"],
  ["from", "from"],
  ["to", "to"],
] as const;
const kinds = ["image", "video", "audio", "document", "other"] as const;
const isScalar = (field: Field) =>
  ["name_contains", "name_equals", "from", "to"].includes(field);
export function normalizedFilters<T extends FilterValues>(value: T): T {
  return {
    ...value,
    name: "",
    keyword: [...(value.keyword || []), ...(value.name ? [value.name] : [])],
  };
}
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
  const [working, setWorking] = useState(() => normalizedFilters(value));
  const [previous, setPrevious] = useState(value);
  const [field, setField] = useState<Field | null>(null);
  const [stage, setStage] = useState<"field" | "operator" | "value">("field");
  const [editing, setEditing] = useState<Field | number | null>(null);
  const [draft, setDraft] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const combo = useCombobox({
    onDropdownClose: () => combo.resetSelectedOption(),
  });
  const reset = () => {
    setField(null);
    setDraft("");
    setStage("field");
    setEditing(null);
    combo.resetSelectedOption();
  };
  // Navigation replaces the draft; sorting alone preserves unsubmitted conditions.
  if (previous !== value) {
    setPrevious(value);
    const conditions = ({ sort: _sort, asc: _asc, ...rest }: FilterValues) =>
      JSON.stringify(rest);
    if (conditions(previous) === conditions(value))
      setWorking({ ...working, sort: value.sort, asc: value.asc });
    else {
      setWorking(normalizedFilters(value));
      setField(null);
      setDraft("");
      setStage("field");
      setEditing(null);
    }
  }
  const flat = flattenTags(tags);
  const change = (patch: Partial<FilterValues>) =>
    onChange({ ...value, ...patch });
  const visibleFields = fields.filter(
    ([key]) =>
      (entity !== "boxes" || (key !== "kind" && key !== "box")) &&
      (key !== "box" || boxes !== undefined),
  );
  const title = (key: Field) =>
    key.startsWith("name_")
      ? t("name")
      : key.startsWith("tag_")
        ? t("tags")
        : t(fields.find(([id]) => id === key)![1]);
  const operator = (key: Field) =>
    key === "name_contains"
      ? t("contains")
      : key === "tag_all"
        ? t("tagAll")
        : key === "tag_any"
          ? t("tagAny")
          : key === "tag_not"
            ? t("tagNot")
            : key === "from"
              ? "≥"
              : key === "to"
                ? "≤"
                : "=";
  const operators = (key: Field): Field[] =>
    key.startsWith("name_")
      ? ["name_contains", "name_equals"]
      : key.startsWith("tag_")
        ? ["tag_all", "tag_any", "tag_not"]
        : [key];
  const describe = (key: Field) => {
    const selected = working[key];
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
  const edit = (key: Field | number, nextStage: typeof stage) => {
    setEditing(key);
    setStage(nextStage);
    setField(typeof key === "number" ? null : key);
    setDraft(
      typeof key === "number"
        ? working.keyword?.[key] || ""
        : nextStage === "value" && isScalar(key)
          ? String(working[key] || "")
          : "",
    );
    combo.resetSelectedOption();
    combo.openDropdown();
    requestAnimationFrame(() => input.current?.focus());
  };
  const needle = draft.trim().toLocaleLowerCase();
  const matches = (text: string) => text.toLocaleLowerCase().includes(needle);
  const options =
    field === "kind"
      ? kinds.map((kind) => ({ value: kind, label: t(kind) }))
      : field === "box"
        ? (boxes || []).map((box) => ({ value: box.id, label: box.name }))
        : flat.map(tagOption);
  const invalid =
    (field === "from" || field === "to") &&
    !!draft.trim() &&
    !validDate(draft.trim());
  const textResult = (): T | null => {
    if (invalid) return null;
    if (!draft.trim()) return field ? null : working;
    if (field && (!isScalar(field) || stage !== "value")) return null;
    const next = { ...working };
    if (field) {
      if (typeof editing === "string" && editing !== field)
        Object.assign(next, { [editing]: isScalar(editing) ? "" : [] });
      Object.assign(next, { [field]: draft.trim() });
      if (typeof editing === "number")
        next.keyword = next.keyword?.filter((_, index) => index !== editing);
    } else {
      const terms = [...(next.keyword || [])];
      if (typeof editing === "number") terms[editing] = draft.trim();
      else terms.push(draft.trim());
      next.keyword = terms;
    }
    return next;
  };
  const commitText = (execute: boolean) => {
    const next = textResult();
    if (!next) return;
    setWorking(next);
    reset();
    if (execute) {
      onChange(next);
      combo.closeDropdown();
    } else combo.openDropdown();
    requestAnimationFrame(() => {
      input.current?.focus();
      if (execute) combo.closeDropdown();
    });
  };
  const submit = (option: string) => {
    if (option.startsWith("field:")) {
      const next = option.slice(6) as Field;
      setField(next);
      setDraft("");
      setStage(operators(next).length > 1 ? "operator" : "value");
    } else if (option.startsWith("operator:")) {
      const nextField = option.slice(9) as Field;
      setField(nextField);
      setStage("value");
      setDraft(
        typeof editing === "string" && isScalar(editing)
          ? String(working[editing] || "")
          : "",
      );
      if (
        typeof editing === "string" &&
        editing.startsWith("tag_") &&
        nextField.startsWith("tag_")
      ) {
        const values = [
          ...new Set([
            ...(working[editing] || []),
            ...(working[nextField] || []),
          ]),
        ];
        setWorking({ ...working, [editing]: [], [nextField]: values });
        reset();
      }
    } else if (option === "apply") {
      commitText(false);
      return;
    } else if (field && option.startsWith("value:")) {
      const item = option.slice(6);
      const selected = working[field];
      const values = Array.isArray(selected) ? selected : [];
      const next = {
        ...working,
        [field]: values.includes(item)
          ? values.filter((id) => id !== item)
          : [...values, item],
      };
      if (typeof editing === "string" && editing !== field)
        Object.assign(next, { [editing]: isScalar(editing) ? "" : [] });
      if (typeof editing === "number")
        next.keyword = next.keyword?.filter((_, index) => index !== editing);
      setWorking(next);
      reset();
    }
    combo.resetSelectedOption();
    combo.openDropdown();
    requestAnimationFrame(() => input.current?.focus());
  };
  const active = (
    [
      "name_contains",
      "name_equals",
      "box",
      "kind",
      "tag_all",
      "tag_any",
      "tag_not",
      "from",
      "to",
    ] as Field[]
  ).filter((key) => !!working[key]?.length);
  const remove = (key: Field | number) => {
    setWorking(
      typeof key === "number"
        ? { ...working, keyword: working.keyword?.filter((_, i) => i !== key) }
        : { ...working, [key]: isScalar(key) ? "" : [] },
    );
    reset();
    input.current?.focus();
  };
  const editor = (
    <Combobox.DropdownTarget>
      <div className="filter-inline-editor">
        {field && (
          <span className="filter-field-label">
            {title(field)}
            {stage === "value" && ` ${operator(field)}`}
          </span>
        )}
        <Combobox.EventsTarget withExpandedAttribute>
          <input
            ref={input}
            role="combobox"
            aria-label={field ? title(field) : t("searchConditions")}
            placeholder={
              field
                ? t(stage === "operator" ? "chooseOperator" : "filterValue")
                : t("searchConditions")
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
              if (event.key === "Backspace" && !draft) {
                if (field) reset();
                else if (working.keyword?.length)
                  edit(working.keyword.length - 1, "value");
                else if (active.length)
                  edit(active[active.length - 1], "value");
              }
              if (
                event.key === "Enter" &&
                combo.getSelectedOptionIndex() === -1
              ) {
                event.preventDefault();
                commitText(true);
              }
            }}
          />
        </Combobox.EventsTarget>
      </div>
    </Combobox.DropdownTarget>
  );
  return (
    <section className="filters-bar" aria-label={t("filters")}>
      <div className="filter-search">
        <Combobox
          store={combo}
          onOptionSubmit={submit}
          position="bottom-start"
          width={300}
          offset={6}
          withinPortal
          middlewares={{ flip: true, shift: { padding: 16 } }}
        >
          <div className="filter-search-input">
            <div
              className="filter-input-content"
              onClick={(event) => {
                if (event.target === event.currentTarget)
                  input.current?.focus();
              }}
            >
              {active.map((key) =>
                editing === key ? (
                  <div key={key} className="filter-edit-slot">
                    {editor}
                  </div>
                ) : (
                  <div className="filter-token" key={key}>
                    <UnstyledButton
                      className="filter-token-segment"
                      onClick={() => edit(key, "field")}
                    >
                      {title(key)}
                    </UnstyledButton>
                    <UnstyledButton
                      className="filter-token-segment"
                      aria-label={`${t("operator")}: ${title(key)}`}
                      onClick={() => edit(key, "operator")}
                    >
                      {operator(key)}
                    </UnstyledButton>
                    <UnstyledButton
                      className="filter-token-segment filter-token-value"
                      aria-label={`${title(key)}: ${describe(key)}`}
                      onClick={() => edit(key, "value")}
                    >
                      {describe(key)}
                    </UnstyledButton>
                    <ActionIcon
                      size="xs"
                      variant="transparent"
                      color="gray"
                      aria-label={`${t("remove")}: ${title(key)}`}
                      onClick={() => remove(key)}
                    >
                      ×
                    </ActionIcon>
                  </div>
                ),
              )}
              {(working.keyword || []).map((term, index) =>
                editing === index ? (
                  <div key={`keyword:${index}`} className="filter-edit-slot">
                    {editor}
                  </div>
                ) : (
                  <div
                    className="filter-token filter-keyword"
                    key={`keyword:${index}`}
                  >
                    <UnstyledButton
                      className="filter-token-segment"
                      onClick={() => edit(index, "value")}
                    >
                      {term}
                    </UnstyledButton>
                    <ActionIcon
                      size="xs"
                      variant="transparent"
                      color="gray"
                      aria-label={`${t("remove")}: ${term}`}
                      onClick={() => remove(index)}
                    >
                      ×
                    </ActionIcon>
                  </div>
                ),
              )}
              {editing === null && editor}
            </div>
            {(active.length > 0 || !!working.keyword?.length || !!draft) && (
              <ActionIcon
                variant="transparent"
                color="gray"
                aria-label={t("clearFilters")}
                onClick={() => {
                  const next = {
                    ...value,
                    ...emptyFilters,
                    sort: value.sort,
                    asc: value.asc,
                  };
                  setWorking(next);
                  onChange(next);
                  reset();
                  input.current?.focus();
                }}
              >
                ×
              </ActionIcon>
            )}
            <ActionIcon
              className="filter-submit"
              variant="transparent"
              color="gray"
              aria-label={t("executeSearch")}
              onClick={() => commitText(true)}
            >
              <svg
                width="19"
                height="19"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.7"
                aria-hidden="true"
              >
                <circle cx="10.5" cy="10.5" r="6.5" />
                <path d="m16 16 5 5" />
              </svg>
            </ActionIcon>
          </div>
          <Combobox.Dropdown className="filter-editor">
            <Combobox.Options className="filter-options">
              {(stage === "field" &&
                editing !== null &&
                typeof editing !== "number") ||
              !field ? (
                <>
                  {!!draft.trim() && (
                    <Combobox.Option value="apply">
                      {t("searchText", { value: draft.trim() })}
                    </Combobox.Option>
                  )}
                  {visibleFields
                    .filter(([key]) => matches(title(key)) || matches(key))
                    .map(([key]) => (
                      <Combobox.Option key={key} value={`field:${key}`}>
                        {title(key)}
                        <span aria-hidden="true" className="filter-option-hint">
                          {t("filterCondition")}
                        </span>
                      </Combobox.Option>
                    ))}
                </>
              ) : stage === "operator" ? (
                operators(field).map((key) => (
                  <Combobox.Option value={`operator:${key}`} key={key}>
                    {operator(key)}
                  </Combobox.Option>
                ))
              ) : isScalar(field) ? (
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
                      const checked = working[field]?.includes(option.value);
                      return (
                        <Combobox.Option
                          key={option.value}
                          value={`value:${option.value}`}
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
