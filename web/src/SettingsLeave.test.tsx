import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { InstanceSettings } from "./InstanceSettings";
import { useNavigation } from "./navigation";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  history.replaceState(null, "", "/");
});

function setup(fail = false) {
  localStorage.setItem(storageKey, JSON.stringify({ language: "en" }));
  history.replaceState(null, "", "/settings");
  let settings = {
    revision: "one",
    mode: "client",
    fields: [
      {
        key: "library.language",
        kind: "text",
        value: "zh-CN",
        default: "zh-CN",
        explicit: false,
      },
    ],
    effective: {},
    overrides: [],
    restart_required: [],
  };
  const writes: any[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const path = url.replace("/api/v1", "");
      if (init?.method === "PUT") {
        writes.push(JSON.parse(String(init.body)));
        if (fail)
          return Response.json(
            { code: "invalid", message: "Example save failure" },
            { status: 400 },
          );
        settings = {
          ...settings,
          revision: "two",
          fields: [
            {
              ...settings.fields[0],
              value: writes[0].changes["library.language"],
              explicit: true,
            },
          ],
        };
        return Response.json(settings);
      }
      if (path === "/settings") return Response.json(settings);
      if (path === "/settings/credentials") return Response.json([]);
      if (path === "/settings/storage")
        return Response.json({
          originals_bytes: 0,
          metadata_bytes: 0,
          previews_bytes: 0,
        });
      return Response.json({});
    }),
  );
  function Harness() {
    const nav = useNavigation();
    return nav.route.section === "settings" ? (
      <>
        <button onClick={() => nav.go({ files: true })}>Leave settings</button>
        <button
          onClick={() =>
            nav.go(
              { files: false, section: "settings" },
              false,
              "settings-storage",
            )
          }
        >
          Another settings section
        </button>
        <InstanceSettings registerNavigationBlocker={nav.registerBlocker} />
      </>
    ) : (
      <h1>Left settings</h1>
    );
  }
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">
          <Harness />
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return { writes, view, client, user: userEvent.setup() };
}

it("keeps the draft on cancel, allows same-page navigation, and discards only on request", async () => {
  const { user, writes } = setup();
  const input = await screen.findByRole("textbox", {
    name: "Library language",
  });
  const cleanUnload = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(cleanUnload);
  expect(cleanUnload.defaultPrevented).toBe(false);
  await user.clear(input);
  await user.type(input, "en");
  const dirtyUnload = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(dirtyUnload);
  expect(dirtyUnload.defaultPrevented).toBe(true);
  await user.click(
    screen.getByRole("button", { name: "Another settings section" }),
  );
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Leave settings" }));
  const dialog = await screen.findByRole("dialog", {
    name: "Unsaved settings",
  });
  expect(location.pathname).toBe("/settings");
  await user.click(
    within(dialog).getByRole("button", { name: "Continue editing" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(input).toHaveValue("en");
  expect(writes).toHaveLength(0);
  await user.click(screen.getByRole("button", { name: "Leave settings" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", {
      name: "Discard and leave",
    }),
  );
  await screen.findByRole("heading", { name: "Left settings" });
  expect(writes).toHaveLength(0);
  const leftUnload = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(leftUnload);
  expect(leftUnload.defaultPrevented).toBe(false);
});

it("saves once before resuming navigation", async () => {
  const { user, writes } = setup();
  const input = await screen.findByRole("textbox", {
    name: "Library language",
  });
  await user.clear(input);
  await user.type(input, "en");
  await user.click(screen.getByRole("button", { name: "Leave settings" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", {
      name: "Save and leave",
    }),
  );
  await screen.findByRole("heading", { name: "Left settings" });
  expect(writes).toHaveLength(1);
  expect(writes[0].changes).toEqual({ "library.language": "en" });
});

it("keeps the page and draft when saving before leaving fails", async () => {
  const { user, writes } = setup(true);
  const input = await screen.findByRole("textbox", {
    name: "Library language",
  });
  await user.clear(input);
  await user.type(input, "en");
  await user.click(screen.getByRole("button", { name: "Leave settings" }));
  const dialog = await screen.findByRole("dialog");
  await user.click(
    within(dialog).getByRole("button", { name: "Save and leave" }),
  );
  expect(await within(dialog).findByRole("alert")).toHaveTextContent(
    "Check the entered values and start a corrected operation.",
  );
  expect(location.pathname).toBe("/settings");
  expect(input).toHaveValue("en");
  expect(writes).toHaveLength(1);
});

it("restores a blocked Back entry and resumes it after discard", async () => {
  const { user } = setup();
  await screen.findByRole("textbox", { name: "Library language" });
  // Add a prior owned entry, then return to settings without remounting the hook.
  history.replaceState({ kistHistoryIndex: -1 }, "", "/files");
  history.pushState({ kistHistoryIndex: 0 }, "", "/settings");
  const input = screen.getByRole("textbox", { name: "Library language" });
  await user.clear(input);
  await user.type(input, "en");
  await act(async () => {
    history.back();
  });
  const dialog = await screen.findByRole("dialog");
  await waitFor(() => expect(location.pathname).toBe("/settings"));
  expect(input).toHaveValue("en");
  await user.click(
    within(dialog).getByRole("button", { name: "Discard and leave" }),
  );
  await screen.findByRole("heading", { name: "Left settings" });
  expect(location.pathname).toBe("/files");
});

it("protects Forward and restores the exact settings fragment after a multi-entry Back", async () => {
  const { user } = setup();
  const input = await screen.findByRole("textbox", {
    name: "Library language",
  });
  history.replaceState({ kistHistoryIndex: -1 }, "", "/files");
  history.pushState({ kistHistoryIndex: 0 }, "", "/settings");
  await user.clear(input);
  await user.type(input, "en");
  await act(async () => {
    location.hash = "settings-storage";
  });
  await waitFor(() => expect(history.state?.kistHistoryIndex).toBe(1));
  history.pushState({ kistHistoryIndex: 2 }, "", "/files");
  await act(async () => {
    history.back();
  });
  await waitFor(() => expect(location.hash).toBe("#settings-storage"));
  await act(async () => {
    history.go(-2);
  });
  const dialog = await screen.findByRole("dialog");
  await waitFor(() =>
    expect(location.pathname + location.hash).toBe(
      "/settings#settings-storage",
    ),
  );
  await user.click(
    within(dialog).getByRole("button", { name: "Discard and leave" }),
  );
  await screen.findByRole("heading", { name: "Left settings" });
  await act(async () => {
    history.forward();
  });
  const returned = await screen.findByRole("textbox", {
    name: "Library language",
  });
  await user.clear(returned);
  await user.type(returned, "en");
  // Forward targets the retained settings fragment and stays on this page.
  await act(async () => {
    history.forward();
  });
  await waitFor(() => expect(location.hash).toBe("#settings-storage"));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(returned).toHaveValue("en");
  await act(async () => {
    history.forward();
  });
  const forwardDialog = await screen.findByRole("dialog");
  await waitFor(() =>
    expect(location.pathname + location.hash).toBe(
      "/settings#settings-storage",
    ),
  );
  expect(returned).toHaveValue("en");
  await user.click(
    within(forwardDialog).getByRole("button", { name: "Continue editing" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
});
