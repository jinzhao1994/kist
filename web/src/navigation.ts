import { useEffect, useState } from "react";
export interface Route {
  files: boolean;
  box?: string;
  file?: string;
}
function read(): Route {
  const url = new URL(window.location.href);
  const match = url.pathname.match(/^\/boxes\/([^/]+)$/);
  let box: string | undefined;
  try {
    box = match ? decodeURIComponent(match[1]) : undefined;
  } catch {
    box = match?.[1];
  }
  return {
    files: url.pathname === "/files" || !!match,
    box,
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
    go: (next: Route) => {
      const path = next.box
        ? `/boxes/${encodeURIComponent(next.box)}`
        : next.files
          ? "/files"
          : "/";
      history.pushState(
        null,
        "",
        path + (next.file ? `?file=${encodeURIComponent(next.file)}` : ""),
      );
      setRoute(next);
    },
  };
}
