import { act, renderHook, waitFor, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import {
  parseRoute,
  permanentURL,
  routeURL,
  useResolvedNavigation,
} from "./navigation";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  history.replaceState(null, "", "/");
});
function setup(href: string) {
  history.replaceState(null, "", href);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}
it("preserves legacy identity, filter, page and history parameters", () => {
  const id = "01a16a65-522a-7daf-960a-ad239a94a13d";
  const route = parseRoute(
    `/boxes/${id}?folder=parent&file=group&page=2&q=west`,
  );
  expect(route.permanent).toBe(true);
  const url = new URL(routeURL(route, "details"), location.origin);
  expect(url.pathname).toBe(`/boxes/id/${id}`);
  expect(url.searchParams.get("folder")).toBe("parent");
  expect(url.searchParams.get("file")).toBe("group");
  expect(url.searchParams.get("page")).toBe("2");
  expect(url.hash).toBe("#details");
});
it("keeps encoded segments intact and removes ancestor identities from permanent links", () => {
  const path = "/boxes/2026-10_trip/100%25%20%23%3F%252F.jpg";
  expect(parseRoute(path).path).toBe(path);
  expect(
    permanentURL({
      files: true,
      box: "old-box",
      folder: "old-folder",
      file: "file",
      page: "2",
    }),
  ).toBe("/files?file=file&page=2");
  expect(permanentURL({ files: true, box: "box", folder: "folder" })).toBe(
    "/files?folder=folder",
  );
  expect(permanentURL({ files: true, box: "box" })).toBe("/boxes/id/box");
});
it("resolves readable paths, preserves page state and follows a rename by identity", async () => {
  const path = "/boxes/2026-10_trip/document";
  const { wrapper, client } = setup(path + "?page=2");
  let current = path;
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: true,
      json: async () => ({
        box: "box",
        folder: "parent",
        file: "set",
        kind: "fileset",
        path: current,
      }),
    })),
  );
  const { result } = renderHook(useResolvedNavigation, { wrapper });
  await waitFor(() => expect(result.current.route.file).toBe("set"));
  await waitFor(() => expect(location.pathname).toBe(path));
  act(() => result.current.go({ ...result.current.route, page: "3" }, true));
  await waitFor(() => expect(location.search).toBe("?page=3"));
  current = "/boxes/2026-10_trip/renamed";
  await act(async () => {
    await client.invalidateQueries({ queryKey: ["navigation"] });
  });
  await waitFor(() => expect(location.pathname).toBe(current));
  expect(location.search).toBe("?page=3");
  act(() => {
    history.replaceState(history.state, "", permanentURL(result.current.route));
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
  await waitFor(() => expect(result.current.route.permanent).toBe(true));
  expect(location.pathname).toBe("/files");
  await act(async () => {
    await client.invalidateQueries({ queryKey: ["navigation"] });
  });
  expect(location.pathname).toBe("/files");
  expect(location.search).toContain("file=set");
});
it("uses readable addresses for ordinary navigation but reports missing paths", async () => {
  const { wrapper } = setup("/");
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input) => {
      const path = new URL(String(input), location.origin).searchParams.get(
        "path",
      );
      return path
        ? {
            ok: false,
            status: 404,
            json: async () => ({
              code: "not_found",
              class: "not_found",
              message: "Not found",
            }),
          }
        : {
            ok: true,
            json: async () => ({
              box: "box",
              kind: "box",
              path: "/boxes/2026_trip",
            }),
          };
    }),
  );
  const { result } = renderHook(useResolvedNavigation, { wrapper });
  act(() => result.current.go({ files: true, box: "box" }));
  await waitFor(() => expect(location.pathname).toBe("/boxes/2026_trip"));
  act(() => {
    history.pushState(null, "", "/boxes/missing");
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
  await waitFor(() => expect(result.current.error).toBeTruthy());
  expect(result.current.route.box).toBeUndefined();
  expect(location.pathname).toBe("/boxes/missing");
});
it("retries a failed destination rather than reloading the previous page", async () => {
  const { wrapper } = setup("/");
  let unavailable = true;
  const fetch = vi.fn(async () =>
    unavailable
      ? {
          ok: false,
          status: 503,
          json: async () => ({ code: "unavailable", message: "Try again" }),
        }
      : {
          ok: true,
          json: async () => ({
            box: "box",
            kind: "box",
            path: "/boxes/2026_trip",
          }),
        },
  );
  vi.stubGlobal("fetch", fetch);
  const { result } = renderHook(useResolvedNavigation, { wrapper });
  act(() => result.current.go({ files: true, box: "box" }));
  await waitFor(() => expect(result.current.error).toBeTruthy());
  expect(location.pathname).toBe("/");
  unavailable = false;
  act(() => result.current.retry());
  await waitFor(() => expect(location.pathname).toBe("/boxes/2026_trip"));
  expect(result.current.error).toBeNull();
});
it("does not apply a delayed navigation after leaving for another section", async () => {
  const { wrapper } = setup("/");
  let finish!: (value: unknown) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    ),
  );
  const { result } = renderHook(useResolvedNavigation, { wrapper });
  act(() => result.current.go({ files: true, box: "box" }));
  await waitFor(() => expect(finish).toBeTypeOf("function"));
  act(() => result.current.go({ files: false, section: "settings" }));
  await act(async () =>
    finish({
      ok: true,
      json: async () => ({ box: "box", kind: "box", path: "/boxes/2026_trip" }),
    }),
  );
  expect(location.pathname).toBe("/settings");
  expect(result.current.route.section).toBe("settings");
});
