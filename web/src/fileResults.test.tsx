import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { api } from "./api";
import { emptyFilters } from "./Filters";
import { useFileResults } from "./fileResults";

afterEach(() => vi.restoreAllMocks());
it("sends selected boxes with tag and name filters and keeps box pages scoped", async () => {
  const files = vi
    .spyOn(api, "files")
    .mockResolvedValue({ files: [], cursor: "" });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const filters = {
    ...emptyFilters,
    box: ["a", "b"],
    name: "photo",
    tag_all: ["travel"],
  };
  const { rerender } = renderHook(
    ({ box, folder }: { box?: string; folder?: string }) =>
      useFileResults(box, filters, folder),
    {
      initialProps: {} as { box?: string; folder?: string },
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    },
  );
  await waitFor(() =>
    expect(files).toHaveBeenCalledWith(
      expect.objectContaining({
        box: ["a", "b"],
        name: "photo",
        tag_all: ["travel"],
      }),
      expect.any(AbortSignal),
    ),
  );
  rerender({ box: "fixed", folder: "child" });
  await waitFor(() =>
    expect(files).toHaveBeenLastCalledWith(
      expect.objectContaining({ box: ["fixed"], folder: "child" }),
      expect.any(AbortSignal),
    ),
  );
});

it("keeps folder scope through recursive search, paging, and returning to browsing", async () => {
  const files = vi.spyOn(api, "files").mockImplementation(async (query) => ({
    files: [],
    cursor: query.cursor ? "" : "next",
  }));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const search = {
    ...emptyFilters,
    condition: [JSON.stringify({ field: "name_contains", values: ["match"] })],
  };
  const { result, rerender } = renderHook(
    ({ filters, folder }) => useFileResults("box", filters, folder),
    {
      initialProps: { filters: search, folder: "child" },
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    },
  );
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(files).toHaveBeenLastCalledWith(
    expect.objectContaining({ folder: "child", recursive: true }),
    expect.any(AbortSignal),
  );
  await result.current.fetchNextPage();
  expect(files).toHaveBeenLastCalledWith(
    expect.objectContaining({
      folder: "child",
      recursive: true,
      cursor: "next",
    }),
    expect.any(AbortSignal),
  );
  rerender({ filters: search, folder: "sibling" });
  await waitFor(() =>
    expect(files).toHaveBeenLastCalledWith(
      expect.objectContaining({
        folder: "sibling",
        recursive: true,
        cursor: "",
      }),
      expect.any(AbortSignal),
    ),
  );
  rerender({ filters: { ...emptyFilters, condition: [] }, folder: "child" });
  await waitFor(() =>
    expect(files).toHaveBeenLastCalledWith(
      expect.objectContaining({ folder: "child", recursive: false }),
      expect.any(AbortSignal),
    ),
  );
});
