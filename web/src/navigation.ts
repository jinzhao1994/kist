import type { Filters, BoxFilters } from "./Filters";
import { readURLFilters, writeURLFilters } from "./urlFilters";
import { useEffect, useState, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get } from "./api";
import type { components } from "./api-schema";
export interface Route {
  files: boolean;
  permanent?: boolean;
  identityOnly?: boolean;
  path?: string;
  readablePath?: string;
  view?: string;
  scope?: { box?: string; folder?: string; tag?: string };

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
export function parseRoute(href: string): Route {
  const url = new URL(href, window.location.origin);
  const match =
    url.pathname.match(/^\/boxes\/id\/([^/]+)$/) ||
    url.pathname.match(/^\/boxes\/([0-9a-f]{8}-[0-9a-f-]{27})$/i);
  const readable = !match && url.pathname.startsWith("/boxes/");
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
    folder: url.searchParams.get("folder") || undefined,
    permanent:
      !readable &&
      !!(
        match ||
        url.searchParams.get("file") ||
        url.searchParams.get("folder")
      ),
    identityOnly: url.pathname === "/files",
    path: readable ? url.pathname : undefined,
    view: url.searchParams.get("view") || undefined,
    scope: history.state?.kistScope,
    filters: readURLFilters(url.searchParams) as Filters | undefined,
    boxFilters: readURLFilters(url.searchParams, true) as
      BoxFilters | undefined,
    tag,
    files: url.pathname === "/files" || !!match || readable,
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
function read(): Route {
  return parseRoute(window.location.href);
}
export function routeURL(next: Route, anchor?: string): string {
  const readable = !next.section && !next.permanent && next.readablePath;
  const path =
    readable ||
    (next.section
      ? `/${next.section}`
      : next.identityOnly && (next.file || next.folder)
        ? "/files"
        : next.tag
          ? `/tags/${encodeURIComponent(next.tag)}`
          : next.box
            ? `/boxes/id/${encodeURIComponent(next.box)}`
            : next.files
              ? "/files"
              : "/");
  const params = new URLSearchParams();
  if (!readable && next.folder && !next.section)
    params.set("folder", next.folder);
  if (!readable && next.file) params.set("file", next.file);
  if (readable && next.view) params.set("view", next.view);
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
    go: (requested: Route, replace = false, anchor?: string) => {
      const next = { ...requested };
      if (!next.file) next.scope = undefined;
      const opening = !!next.file && !route.file && !next.section;
      const state = replace
        ? history.state
        : opening
          ? {
              kistGallery: true,
              kistScope: {
                box: route.box,
                folder: route.folder,
                tag: route.tag,
              },
            }
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

export type NavigationLocation = components["schemas"]["NavigationLocation"];
export function navigationQuery(route: Route) {
  const params = new URLSearchParams();
  if (route.path) {
    params.set("path", route.path);
    if (route.view) params.set("view", route.view);
  } else {
    if (route.box) params.set("box", route.box);
    if (route.folder) params.set("folder", route.folder);
    if (route.file) params.set("file", route.file);
  }
  return `/navigation?${params}`;
}
export function permanentURL(route: Route) {
  return routeURL({
    ...route,
    permanent: true,
    identityOnly: !!(route.file || route.folder),
    box: route.file || route.folder ? undefined : route.box,
    folder: route.file ? undefined : route.folder,
    readablePath: undefined,
    path: undefined,
  });
}

// Resolve current names from the server's logical catalog; no browser slug cache
// or disk projection is authoritative. Permanent addresses retain their mode.
export function useResolvedNavigation() {
  const nav = useNavigation();
  const navRef = useRef(nav);
  navRef.current = nav;
  const queries = useQueryClient();
  const queue = useRef(Promise.resolve());
  const generation = useRef(0);
  const retryTransition = useRef<(() => void) | null>(null);
  const [transitionError, setTransitionError] = useState<Error | null>(null);
  useEffect(() => {
    const cancel = () => {
      generation.current++;
      retryTransition.current = null;
      setTransitionError(null);
    };
    window.addEventListener("popstate", cancel);
    return () => {
      generation.current++;
      window.removeEventListener("popstate", cancel);
    };
  }, []);
  const raw = nav.route;
  const enabled =
    !raw.section && !!(raw.path || raw.box || raw.folder || raw.file);
  const query = useQuery({
    queryKey: ["navigation", navigationQuery(raw)],
    queryFn: ({ signal }) =>
      get<NavigationLocation>(navigationQuery(raw), signal),
    enabled,
    retry: false,
  });
  const location = enabled ? query.data : undefined;
  const route: Route = location
    ? {
        ...raw,
        box: raw.file && !raw.path ? raw.box : location.box,
        folder: raw.file && !raw.path ? raw.folder : location.folder,
        file: location.file,
        readablePath: location.path,
        view: location.view,
      }
    : raw;
  if (route.file && raw.scope) Object.assign(route, raw.scope);
  useEffect(() => {
    if (!location) return;
    const next = { ...route, path: undefined };
    const url = routeURL(next) + window.location.hash;
    if (
      raw.path ||
      url !==
        window.location.pathname + window.location.search + window.location.hash
    ) {
      // The resolved pathname is already encoded. Preserve the gallery history
      // entry while replacing its transient identity-based address.
      if (raw.path) nav.go(next, true);
      history.replaceState(history.state, "", url);
    }
  });
  return {
    ...nav,
    go: function go(requested: Route, replace = false, anchor?: string) {
      retryTransition.current = null;
      setTransitionError(null);
      const same =
        requested.box === route.box &&
        requested.folder === route.folder &&
        requested.file === route.file &&
        !requested.path;
      if (
        requested.section ||
        !(
          requested.path ||
          requested.box ||
          requested.folder ||
          requested.file
        ) ||
        same
      ) {
        generation.current++;
        nav.go(
          {
            ...requested,
            readablePath: same ? route.readablePath : undefined,
            view: same ? route.view : undefined,
          },
          replace,
          anchor,
        );
        return;
      }
      const target = {
        ...requested,
        path: requested.path,
        readablePath: undefined,
        view: requested.path ? requested.view : undefined,
      };
      const epoch = generation.current;
      queue.current = queue.current.then(async () => {
        if (epoch !== generation.current) return;
        try {
          const queryPath = navigationQuery(target);
          const located = await queries.fetchQuery({
            queryKey: ["navigation", queryPath],
            queryFn: ({ signal }) => get<NavigationLocation>(queryPath, signal),
            staleTime: 0,
          });
          if (epoch !== generation.current) return;
          navRef.current.go(
            {
              ...target,
              path: undefined,
              box: target.file ? target.box : located.box,
              folder: target.file ? target.folder : located.folder,
              file: located.file,
              readablePath: located.path,
              view: located.view,
            },
            replace,
            anchor,
          );
        } catch (error) {
          if (epoch === generation.current) {
            generation.current++;
            retryTransition.current = () => go(requested, replace, anchor);
            setTransitionError(error as Error);
          }
        }
      });
    },
    route,
    pending:
      enabled && query.isPending && !!(raw.path || (raw.folder && !raw.box)),
    error: transitionError || (enabled ? query.error : null),
    retry: () => {
      if (retryTransition.current) retryTransition.current();
      else void query.refetch();
    },
  };
}
