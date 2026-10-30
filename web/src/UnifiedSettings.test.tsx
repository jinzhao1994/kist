import { act, render, screen, waitFor } from "@testing-library/react";
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

it("saves all configuration drafts through the page toolbar in one request", async () => {
  localStorage.setItem(storageKey, JSON.stringify({ language: "en" }));
  const aiDefaults = {
    model: "",
    proxy_url: "",
    auto_ocr: false,
    auto_desc: false,
    auto_facts: false,
    auto_orientation: false,
  };
  let ai = {
    ...aiDefaults,
    model: "test-model",
    defaults: aiDefaults,
    explicit: ["model"],
    revision: "one",
  };
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
  const writes: { path: string; body: any }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const path = url.replace("/api/v1", "");
      if (init?.method === "PUT") {
        expect(path).toBe("/settings");
        const body = JSON.parse(String(init.body));
        writes.push({ path, body });
        ai = { ...ai, ...body.ai, revision: "two" };
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
      if (path === "/ai/settings") return Response.json(ai);
      if (path === "/geo/settings") return Response.json(geo);
      if (path === "/ai/models")
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
  await user.type(language, "en");
  await user.click(
    screen.getByLabelText("AI proxy address", { selector: "input" }),
  );
  await user.paste("http://127.0.0.1:7890");
  const interval = screen.getByRole("textbox", {
    name: "Request interval (seconds)",
  });
  await user.clear(interval);
  expect(saveButton).toBeDisabled();
  await user.hover(saveHint);
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "Correct invalid settings first",
  );
  await user.unhover(saveHint);
  await user.type(interval, "7");
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
      ai: { proxy_url: "http://127.0.0.1:7890", changed: ["proxy_url"] },
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
  await user.type(interval, "9");
  await user.click(screen.getByRole("button", { name: "Reload settings" }));
  await waitFor(() => expect(interval).toHaveValue("7"));
  expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  expect(writes).toHaveLength(1);
  view.unmount();
  client.clear();
});
