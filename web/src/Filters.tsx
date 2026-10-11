import { TextInput, Button, Checkbox, MultiSelect } from "@mantine/core";
import { useId } from "react";
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
  const id = useId();
  const flat = flattenTags(tags);
  const change = (patch: Partial<Filters>) => onChange({ ...value, ...patch });
  return (
    <details className="filters-panel" open>
      <summary>{t("filters")}</summary>
      <div className="filter-fields">
        <TextInput
          id={`${id}-name`}
          label={t("search")}
          type="search"
          value={value.name || ""}
          onChange={(e) => change({ name: e.target.value })}
        />
        <Select
          id={`${id}-kind`}
          label={t("kind")}
          value={value.kind?.[0] || ""}
          onValueChange={(kind) => change({ kind: kind ? [kind] : [] })}
          options={[
            { value: "", label: t("allTypes") },
            ...(["image", "video", "audio", "document", "other"] as const).map(
              (kind) => ({ value: kind, label: t(kind) }),
            ),
          ]}
        />
        <Select
          id={`${id}-sort`}
          label={t("sort")}
          value={value.sort || "captured"}
          onValueChange={(sort) => change({ sort: sort as Filters["sort"] })}
          options={[
            { value: "captured", label: t("captured") },
            { value: "imported", label: t("imported") },
            { value: "name", label: t("filename") },
          ]}
        />
        <Checkbox
          label={t("ascending")}
          checked={!!value.asc}
          onChange={(e) => change({ asc: e.currentTarget.checked })}
        />
        <div className="field-pair">
          <div>
            <TextInput
              id={`${id}-from`}
              label={t("from")}
              value={value.from || ""}
              onChange={(e) => change({ from: e.target.value })}
              placeholder="2026"
            />
          </div>
          <div>
            <TextInput
              id={`${id}-to`}
              label={t("to")}
              value={value.to || ""}
              onChange={(e) => change({ to: e.target.value })}
              placeholder="2026-12"
            />
          </div>
        </div>
        {(
          [
            ["tag_all", "tagAll"],
            ["tag_any", "tagAny"],
            ["tag_not", "tagNot"],
          ] as const
        ).map(([field, label]) => (
          <MultiSelect
            key={field}
            label={t(label)}
            id={`${id}-${field}`}
            value={value[field] || []}
            onChange={(values) => change({ [field]: values })}
            data={flat.map((tag) => ({
              value: tag.id,
              label: `${"　".repeat(tag.depth)}${tag.name}`,
            }))}
            searchable
            clearable
          />
        ))}
        <Button onClick={() => onChange({ ...emptyFilters })}>
          {t("clearFilters")}
        </Button>
      </div>
    </details>
  );
}
