import {
  render,
  screen,
  waitFor,
  fireEvent,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ClientConflictEditor } from "./ClientConflictEditor";
import { I18nProvider, storageKey } from "./i18n";
import { zeroID } from "./Filesets";
import { ThemeProvider } from "./theme";
let bodies: any[];
let fail: boolean;
let supported: boolean;
let draftOverride: any;
beforeEach(() => {
  localStorage.clear();
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      language: "en",
      formatLocale: "en-US",
      timezone: "UTC",
      direction: "ltr",
    }),
  );
  bodies = [];
  fail = false;
  supported = true;
  draftOverride = undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, options?: RequestInit) => {
      if (options?.method === "POST") {
        bodies.push(JSON.parse(options.body as string));
        if (fail) {
          fail = false;
          throw new TypeError("lost response");
        }
        return new Response(JSON.stringify({}), { status: 200 });
      }
      if (_path.endsWith("/filesets"))
        return Response.json({ filesets: [draftOverride.fileset.current] });
      if (_path.includes("/folders"))
        return Response.json({
          folders: [{ id: "set", name: "Pages", parent_id: "root" }],
        });
      if (_path.includes("/files?"))
        return Response.json({
          files: ["first", "second"].map((id) => ({
            id,
            name: `${id}.png`,
            folder_id: "set",
            box_id: "box",
          })),
        });
      if (draftOverride) return Response.json(draftOverride);
      return new Response(
        JSON.stringify({
          summary: {},
          fields: supported
            ? [
                {
                  key: "name",
                  path: ["name", "after"],
                  before: "Original title",
                  current: "NAS title",
                  proposed: "Laptop title",
                },
              ]
            : [],
          replacement: supported
            ? {
                version: 1,
                id: "stable-replacement",
                occurred_at: "2026-10-21T10:00:00Z",
                action: "update_box",
                origin: "user",
                payload: {
                  box_id: "box",
                  name: { before: "NAS title", after: "Laptop title" },
                },
              }
            : null,
        }),
        { status: 200 },
      );
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());
function setup() {
  const close = vi.fn(),
    locked = vi.fn();
  const rendered = render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <ClientConflictEditor
            id="conflict"
            action="update_box"
            close={close}
            onLocked={locked}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { close, locked, ...rendered };
}
it("shows the compared values and keeps the exact replacement when retrying a lost response", async () => {
  fail = true;
  const { close, locked, unmount } = setup();
  const input = await screen.findByLabelText("Value to submit");
  expect(screen.getByText(/Before your change · Original title/)).toBeVisible();
  expect(screen.getByText(/Current value · NAS title/)).toBeVisible();
  await userEvent.clear(input);
  await userEvent.type(input, "Reconciled title");
  await userEvent.click(
    screen.getByRole("button", { name: "Save revised change" }),
  );
  await screen.findByRole("alert");
  expect(input).toBeDisabled();
  expect(close).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(bodies).toHaveLength(2);
  expect(bodies[1]).toEqual(bodies[0]);
  expect(bodies[0].replacement.id).toBe("stable-replacement");
  expect(bodies[0].replacement.payload.name).toEqual({
    before: "NAS title",
    after: "Reconciled title",
  });
  unmount();
  expect(locked).toHaveBeenLastCalledWith(false);
});
it("does not offer an unsafe generic overwrite for unsupported changes", async () => {
  supported = false;
  setup();
  expect(
    await screen.findByText(/This change requires its own domain editor/),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Save revised change" }),
  ).not.toBeInTheDocument();
  expect(bodies).toEqual([]);
});

