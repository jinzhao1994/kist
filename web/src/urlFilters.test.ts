import { describe, expect, it } from "vitest";
import { emptyFilters, searchConditions } from "./Filters";
import { readURLFilters, writeURLFilters } from "./urlFilters";

const condition = (field: string, values: string[]) =>
  JSON.stringify({ field, values });
describe("filter URLs", () => {
  it("omits empty filters and default sorting", () => {
    const params = new URLSearchParams({ file: "file-1" });
    writeURLFilters(params, emptyFilters);
    writeURLFilters(params, emptyFilters, true);
    expect(params.toString()).toBe("file=file-1");
    expect(readURLFilters(params)).toBeUndefined();
  });
  it("round trips Unicode and reserved characters without double encoding", () => {
    const params = new URLSearchParams();
    const term = "旅行 & 日志+#?=100%";
    writeURLFilters(params, {
      condition: [condition("keyword", [term])],
      sort: "name",
      asc: true,
    });
    expect(params.get("q")).toBe(term);
    expect(params.has("filter")).toBe(false);
    const result = readURLFilters(new URLSearchParams(params.toString()))!;
    expect(searchConditions(result)).toEqual([
      { field: "keyword", values: [term] },
    ]);
    expect(result).toMatchObject({ sort: "name", asc: true });
  });
  it("preserves repeated clause order and union groups", () => {
    const clauses = [
      condition("kind", ["image", "video"]),
      condition("keyword", ["a"]),
      condition("keyword", ["b"]),
      condition("tag_any", ["one", "two"]),
      condition("tag_any", ["three"]),
    ];
    const params = new URLSearchParams();
    writeURLFilters(params, { condition: clauses });
    expect(params.getAll("q")).toEqual(["a", "b"]);
    expect(readURLFilters(params)?.condition).toEqual(clauses);
  });
  it("separates box and file filters without conflicting with the route scope", () => {
    const params = new URLSearchParams({
      box: "scope",
      folder: "folder",
      file: "file",
    });
    writeURLFilters(params, { keyword: ["files"], box: ["one"], sort: "size" });
    writeURLFilters(
      params,
      { name: "boxes", sort: "activity", asc: true },
      true,
    );
    expect(params.get("box")).toBe("scope");
    expect(params.get("in_box")).toBe("one");
    expect(searchConditions(readURLFilters(params)!)).toEqual([
      { field: "keyword", values: ["files"] },
      { field: "box", values: ["one"] },
    ]);
    expect(readURLFilters(params, true)).toMatchObject({
      sort: "activity",
      asc: true,
    });
    expect(searchConditions(readURLFilters(params, true)!)).toEqual([
      { field: "keyword", values: ["boxes"] },
    ]);
  });
  it("reads old links and rewrites their filters without changing meaning", () => {
    const legacy = {
      ...emptyFilters,
      name: "old",
      tag_any: ["a", "b"],
      sort: "name" as const,
    };
    const params = new URLSearchParams({ filter: JSON.stringify(legacy) });
    const parsed = readURLFilters(params)!;
    const next = new URLSearchParams();
    writeURLFilters(next, parsed);
    expect(searchConditions(readURLFilters(next)!)).toEqual(
      searchConditions(legacy),
    );
    expect(next.has("filter")).toBe(false);
    expect(
      readURLFilters(new URLSearchParams("filter=broken&sort=bad&asc=bad")),
    ).toBeUndefined();
  });
  it("keeps malformed and extended clauses for server validation", () => {
    const clauses = ["broken", '{"field":"keyword","values":["a"],"extra":1}'];
    const params = new URLSearchParams();
    writeURLFilters(params, { condition: clauses });
    expect(readURLFilters(params)?.condition).toEqual(clauses);
  });
});
