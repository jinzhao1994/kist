import {
  TextInput,
  Button,
  MultiSelect,
  Popover,
  ActionIcon,
  UnstyledButton,
} from "@mantine/core";
import { useState } from "react";
import type { FileQuery, Tag } from "./api";
import { flattenTags } from "./Tags";
import { useI18n } from "./i18n";
import { Select } from "./Select";
export type Filters = Omit<FileQuery, "box" | "folder" | "cursor" | "limit">;
export const emptyFilters: Filters = {
  name: "",
  sort: "captured",
  asc: false,
  from: "",
  to: "",
  kind: [],
  tag_all: [],
  tag_any: [],
  tag_not: [],
};
type Field = "kind" | "from" | "to" | "tag_all" | "tag_any" | "tag_not";
const fields = [
  ["kind", "kind"],
  ["from", "from"],
  ["to", "to"],
  ["tag_all", "tagAll"],
  ["tag_any", "tagAny"],
  ["tag_not", "tagNot"],
] as const;
export function FiltersPanel({
  value,
  onChange,
  tags,
}: {
  value: Filters;
  onChange: (value: Filters) => void;
  tags: Tag[];
}) {
  const { t } = useI18n();
  const [opened, setOpened] = useState(false);
  const [field, setField] = useState<Field | null>(null);
  const [date, setDate] = useState("");
  const flat = flattenTags(tags);
  const change = (patch: Partial<Filters>) => onChange({ ...value, ...patch });
  const edit = (next: Field | null) => {
    setField(next);
    setDate(next === "from" || next === "to" ? value[next] || "" : "");
    setOpened(true);
  };
  const active = fields.filter(([key]) => !!value[key]?.length);
  const label = fields.find(([key]) => key === field)?.[1];
  const describe = (key: Field) => {
    const selected = value[key];
    if (!Array.isArray(selected)) return selected || "";
    return selected
      .map((item) =>
        key === "kind"
          ? t(item as "image" | "video" | "audio" | "document" | "other")
          : flat.find((tag) => tag.id === item)?.name || item,
      )
      .join(", ");
  };
  return (
    <section className="filters-bar" aria-label={t("filters")}>
      <div className="filter-search">
        <TextInput
          className="filter-search-input"
          aria-label={t("search")}
          placeholder={t("search")}
          type="search"
          value={value.name || ""}
          onChange={(event) => change({ name: event.currentTarget.value })}
        />
        <Popover
          opened={opened}
          onChange={setOpened}
          position="bottom-start"
          width={320}
          trapFocus
          returnFocus
        >
          <Popover.Target>
            <Button
              variant="light"
              onClick={() => (opened ? setOpened(false) : edit(null))}
              aria-expanded={opened}
            >
              {t("addFilter")}
            </Button>
          </Popover.Target>
          <Popover.Dropdown className="filter-editor">
            <div className="filter-editor-heading">
              <strong>{label ? t(label) : t("addFilter")}</strong>
              <ActionIcon
                variant="subtle"
                aria-label={t("close")}
                onClick={() => setOpened(false)}
              >
                ×
              </ActionIcon>
            </div>
            {field === null ? (
              <div className="filter-options">
                {fields.map(([key, title]) => (
                  <Button key={key} variant="subtle" onClick={() => edit(key)}>
                    {t(title)}
                  </Button>
                ))}
              </div>
            ) : field === "from" || field === "to" ? (
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  change({ [field]: date.trim() });
                  setOpened(false);
                }}
              >
                <TextInput
                  label={t(field)}
                  value={date}
                  onChange={(event) => setDate(event.currentTarget.value)}
                  placeholder="2026-10"
                  description={t("filterDateHint")}
                  autoFocus
                />
                <Button type="submit" mt="sm">
                  {t("applyFilter")}
                </Button>
              </form>
            ) : (
              <MultiSelect
                label={label ? t(label) : undefined}
                value={value[field] || []}
                onChange={(values) => change({ [field]: values })}
                data={
                  field === "kind"
                    ? (
                        [
                          "image",
                          "video",
                          "audio",
                          "document",
                          "other",
                        ] as const
                      ).map((kind) => ({ value: kind, label: t(kind) }))
                    : flat.map((tag) => ({
                        value: tag.id,
                        label: `${"　".repeat(tag.depth)}${tag.name}`,
                      }))
                }
                searchable
                clearable
                comboboxProps={{ withinPortal: false }}
                nothingFoundMessage={t("noFilterOptions")}
              />
            )}
          </Popover.Dropdown>
        </Popover>
      </div>
      <div className="filter-sort">
        <Select
          label={t("sort")}
          value={value.sort || "captured"}
          onValueChange={(sort) => change({ sort: sort as Filters["sort"] })}
          options={[
            { value: "captured", label: t("captured") },
            { value: "imported", label: t("imported") },
            { value: "name", label: t("filename") },
          ]}
        />
        <ActionIcon
          size="lg"
          variant="default"
          aria-label={t("ascending")}
          aria-pressed={!!value.asc}
          title={t(value.asc ? "ascending" : "descending")}
          onClick={() => change({ asc: !value.asc })}
        >
          {value.asc ? "↑" : "↓"}
        </ActionIcon>
      </div>
      {(active.length > 0 || value.name) && (
        <div className="filter-tokens">
          {active.map(([key, title]) => (
            <div className="filter-token" key={key}>
              <UnstyledButton
                onClick={() => edit(key)}
                className="filter-token-label"
              >
                {t(title)}: {describe(key)}
              </UnstyledButton>
              <ActionIcon
                size="sm"
                variant="subtle"
                aria-label={`${t("remove")}: ${t(title)}`}
                onClick={() =>
                  change({ [key]: key === "from" || key === "to" ? "" : [] })
                }
              >
                ×
              </ActionIcon>
            </div>
          ))}
          <Button
            variant="subtle"
            size="compact-sm"
            onClick={() => {
              onChange({ ...emptyFilters, sort: value.sort, asc: value.asc });
              setOpened(false);
            }}
          >
            {t("clearFilters")}
          </Button>
        </div>
      )}
    </section>
  );
}
