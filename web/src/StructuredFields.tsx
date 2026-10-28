import type { FileDetail } from "./api";
import { MetadataLabel } from "./MetadataLabel";
import { MetadataValue } from "./MetadataValue";
import { CopyTextPreview } from "./CopyTextPreview";
import { useI18n } from "./i18n";

type Row = NonNullable<FileDetail["metadata"]>[number];
export type FactField = {
  key: string;
  label: string;
  type: string;
  value?: string;
  unit?: string;
  uncertain?: boolean;
  source: "ai" | "user";
  deleted?: boolean;
  children?: FactField[];
};

// The server validates writes. Rendering still bounds depth/count and rejects
// unknown document versions rather than displaying or editing raw JSON.
function readFields(raw: string): FactField[] {
  const document = JSON.parse(raw);
  let count = 0;
  const validate = (fields: FactField[], depth: number) => {
    if (!Array.isArray(fields) || depth > 8) throw new Error("invalid fields");
    const keys = new Set<string>();
    for (const f of fields) {
      if (
        ++count > 512 ||
        !f ||
        typeof f.label !== "string" ||
        typeof f.key !== "string" ||
        !/^[a-z][a-z0-9_]{0,63}$/u.test(f.key) ||
        keys.has(f.key) ||
        !["ai", "user"].includes(f.source) ||
        (f.value !== undefined && typeof f.value !== "string")
      )
        throw new Error("invalid field");
      keys.add(f.key);
      if (f.children) validate(f.children, depth + 1);
    }
  };
  if (document.version !== 1) throw new Error("unknown version");
  validate(document.fields, 1);
  return document.fields;
}

export function StructuredFields({
  fileID,
  blobKey,
  row,
}: {
  fileID: string;
  blobKey: string;
  row: Row;
}) {
  const { t } = useI18n();
  let fields: FactField[];
  try {
    fields = readFields(row.value);
  } catch {
    return (
      <span role="alert" className="metadata-help">
        {t("factsInvalid")}
      </span>
    );
  }
  const render = (nodes: FactField[], parent = ""): React.ReactNode => {
    const visible = nodes.filter((f) => !f.deleted);
    return visible.map((f) => {
      const path = `${parent}/${f.key}`;
      const container = f.type === "object" || f.type === "array";
      const node: Row = {
        ...row,
        key: path,
        value: f.value || "",
        source: f.source,
      };
      const control = (children: React.ReactNode) => (
        <MetadataValue
          fileID={fileID}
          blobKey={blobKey}
          row={node}
          label={f.label}
          binding={{ row, path }}
          editable={!container}
        >
          {children}
        </MetadataValue>
      );
      if (container)
        return (
          <section key={path}>
            <div className="metadata-group-heading">
              {control(
                <h4>
                  <bdi>{f.label}</bdi>
                </h4>,
              )}
            </div>
            {render(f.children || [], path)}
          </section>
        );
      return (
        <dl key={path} className="metadata-list">
          <div>
            <MetadataLabel
              metadataKey={`facts:data${path}`}
              label={f.label}
              help={f.uncertain ? t("factsUncertainHelp") : undefined}
            />
            <dd>
              {control(
                <>
                  {f.value ? (
                    <CopyTextPreview text={f.value} copyLabel="copy" />
                  ) : (
                    <span className="metadata-help">
                      {t("factsUnreadable")}
                    </span>
                  )}
                  {(f.unit || f.uncertain) && (
                    <span className="metadata-help">
                      {[f.unit, f.uncertain ? t("factsUncertain") : ""]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                  )}
                </>,
              )}
            </dd>
          </div>
        </dl>
      );
    });
  };
  return (
    <div className="metadata-field-tree">
      {fields.some((f) => !f.deleted) ? (
        render(fields)
      ) : (
        <span className="metadata-help">{t("factsEmpty")}</span>
      )}
    </div>
  );
}
