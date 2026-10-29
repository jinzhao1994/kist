import { QueryClient } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { waitFor } from "@testing-library/react";
import { clientQueries, loadClientPage } from "./clientQueries";

afterEach(() => vi.unstubAllGlobals());
it("starts retained items and history alongside status and shares their loaded data", async () => {
  let status!: (r: Response) => void, history!: (r: Response) => void;
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      paths.push(url);
      if (url.endsWith("/status"))
        return new Promise<Response>((r) => {
          status = r;
        });
      if (url.includes("/history?"))
        return new Promise<Response>((r) => {
          history = r;
        });
      return Response.json({});
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  let done = false;
  const loaded = loadClientPage(client).then((value) => {
    done = true;
    return value;
  });
  await waitFor(() => expect(paths).toHaveLength(3));
  expect(paths).toContain("/api/v1/client/offline");
  expect(paths).toContain("/api/v1/client/history?limit=25&before=");
  status(Response.json({ pending: [], connection: null }));
  await Promise.resolve();
  expect(done).toBe(false);
  history(Response.json({ entries: [] }));
  expect(await loaded).toMatchObject({ pending: [] });
  expect(client.getQueryData(["client", "history"])).toMatchObject({
    pages: [{ entries: [] }],
  });
  await client.fetchQuery(clientQueries.offline);
  expect(paths).toHaveLength(3);
  client.clear();
});
