import { act, renderHook, waitFor } from "@testing-library/react";
import {
  focusManager,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { useCatalogRefresh } from "./catalogRefresh";

afterEach(() => {
  vi.unstubAllGlobals();
  focusManager.setFocused(undefined);
});

it("invalidates all catalog scopes only on changes, including inactive folders on return", async () => {
  let revision = "first";
  const fetcher = vi.fn(async () => Response.json({ revision }));
  vi.stubGlobal("fetch", fetcher);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const keys = [
    ["files"],
    ["files", "box", "folder"],
    ["files", "search"],
    ["filesets"],
    ["tags"],
    ["boxes"],
    ["navigation"],
  ];
  for (const key of keys) client.setQueryData(key, { retained: true });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const hook = renderHook(({ path }) => useCatalogRefresh(path), {
    wrapper,
    initialProps: { path: "/files" },
  });
  await waitFor(() =>
    expect(client.getQueryState(["files"])?.isInvalidated).toBe(true),
  );
  for (const key of keys) client.setQueryData(key, { retained: true });
  hook.rerender({ path: "/boxes/one/folder" });
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  expect(client.getQueryState(["files"])?.isInvalidated).toBe(false);
  revision = "second";
  hook.rerender({ path: "/boxes/two" });
  await waitFor(() =>
    expect(client.getQueryState(["files"])?.isInvalidated).toBe(true),
  );
  for (const key of keys) {
    expect(client.getQueryState(key)?.isInvalidated).toBe(true);
    expect(client.getQueryData(key)).toEqual({ retained: true });
    client.setQueryData(key, { retained: true });
  }
  revision = "third";
  act(() => {
    focusManager.setFocused(false);
    focusManager.setFocused(true);
  });
  await waitFor(() =>
    expect(client.getQueryState(["files"])?.isInvalidated).toBe(true),
  );
  expect(client.getQueryState(["catalog-revision"])?.isInvalidated).toBe(false);
  hook.unmount();
  client.clear();
});
