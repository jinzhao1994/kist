import {
  ActionIcon,
  Button,
  Menu,
  Combobox,
  UnstyledButton,
  useCombobox,
} from "@mantine/core";
import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "./api-schema";
import type { FileQuery, BoxQuery, Box, Tag } from "./api";
import { flattenTags, tagOption, TagPath } from "./TagPath";
import { useI18n } from "./i18n";
export type Filters = Omit<
  FileQuery,
  "folder" | "recursive" | "cursor" | "limit"
>;
export const emptyFilters: Filters = {
  condition: [],
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
export type SearchCondition = components["schemas"]["SearchCondition"];
type Field =
  | "keyword"
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
const scalar = (field: Field) =>
  ["keyword", "name_contains", "name_equals", "from", "to"].includes(field);
export function searchConditions(value: FilterValues): SearchCondition[] {
  const result: SearchCondition[] = [];
  for (const raw of value.condition || []) {
    try {
      const c = JSON.parse(raw);
      if (
        typeof c.field === "string" &&
        Array.isArray(c.values) &&
        c.values.every((v: unknown) => typeof v === "string")
      )
        result.push(c);
    } catch {
      /* Invalid URL clauses remain server-validated until edited. */
    }
  }
  for (const term of [
    ...(value.keyword || []),
    ...(value.name ? [value.name] : []),
  ])
    result.push({ field: "keyword", values: [term] });
  for (const field of ["name_contains", "name_equals", "from", "to"] as const)
    if (value[field]) result.push({ field, values: [value[field]] });
  for (const id of value.tag_all || [])
    result.push({ field: "tag_all", values: [id] });
  // Preserve the meaning of legacy union/exclusion groups instead of splitting them.
  for (const field of ["box", "kind", "tag_any", "tag_not"] as const)
    if (value[field]?.length) result.push({ field, values: [...value[field]] });
  return result;
}
export function normalizedFilters<T extends FilterValues>(value: T): T {
  return {
    ...value,
    ...emptyFilters,
    sort: value.sort,
    asc: value.asc,
    condition: searchConditions(value).map((c) => JSON.stringify(c)),
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
  const [field, setField] = useState<Field>("keyword");
  const [stage, setStage] = useState<"field" | "operator" | "value">("field");
  const [editing, setEditing] = useState<number | null>(null);
  const [draft, setDraft] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const restoreFocus = useRef<boolean | null>(null);
  const suppressFocusDropdown = useRef(false);
  const combo = useCombobox({
    onDropdownClose: () => combo.resetSelectedOption(),
  });
  useLayoutEffect(() => {
    if (restoreFocus.current !== null) {
      suppressFocusDropdown.current = restoreFocus.current;
      input.current?.focus();
      suppressFocusDropdown.current = false;
      if (restoreFocus.current) combo.closeDropdown();
      restoreFocus.current = null;
    }
  });
  const reset = () => {
    setField("keyword");
    setStage("field");
    setEditing(null);
    setDraft("");
    combo.resetSelectedOption();
  };
  if (previous !== value) {
    setPrevious(value);
    const conditions = ({ sort: _sort, asc: _asc, ...rest }: FilterValues) =>
      JSON.stringify(rest);
    if (conditions(previous) === conditions(value))
      setWorking({ ...working, sort: value.sort, asc: value.asc });
    else {
      setWorking(normalizedFilters(value));
      setField("keyword");
      setStage("field");
      setEditing(null);
      setDraft("");
    }
  }
  const clauses = searchConditions(working);
  const flat = flattenTags(tags);
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
        : key === "keyword"
          ? t("searchConditions")
          : fields.some(([id]) => id === key)
            ? t(fields.find(([id]) => id === key)![1])
            : key;
  const operator = (key: Field) =>
    key === "name_contains"
      ? t("contains")
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
    key.startsWith("name_") ? ["name_contains", "name_equals"] : [key];
  const describe = (c: SearchCondition) =>
    (c.values || [])
      .map((item) =>
        c.field === "kind"
          ? t(item as (typeof kinds)[number])
          : c.field === "box"
            ? boxes?.find((box) => box.id === item)?.name || item
            : c.field.startsWith("tag_")
              ? flat.map(tagOption).find((tag) => tag.value === item)?.label ||
                item
              : item,
      )
      .join(", ");
  const change = (patch: Partial<FilterValues>) =>
    onChange({ ...value, ...patch });
  const edit = (index: number, nextStage: typeof stage) => {
    const c = clauses[index];
    setEditing(index);
    setField(c.field as Field);
    setStage(nextStage);
    setDraft(
      nextStage === "value" && scalar(c.field as Field)
        ? c.values?.[0] || ""
        : "",
    );
    combo.openDropdown();
    restoreFocus.current = false;
  };
  const nextFilters = (c?: SearchCondition) => {
    const next = [...clauses];
    if (c) {
      if (editing === null) next.push(c);
      else next[editing] = c;
    }
    return { ...working, condition: next.map((item) => JSON.stringify(item)) };
  };
  const invalid =
    (field === "from" || field === "to") &&
    !!draft.trim() &&
    !validDate(draft.trim());
  const commitText = (execute: boolean) => {
    if (
      invalid ||
      (!draft.trim() && field !== "keyword") ||
      (draft.trim() &&
        (!scalar(field) || (field !== "keyword" && stage !== "value")))
    )
      return;
    const next = nextFilters(
      draft.trim() ? { field, values: [draft.trim()] } : undefined,
    );
    setWorking(next);
    reset();
    restoreFocus.current = execute;
    if (execute) {
      onChange(next);
      combo.closeDropdown();
    } else combo.openDropdown();
  };
  const submit = (option: string) => {
    if (option.startsWith("field:")) {
      const next = option.slice(6) as Field;
      setField(next);
      setDraft("");
      setStage(operators(next).length > 1 ? "operator" : "value");
    } else if (option.startsWith("operator:")) {
      setField(option.slice(9) as Field);
      setStage("value");
      setDraft(
        editing !== null && scalar(field)
          ? clauses[editing].values?.[0] || ""
          : "",
      );
    } else if (option === "apply") {
      commitText(false);
      return;
    } else if (option.startsWith("value:")) {
      const item = option.slice(6);
      // Existing union groups remain editable as groups; new conditions are atomic.
      const old = editing !== null ? clauses[editing] : undefined;
      const values =
        old && old.field === field && (old.values?.length || 0) > 1
          ? old.values!.includes(item)
            ? old.values!.filter((v) => v !== item)
            : [...old.values!, item]
          : [item];
      if (values.length) setWorking(nextFilters({ field, values }));
      reset();
    }
    combo.resetSelectedOption();
    combo.openDropdown();
    restoreFocus.current = false;
  };
  const remove = (index: number) => {
    setWorking({
      ...working,
      condition: clauses
        .filter((_, i) => i !== index)
        .map((c) => JSON.stringify(c)),
    });
    reset();
    restoreFocus.current = false;
  };
  const needle = draft.trim().toLocaleLowerCase();
  const matches = (text: string) => text.toLocaleLowerCase().includes(needle);
  const options =
    field === "kind"
      ? kinds.map((kind) => ({ value: kind, label: t(kind) }))
      : field === "box"
        ? (boxes || []).map((box) => ({ value: box.id, label: box.name }))
        : flat.map(tagOption);
  const editor = (
    <div className="filter-inline-editor">
      {field !== "keyword" && (
        <span className="filter-token filter-editor-prefix">
          <span className="filter-token-segment">{title(field)}</span>
          {stage === "value" && (
            <span className="filter-token-segment">{operator(field)}</span>
          )}
        </span>
      )}
      <Combobox.DropdownTarget>
        <Combobox.EventsTarget withExpandedAttribute>
          <input
            ref={input}
            role="combobox"
            aria-label={
              field === "keyword" ? t("searchConditions") : title(field)
            }
            placeholder={
              field === "keyword"
                ? t("searchConditions")
                : t(stage === "operator" ? "chooseOperator" : "filterValue")
            }
            value={draft}
            maxLength={4096}
            onFocus={() => {
              if (!suppressFocusDropdown.current) combo.openDropdown();
            }}
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
                restoreFocus.current = true;
              }
              if (event.key === "Backspace" && !draft) {
                if (field !== "keyword") reset();
                else if (clauses.length) edit(clauses.length - 1, "value");
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
      </Combobox.DropdownTarget>
    </div>
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
              {clauses.map((c, index) =>
                editing === index ? (
                  <div className="filter-edit-slot" key={index}>
                    {editor}
                  </div>
                ) : (
                  <div className="filter-token" key={index}>
                    {c.field !== "keyword" && (
                      <>
                        <UnstyledButton
                          className="filter-token-segment"
                          onClick={() => edit(index, "field")}
                        >
                          {title(c.field as Field)}
                        </UnstyledButton>
                        <UnstyledButton
                          className="filter-token-segment filter-token-operator"
                          aria-label={`${t("operator")}: ${title(c.field as Field)}`}
                          onClick={() => edit(index, "operator")}
                        >
                          {operator(c.field as Field)}
                        </UnstyledButton>
                      </>
                    )}
                    <div className="filter-token-tail">
                      <UnstyledButton
                        className="filter-token-segment filter-token-value"
                        aria-label={
                          c.field === "keyword"
                            ? describe(c)
                            : `${title(c.field as Field)}: ${describe(c)}`
                        }
                        onClick={() => edit(index, "value")}
                      >
                        {describe(c)}
                      </UnstyledButton>
                      <ActionIcon
                        className="filter-token-remove"
                        size="xs"
                        variant="transparent"
                        color="gray"
                        aria-label={`${t("remove")}: ${c.field === "keyword" ? "" : `${title(c.field as Field)}: `}${describe(c)}`}
                        onClick={() => remove(index)}
                      >
                        <svg
                          width="14"
                          height="14"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2.5"
                          strokeLinecap="round"
                          aria-hidden="true"
                        >
                          <path d="m6 6 12 12M18 6 6 18" />
                        </svg>
                      </ActionIcon>
                    </div>
                  </div>
                ),
              )}
              {editing === null && editor}
            </div>
            {(clauses.length > 0 || !!draft) && (
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
                  restoreFocus.current = false;
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
              {stage === "field" || field === "keyword" ? (
                <>
                  {!!draft.trim() && (
                    <Combobox.Option value="apply">
                      <span className="filter-option-value">
                        <svg
                          width="18"
                          height="18"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="1.7"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          aria-hidden="true"
                        >
                          <path d="M3 6V4h12v2M9 4v16M6 20h6M15 12h6M15 16h6" />
                        </svg>
                        {t("searchText")}
                      </span>
                    </Combobox.Option>
                  )}
                  {visibleFields
                    .filter(([key]) => matches(title(key)) || matches(key))
                    .map(([key]) => (
                      <Combobox.Option key={key} value={`field:${key}`}>
                        {title(key)}
                      </Combobox.Option>
                    ))}
                </>
              ) : stage === "operator" ? (
                operators(field).map((key) => (
                  <Combobox.Option value={`operator:${key}`} key={key}>
                    {operator(key)}
                  </Combobox.Option>
                ))
              ) : scalar(field) ? (
                <>
                  {(field === "from" || field === "to") && (
                    <Combobox.Empty>
                      {t(entity === "boxes" ? "boxDateHint" : "filterDateHint")}
                    </Combobox.Empty>
                  )}
                  {invalid && <p role="alert">{t("invalidDate")}</p>}
                  {draft.trim() ? (
                    <Combobox.Option value="apply" disabled={invalid}>
                      {draft.trim()}
                    </Combobox.Option>
                  ) : (
                    <Combobox.Empty>{t("filterValue")}</Combobox.Empty>
                  )}
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
                      return (
                        <Combobox.Option
                          key={option.value}
                          value={`value:${option.value}`}
                        >
                          <span className="filter-option-value">
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
              className="filter-toolbar-button"
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
