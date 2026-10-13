import type { Filters } from "./Filters";
import { useEffect, useState } from "react";
export interface Route {
  files: boolean;
  filters?: Filters;
  box?: string;
  tag?: string;
  file?: string;
  section?: "history" | "trash";
}
function readFilters(raw: string | null): Filters | undefined {
  if (!raw) return;
  try {
    const input = JSON.parse(raw);
    if (!input || typeof input !== "object" || Array.isArray(input)) return;
    const out: Filters = {};
    for (const key of ["name", "from", "to"] as const)
      if (typeof input[key] === "string") out[key] = input[key];
    for (const key of ["kind", "tag_all", "tag_any", "tag_not"] as const)
      if (
        Array.isArray(input[key]) &&
        input[key].every((v: unknown) => typeof v === "string")
      )
        out[key] = input[key];
    if (["captured", "imported", "name"].includes(input.sort))
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
    filters: readFilters(url.searchParams.get("filter")),
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
      if (next.file) params.set("file", next.file);
      if (next.section && next.box) params.set("box", next.box);
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
