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
    ({ box }: { box?: string }) => useFileResults(box, filters),
    {
      initialProps: {} as { box?: string },
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
  rerender({ box: "fixed" });
  await waitFor(() =>
    expect(files).toHaveBeenLastCalledWith(
      expect.objectContaining({ box: ["fixed"] }),
      expect.any(AbortSignal),
    ),
  );
});
