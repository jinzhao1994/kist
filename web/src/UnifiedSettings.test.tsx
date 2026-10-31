import { aiServicesFixture } from "./aiServices.fixture";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { InstanceSettings } from "./InstanceSettings";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

function setup() {
  localStorage.setItem(storageKey, JSON.stringify({ language: "en" }));
  let ai = aiServicesFixture({ model: "test-model" });
  const geoDefaults = {
    automatic: true,
    endpoint: "https://photon.komoot.io/reverse",
    request_interval_seconds: 2,
    pause_on_error: true,
  };
  let geo = {
    ...geoDefaults,
    defaults: geoDefaults,
    explicit: [] as string[],
    revision: "one",
  };
  let settings = {
    mode: "library",
    revision: "one",
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
  let invalidAI = false;
  const writes: { path: string; body: any }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const path = url.replace("/api/v1", "");
      if (init?.method === "PUT") {
        expect(path).toBe("/settings");
        const body = JSON.parse(String(init.body));
        writes.push({ path, body });
        if (invalidAI)
          return Response.json(
            {
              code: "invalid_input",
              message: "Invalid settings",
              fields: [
                {
                  field: "ai_services.accounts.1.id",
                  code: "unique_id",
                  message: "Enter a unique name.",
                },
              ],
            },
            { status: 400 },
          );
        ai = { ...ai, ...body.ai_services, revision: "two" };
        geo = { ...geo, ...body.location, revision: "two" };
        delete (ai as any).changed;
        delete (geo as any).changed;
        settings = {
          ...settings,
          revision: "two",
          fields: [
            {
              ...settings.fields[0],
              value: body.changes["library.language"],
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
      if (path === "/ai")
        return Response.json({
          supported: true,
          connected: true,
          pending: false,
        });
      if (path === "/ai/services") return Response.json(ai);
      if (path === "/geo/settings") return Response.json(geo);
      if (path === "/ai/accounts/1/models")
        return Response.json([{ id: "test-model", name: "Test model" }]);
      if (path === "/info" || path === "/client") return Response.json({});
      return Response.json({
        paused: false,
        queued: 0,
        failed: 0,
        running: "",
        scanning: false,
      });
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <ThemeProvider env="test">
          <InstanceSettings />
        </ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return {
    client,
    view,
    writes,
    failAI: () => {
      invalidAI = true;
    },
  };
}

// Multi-section user flows retain a bounded budget on slower CI workers.
it("saves all configuration drafts through the page toolbar in one request", async () => {
  const { client, view, writes } = setup();
  const language = await screen.findByRole("textbox", {
    name: "Library language",
  });
  expect(
    screen.queryByRole("button", { name: "Save AI settings" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Save GPS settings" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Reload AI settings" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Reload GPS settings" }),
  ).not.toBeInTheDocument();
  const user = userEvent.setup();
  const saveButton = screen.getByRole("button", { name: "Save" });
  const saveHint = saveButton.closest<HTMLElement>(".tooltip-target")!;
  expect(saveButton).toBeDisabled();
  await user.hover(saveHint);
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "No unsaved changes",
  );
  await user.unhover(saveHint);
  await act(async () => saveHint.focus());
  expect(saveHint).toHaveFocus();
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "No unsaved changes",
  );
  await act(async () => saveHint.blur());
  await waitFor(() =>
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument(),
  );
  await user.clear(language);
  await user.paste("en");
  await user.click(screen.getByRole("button", { name: "Add proxy" }));
  await user.click(screen.getByRole("textbox", { name: "Name" }));
  await user.paste("Home proxy");
  await user.click(
    screen.getByLabelText("Proxy address", { selector: "input" }),
  );
  await user.paste("http://127.0.0.1:7890");
  const interval = screen.getByRole("textbox", {
    name: "Request interval (seconds)",
  });
  await user.clear(interval);
  expect(saveButton).toBeEnabled();
  await user.paste("7");
  expect(saveButton).toBeEnabled();
  await user.hover(saveHint);
  expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  await user.unhover(saveHint);
  expect(writes).toHaveLength(0);
  const dirtyUnload = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(dirtyUnload);
  expect(dirtyUnload.defaultPrevented).toBe(true);
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0]).toMatchObject({
    path: "/settings",
    body: {
      revision: "one",
      changes: { "library.language": "en" },
      ai_services: {
        proxies: [{ id: "Home proxy", url: "http://127.0.0.1:7890" }],
      },
      location: {
        request_interval_seconds: 7,
        changed: ["request_interval_seconds"],
      },
    },
  });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled(),
  );
  const savedUnload = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(savedUnload);
  expect(savedUnload.defaultPrevented).toBe(false);
  await user.clear(interval);
  await user.paste("9");
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
  await waitFor(() => expect(interval).toHaveValue("7"));
  expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  expect(writes).toHaveLength(1);
  view.unmount();
  client.clear();
}, 15_000);

it("opens a collapsed account editor and locates its server validation error", async () => {
  const { client, view, failAI } = setup();
  await screen.findByRole("button", { name: "Add account" });
  failAI();
  const user = userEvent.setup();
  const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
  await user.click(screen.getByRole("button", { name: "Add account" }));
  await user.click(screen.getByRole("button", { name: "Add account" }));

  scroll.mockClear();
  await user.click(screen.getByRole("button", { name: "Save" }));
  const editor = await screen.findByRole("region", { name: "Edit AI account" });
  expect(within(editor).getByText("Enter a unique name.")).toBeInTheDocument();
  await waitFor(() =>
    expect(
      scroll.mock.contexts.some(
        (element) =>
          element instanceof HTMLElement &&
          element.dataset.settingField === "ai_services.accounts.1.id",
      ),
    ).toBe(true),
  );
  scroll.mockRestore();
  view.unmount();
  client.clear();
});
