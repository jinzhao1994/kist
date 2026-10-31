import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { InstanceSettings } from "./InstanceSettings";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

function mount(
  initial: { id: string; note: string; created_at: string }[] = [],
) {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  let credentials = initial;
  let settings = {
    mode: "client",
    revision: "one",
    fields: [
      {
        key: "server.max_upload_files",
        kind: "integer",
        value: 1000,
        default: 1000,
        explicit: false,
        restart: false,
      },
    ],
    effective: {},
    overrides: [],
    restart_required: [],
  };
  const writes: { path: string; body: any }[] = [];
  let fail = false;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (init?.method && init.method !== "GET") {
        const body = JSON.parse(String(init.body));
        writes.push({ path, body });
        if (path !== "/api/v1/settings" || init.method !== "PUT")
          throw new Error("Credential mutation bypassed unified save");
        if (fail)
          return Response.json(
            {
              code: "invalid_request",
              class: "invalid",
              message: "Invalid note",
              fields: [
                {
                  field: "credentials.items.0.note",
                  code: "credential_note",
                  message: "Use at most 200 characters.",
                },
              ],
            },
            { status: 400 },
          );
        const created: {
          id: string;
          note: string;
          created_at: string;
          token: string;
        }[] = [];
        if (body.credentials)
          credentials = body.credentials.items.map(
            (item: { id: string; note: string }, index: number) => {
              const prior = credentials.find((row) => row.id === item.id);
              if (prior) return { ...prior, note: item.note };
              const row = {
                id: `synthetic-${index}`,
                note: item.note,
                created_at: "2026-10-31T00:00:00Z",
              };
              created.push({ ...row, token: `synthetic-secret-${index}` });
              return row;
            },
          );
        settings = { ...settings, revision: "two" };
        return Response.json({ ...settings, created_credentials: created });
      }
      if (path === "/api/v1/settings") return Response.json(settings);
      if (path === "/api/v1/settings/credentials")
        return Response.json(credentials);
      if (path === "/api/v1/settings/storage")
        return Response.json({
          originals_bytes: 0,
          metadata_bytes: 0,
          previews_bytes: 0,
        });
      if (["/api/v1/info", "/api/v1/client"].includes(path))
        return Response.json({});
      throw new Error(`Unexpected request ${path}`);
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <InstanceSettings />
          <a href="/elsewhere">Elsewhere</a>
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return {
    ...view,
    client,
    writes,
    fail: (value: boolean) => {
      fail = value;
    },
  };
}

it("stages a credential together with general settings and shows its secret only after save", async () => {
  const user = userEvent.setup();
  const { client, writes } = mount();
  await user.click(
    await screen.findByRole("button", { name: "Create credential" }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Note (optional)" }),
    "Phone",
  );
  await user.click(screen.getByRole("button", { name: "Create credential" }));
  expect(screen.getByText("Generated on save")).toBeVisible();
  expect(screen.queryByText(/synthetic-secret/)).not.toBeInTheDocument();
  const limit = screen.getByRole("textbox", {
    name: "Maximum files per upload",
  });
  await user.clear(limit);
  await user.type(limit, "99");
  expect(writes).toEqual([]);
  await user.click(screen.getByRole("link", { name: "Elsewhere" }));
  await user.click(screen.getByRole("button", { name: "Save and leave" }));
  expect(await screen.findByText("synthetic-secret-0")).toBeVisible();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({
    credentials: { items: [{ id: "", note: "Phone" }] },
    changes: { "server.max_upload_files": 99 },
  });
  expect(
    JSON.stringify(
      client
        .getQueryCache()
        .getAll()
        .map((query) => query.state.data),
    ),
  ).not.toContain("synthetic-secret");
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled(),
  );
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
  await waitFor(() =>
    expect(screen.queryByText("synthetic-secret-0")).not.toBeInTheDocument(),
  );
});

it("stages edits and revocations, clears reverted differences and discards them on reload", async () => {
  const user = userEvent.setup();
  const { writes } = mount([
    { id: "existing", note: "Original", created_at: "2026-10-01T00:00:00Z" },
  ]);
  const edit = await screen.findByRole("button", { name: "Edit note" });
  await user.click(edit);
  const note = screen.getByRole("textbox", { name: "Note (optional)" });
  await user.clear(note);
  await user.type(note, "Other");
  expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  await user.clear(note);
  await user.type(note, "Original");
  expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Done" }));
  await user.click(screen.getByRole("button", { name: "Revoke" }));
  expect(screen.queryByText("existing")).not.toBeInTheDocument();
  expect(writes).toEqual([]);
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
  expect(await screen.findByText("existing")).toBeVisible();
  expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
});

it("keeps rejected credential edits and opens the server's field error next to the note", async () => {
  const user = userEvent.setup();
  const view = mount([
    { id: "existing", note: "Original", created_at: "2026-10-01T00:00:00Z" },
  ]);
  await user.click(await screen.findByRole("button", { name: "Edit note" }));
  const note = screen.getByRole("textbox", { name: "Note (optional)" });
  await user.clear(note);
  await user.type(note, "Rejected");
  await user.click(screen.getByRole("button", { name: "Done" }));
  view.fail(true);
  await user.click(screen.getByRole("button", { name: "Save" }));
  const reopened = await screen.findByRole("textbox", {
    name: "Note (optional)",
  });
  expect(reopened).toHaveValue("Rejected");
  expect(reopened).toHaveAttribute("aria-invalid", "true");
  expect(
    within(reopened.closest<HTMLElement>(".setting-field")!).getByText(
      "Use at most 200 characters.",
    ),
  ).toBeVisible();
});