it("uses the partial-date editor without expanding month precision", async () => {
  draftOverride = {
    fields: [
      {
        key: "start_at",
        path: ["start_at", "after"],
        before: "2026",
        current: "2026-10-22",
        proposed: "2026-10",
      },
    ],
    replacement: {
      version: 1,
      id: "date-replacement",
      occurred_at: "2026-10-21T10:00:00Z",
      action: "update_box",
      origin: "user",
      payload: {
        box_id: "box",
        start_at: { before: "2026-10-22", after: "2026-10" },
      },
    },
  };
  setup();
  const input = await screen.findByLabelText("Value to submit");
  expect(input).toHaveValue("2026-10");
  fireEvent.change(input, { target: { value: "2026-11" } });
  await userEvent.click(
    screen.getByRole("button", { name: "Save revised change" }),
  );
  await waitFor(() => expect(bodies).toHaveLength(1));
  expect(bodies[0].replacement.payload.start_at).toEqual({
    before: "2026-10-22",
    after: "2026-11",
  });
});
it("reuses page ordering while submitting a reviewed fileset replacement", async () => {
  const members = ["first", "second"].map((file_id) => ({
    file_id,
    folder_id: zeroID,
    role: "page",
    language: "",
    default: false,
  }));
  const before = {
    folder_id: "set",
    kind: "document_pages",
    layout: "flat",
    primary: zeroID,
    members,
  };
  const current = { ...before, layout: "directory" };
  const proposed = {
    ...before,
    primary: "second",
    members: [...members].reverse(),
  };
  draftOverride = {
    fields: [],
    fileset: {
      box_id: "box",
      folder_id: "set",
      name: "Pages",
      before,
      current,
      proposed,
    },
    replacement: {
      version: 1,
      id: "fileset-replacement",
      occurred_at: "2026-10-21T10:00:00Z",
      action: "set_fileset",
      origin: "user",
      payload: {
        box_id: "box",
        folder_id: "set",
        ancestors: ["set", "root"],
        before: current,
        after: proposed,
      },
    },
  };
  const { close } = setup();
  const handle = await screen.findByRole("button", {
    name: "Reorder second.png",
  });
  const list = screen.getByRole("list", { name: "Pages" });
  expect(within(list).getAllByRole("listitem")[0]).toHaveTextContent(
    "second.png",
  );
  fireEvent.keyDown(handle, { key: "ArrowDown" });
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(bodies).toHaveLength(1);
  expect(bodies[0].replacement.payload.before).toEqual(current);
  expect(bodies[0].replacement.payload.after.primary).toBe("second");
  expect(
    bodies[0].replacement.payload.after.members.map((m: any) => m.file_id),
  ).toEqual(["first", "second"]);
  expect(bodies[0].replacement.payload.after.layout).toBe("flat");
});

it("requires a valid cover choice and retries the exact reviewed replacement", async () => {
  fail = true;
  draftOverride = {
    fields: [],
    cover: {
      before: { id: "before", name: "Before.png", available: true },
      current: { id: "current", name: "NAS.png", available: true },
      proposed: { id: "missing", name: "", available: false },
    },
    replacement: {
      version: 1,
      id: "cover-replacement",
      occurred_at: "2026-10-21T10:00:00Z",
      action: "update_box",
      origin: "user",
      payload: {
        box_id: "box",
        cover_file_id: { before: "current", after: "missing" },
      },
    },
  };
  const { close } = setup();
  const choice = await screen.findByRole("combobox", {
    name: "Value to submit",
  });
  expect(screen.getByText("Current value · NAS.png")).toBeVisible();
  const save = screen.getByRole("button", { name: "Save revised change" });
  expect(save).toBeDisabled();
  await userEvent.click(choice);
  await userEvent.click(screen.getByRole("option", { name: "No cover" }));
  expect(save).toBeEnabled();
  await userEvent.click(save);
  await screen.findByRole("alert");
  expect(choice).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(close).toHaveBeenCalledOnce());
  expect(bodies).toHaveLength(2);
  expect(bodies[1]).toEqual(bodies[0]);
  expect(bodies[0].replacement.payload.cover_file_id).toEqual({
    before: "current",
    after: zeroID,
  });
  expect(bodies[0].replacement.id).toBe("cover-replacement");
});
