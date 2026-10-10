import { useEffect, useState } from "react";
export interface Route {
  files: boolean;
  box?: string;
  file?: string;
  section?: "history" | "trash";
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
    go: (next: Route) => {
      const path = next.section
        ? `/${next.section}`
        : next.box
          ? `/boxes/${encodeURIComponent(next.box)}`
          : next.files
            ? "/files"
            : "/";
      const params = new URLSearchParams();
      if (next.file) params.set("file", next.file);
      if (next.section && next.box) params.set("box", next.box);
      history.pushState(null, "", path + (params.size ? `?${params}` : ""));
      setRoute(next);
    },
  };
}
