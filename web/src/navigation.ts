import type { Filters, BoxFilters } from "./Filters";
import { readURLFilters, writeURLFilters } from "./urlFilters";
import { useEffect, useState } from "react";
export interface Route {
  files: boolean;
  filters?: Filters;
  boxFilters?: BoxFilters;
  box?: string;
  folder?: string;
  tag?: string;
  file?: string;
  fileset?: string;
  page?: string;
  section?: "history" | "trash" | "sync" | "settings";
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
    filters: readURLFilters(url.searchParams) as Filters | undefined,
    boxFilters: readURLFilters(url.searchParams, true) as
      BoxFilters | undefined,
    tag,
    files: url.pathname === "/files" || !!match,
    box: box || url.searchParams.get("box") || undefined,
    section:
      url.pathname === "/history"
        ? "history"
        : url.pathname === "/trash"
          ? "trash"
          : url.pathname === "/sync"
            ? "sync"
            : url.pathname === "/settings"
              ? "settings"
              : undefined,
    file: url.searchParams.get("file") || undefined,
    page: url.searchParams.get("page") || undefined,
    fileset: url.searchParams.get("fileset") || undefined,
  };
}
export function routeURL(next: Route, anchor?: string): string {
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
  if (next.file && next.page) params.set("page", next.page);
  if (next.section && next.fileset) params.set("fileset", next.fileset);
  if (next.section && next.box) params.set("box", next.box);
  writeURLFilters(params, next.boxFilters, true);
  writeURLFilters(params, next.filters);
  return (
    path +
    (params.size ? `?${params}` : "") +
    (anchor ? `#${encodeURIComponent(anchor)}` : "")
  );
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
    go: (next: Route, replace = false, anchor?: string) => {
      const opening = !!next.file && !route.file && !next.section;
      const state = replace
        ? history.state
        : opening
          ? { kistGallery: true }
          : null;
      history[replace ? "replaceState" : "pushState"](
        state,
        "",
        routeURL(next, anchor),
      );
      setRoute(next);
    },
  };
}
