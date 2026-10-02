import { useId } from "react";
import type { FileQuery, Tag } from "./api";
import { flattenTags } from "./Tags";
import { useI18n } from "./i18n";
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
        <label htmlFor={`${id}-name`}>{t("search")}</label>
        <input
          id={`${id}-name`}
          type="search"
          value={value.name || ""}
          onChange={(e) => change({ name: e.target.value })}
        />
        <label htmlFor={`${id}-kind`}>{t("kind")}</label>
        <select
          id={`${id}-kind`}
          value={value.kind?.[0] || ""}
          onChange={(e) =>
            change({ kind: e.target.value ? [e.target.value] : [] })
          }
        >
          <option value="">{t("allTypes")}</option>
          {(["image", "video", "audio", "document", "other"] as const).map(
            (kind) => (
              <option key={kind} value={kind}>
                {t(kind)}
              </option>
            ),
          )}
        </select>
        <label htmlFor={`${id}-sort`}>{t("sort")}</label>
        <select
          id={`${id}-sort`}
          value={value.sort}
          onChange={(e) => change({ sort: e.target.value as Filters["sort"] })}
        >
          <option value="captured">{t("captured")}</option>
          <option value="imported">{t("imported")}</option>
          <option value="name">{t("filename")}</option>
        </select>
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={!!value.asc}
            onChange={(e) => change({ asc: e.target.checked })}
          />
          {t("ascending")}
        </label>
        <div className="field-pair">
          <div>
            <label htmlFor={`${id}-from`}>{t("from")}</label>
            <input
              id={`${id}-from`}
              value={value.from || ""}
              onChange={(e) => change({ from: e.target.value })}
              placeholder="2026"
            />
          </div>
          <div>
            <label htmlFor={`${id}-to`}>{t("to")}</label>
            <input
              id={`${id}-to`}
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
          <div key={field}>
            <label htmlFor={`${id}-${field}`}>{t(label)}</label>
            <select
              multiple
              size={Math.min(4, Math.max(2, flat.length))}
              id={`${id}-${field}`}
              value={value[field] || []}
              onChange={(e) =>
                change({
                  [field]: Array.from(
                    e.target.selectedOptions,
                    (option) => option.value,
                  ),
                })
              }
            >
              {flat.map((tag) => (
                <option key={tag.id} value={tag.id}>
                  {"　".repeat(tag.depth)}
                  {tag.name}
                </option>
              ))}
            </select>
          </div>
        ))}
        <button onClick={() => onChange({ ...emptyFilters })}>
          {t("clearFilters")}
        </button>
      </div>
    </details>
  );
}
