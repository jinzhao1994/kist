import type { Filters, BoxFilters } from "./Filters";
import { useEffect, useState } from "react";
export interface Route {
  files: boolean;
  filters?: Filters;
  boxFilters?: BoxFilters;
  box?: string;
  folder?: string;
  tag?: string;
  file?: string;
  section?: "history" | "trash";
}
function readFilters(
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
    for (const key of ["name", "from", "to"] as const)
      if (typeof input[key] === "string") out[key] = input[key];
    for (const key of ["box", "kind", "tag_all", "tag_any", "tag_not"] as const)
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
function read(): Route {
  const url = new URL(window.location.href);
  const match = url.pathname.match(/^\/boxes\/([^/]+)$/);
  const tagMatch = url.pathname.match(/^\/tags\/([^/]+)$/);
  let tag: string | undefined;
  try {
    tag = tagMatch ? decodeURIComponent(tagMatch[1]) : undefined;
  } catch {
    tag = tagMatch?.[1];
  }
  let box: string | undefined;
  try {
    box = match ? decodeURIComponent(match[1]) : undefined;
  } catch {
    box = match?.[1];
  }
  return {
    folder: match ? url.searchParams.get("folder") || undefined : undefined,
    filters: readFilters(url.searchParams.get("filter")) as Filters | undefined,
    boxFilters: readFilters(url.searchParams.get("box_filter"), true) as
      BoxFilters | undefined,
    tag,
    files: url.pathname === "/files" || !!match,
    box: box || url.searchParams.get("box") || undefined,
    section:
      url.pathname === "/history"
        ? "history"
        : url.pathname === "/trash"
          ? "trash"
          : undefined,
    file: url.searchParams.get("file") || undefined,
  };
}
export function useNavigation() {
  const [route, setRoute] = useState(read);
  useEffect(() => {
    const change = () => setRoute(read());
    window.addEventListener("popstate", change);
    return () => window.removeEventListener("popstate", change);
  }, []);
  return {
    route,
    go: (next: Route, replace = false) => {
      const path = next.section
        ? `/${next.section}`
        : next.tag
          ? `/tags/${encodeURIComponent(next.tag)}`
          : next.box
            ? `/boxes/${encodeURIComponent(next.box)}`
            : next.files
              ? "/files"
              : "/";
      const params = new URLSearchParams();
      if (next.box && next.folder && !next.section)
        params.set("folder", next.folder);
      if (next.file) params.set("file", next.file);
      if (next.section && next.box) params.set("box", next.box);
      if (next.boxFilters)
        params.set("box_filter", JSON.stringify(next.boxFilters));
      if (next.filters) params.set("filter", JSON.stringify(next.filters));
      const opening = !!next.file && !route.file && !next.section;
      const state = replace
        ? history.state
        : opening
          ? { kistGallery: true }
          : null;
      history[replace ? "replaceState" : "pushState"](
        state,
        "",
        path + (params.size ? `?${params}` : ""),
      );
      setRoute(next);
    },
  };
}
