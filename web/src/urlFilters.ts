import type { Filters, BoxFilters } from "./Filters";
function readLegacyFilters(
  raw: string | null,
  boxes = false,
): Filters | BoxFilters | undefined {
  if (!raw) return;
  try {
    const input = JSON.parse(raw);
    if (!input || typeof input !== "object" || Array.isArray(input)) return;
    const out: Omit<Filters, "sort"> & {
      sort?: Filters["sort"] | BoxFilters["sort"];
    } = {};
    for (const key of [
      "name",
      "name_contains",
      "name_equals",
      "from",
      "to",
    ] as const)
      if (typeof input[key] === "string") out[key] = input[key];
    for (const key of [
      "keyword",
      "condition",
      "box",
      "kind",
      "tag_all",
      "tag_any",
      "tag_not",
    ] as const)
      if (
        Array.isArray(input[key]) &&
        input[key].every((v: unknown) => typeof v === "string")
      )
        out[key] = input[key];
    if (
      (boxes
        ? ["updated", "activity", "name"]
        : ["captured", "imported", "name", "updated", "type", "size"]
      ).includes(input.sort)
    )
      out.sort = input.sort;
    if (typeof input.asc === "boolean") out.asc = input.asc;
    return out;
  } catch {
    return;
  }
}

const fields = [
  "keyword",
  "name_contains",
  "name_equals",
  "from",
  "to",
  "box",
  "kind",
  "tag_all",
  "tag_any",
  "tag_not",
];
const parameter = (field: string) =>
  field === "keyword" ? "q" : field === "box" ? "in_box" : field;

export function writeURLFilters(
  params: URLSearchParams,
  value: Filters | BoxFilters | undefined,
  boxes = false,
) {
  if (!value) return;
  const prefix = boxes ? "b." : "";
  const appendCondition = (raw: string) => {
    try {
      const clause = JSON.parse(raw);
      if (
        clause &&
        Object.keys(clause).length === 2 &&
        fields.includes(clause.field) &&
        Array.isArray(clause.values) &&
        clause.values.length === 1 &&
        typeof clause.values[0] === "string"
      ) {
        params.append(prefix + parameter(clause.field), clause.values[0]);
        return;
      }
    } catch {
      /* Preserve malformed clauses for existing server validation. */
    }
    params.append(prefix + "c", raw);
  };
  for (const raw of value.condition || []) appendCondition(raw);
  const clause = (field: string, values: string[]) =>
    appendCondition(JSON.stringify({ field, values }));
  for (const term of [
    ...(value.keyword || []),
    ...(value.name ? [value.name] : []),
  ])
    clause("keyword", [term]);
  for (const field of ["name_contains", "name_equals", "from", "to"] as const)
    if (value[field]) clause(field, [value[field]]);
  for (const tag of value.tag_all || []) clause("tag_all", [tag]);
  if ("box" in value && value.box?.length) clause("box", value.box);
  for (const field of ["kind", "tag_any", "tag_not"] as const)
    if (value[field]?.length) clause(field, value[field]);
  if (value.sort && value.sort !== "updated")
    params.set(prefix + "sort", value.sort);
  if (!boxes && "include_members" in value && value.include_members)
    params.set("include_members", "1");
  if (value.asc) params.set(prefix + "asc", "1");
}

export function readURLFilters(
  params: URLSearchParams,
  boxes = false,
): Filters | BoxFilters | undefined {
  const prefix = boxes ? "b." : "";
  const out =
    readLegacyFilters(params.get(boxes ? "box_filter" : "filter"), boxes) || {};
  const conditions = [...(out.condition || [])];
  for (const [key, value] of params) {
    if (key === prefix + "c") conditions.push(value);
    else {
      const field = fields.find((field) => key === prefix + parameter(field));
      if (field) conditions.push(JSON.stringify({ field, values: [value] }));
    }
  }
  if (conditions.length) out.condition = conditions;
  const sort = params.get(prefix + "sort");
  const validated = readLegacyFilters(JSON.stringify({ sort }), boxes);
  if (validated?.sort) out.sort = validated.sort;
  if (!boxes && params.has("include_members"))
    (out as Filters).include_members = params.get("include_members") === "1";
  const asc = params.get(prefix + "asc");
  if (asc === "1" || asc === "0") out.asc = asc === "1";
  return Object.keys(out).length ? out : undefined;
}
