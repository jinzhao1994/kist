import { useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useNavigation } from "./navigation";
import { Gallery } from "./Gallery";
import { emptyFilters } from "./Filters";
import { zeroID } from "./Filesets";
import { ThemeProvider } from "./theme";
import { I18nProvider, storageKey } from "./i18n";
import type { ArchiveFile } from "./api";

vi.mock("./Media", () => ({
  Media: ({ file }: { file: ArchiveFile }) => (
    <div data-testid="media">{file.name}</div>
  ),
  OpenMediaButton: () => null,
  usesPDFReader: () => false,
  usesTextPreview: () => false,
}));
const time = "2026-01-01T00:00:00Z";
const files = ["first", "second", "other"].map((id) => ({
  id,
  name: `${id}.jpg`,
  folder_id: "set",
  box_id: "box",
  blob_key: id,
  created_at: time,
  updated_at: time,
  captured_at: null,
}));
function fixture(empty = false) {
  const set = {
    folder_id: "set",
    kind: "document_pages",
    layout: "directory",
    primary: zeroID,
    members: empty
      ? []
      : files.slice(0, 2).map((file) => ({
          file_id: file.id,
          folder_id: zeroID,
          role: "page",
          language: "",
          default: false,
        })),
  };
  const folder = {
    id: "set",
    name: "Travel documents",
    parent_id: "root",
    total_size: 42,
    created_at: time,
    updated_at: time,
  };
  const fetcher = vi.fn(async (input: string) => {
    const url = new URL(input, "http://localhost");
    if (url.pathname === "/api/v1/filesets")
      return Response.json({ filesets: [set] });
    if (url.pathname === "/api/v1/filesets/set")
      return Response.json({ fileset: set, folder, box_id: "box" });
    if (url.pathname === "/api/v1/entries")
      return Response.json({
        files: [
          { ...files[0], fileset_id: "set", name: folder.name },
          files[2],
        ],
      });
    if (url.pathname === "/api/v1/files") return Response.json({ files });
    if (url.pathname === "/api/v1/boxes/box/folders")
      return Response.json({ folders: [folder] });
    const file = files.find(
      (file) => url.pathname === `/api/v1/files/${file.id}`,
    );
    if (file)
      return Response.json({
        file,
        tags: [],
        metadata: [
          { key: "note", value: `${file.id} metadata`, source: "user" },
        ],
        extracted: [],
      });
    throw Error(`Unexpected request: ${input}`);
  });
  vi.stubGlobal("fetch", fetcher);
  return fetcher;
}
function wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>{children}</I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
beforeEach(() => {
  window.history.replaceState(null, "", "/");
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
});
afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});
it("uses the gallery shell and follows current member metadata while turning pages", async () => {
  const fetcher = fixture();
  const user = userEvent.setup();
  const change = vi.fn();
  render(
    <Gallery
      id="set"
      filters={emptyFilters}
      tags={[]}
      change={change}
      close={vi.fn()}
      history={vi.fn()}
    />,
    { wrapper },
  );
  expect(
    await screen.findByRole("heading", { name: "Travel documents" }),
  ).toBeVisible();
  expect(await screen.findByTestId("media")).toHaveTextContent("first.jpg");
  await user.click(screen.getByRole("button", { name: "File details" }));
  expect(await screen.findByText("first metadata")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Next page" }));
  expect(screen.getByTestId("media")).toHaveTextContent("second.jpg");
  const pageNumber = screen.getByRole("spinbutton", { name: "Page number" });
  fireEvent.change(pageNumber, { target: { value: "1" } });
  expect(screen.getByTestId("media")).toHaveTextContent("first.jpg");
  fireEvent.change(pageNumber, { target: { value: "99" } });
  expect(pageNumber).toHaveValue(1);
  fireEvent.keyDown(pageNumber, { key: "ArrowRight" });
  expect(change).not.toHaveBeenCalled();
  fireEvent.change(pageNumber, { target: { value: "2" } });
  expect(screen.getByTestId("media")).toHaveTextContent("second.jpg");

  expect(await screen.findByText("second metadata")).toBeVisible();
  expect(screen.queryByText("first metadata")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Page actions" }));
  await user.click(
    screen.getByRole("menuitem", { name: "Open current page original" }),
  );
  expect(change).toHaveBeenLastCalledWith("second");
  await user.click(screen.getByRole("button", { name: "Next file" }));
  expect(change).toHaveBeenLastCalledWith("other");
  expect(fetcher.mock.calls.some(([url]) => url === "/api/v1/files/set")).toBe(
    false,
  );
});
it("opens empty combinations without requesting a nonexistent primary", async () => {
  const fetcher = fixture(true);
  const user = userEvent.setup();
  render(
    <Gallery
      id="set"
      filters={emptyFilters}
      tags={[]}
      change={vi.fn()}
      close={vi.fn()}
      history={vi.fn()}
    />,
    { wrapper },
  );
  expect(
    await screen.findByRole("heading", { name: "Travel documents" }),
  ).toBeVisible();
  expect(
    await screen.findByRole("spinbutton", { name: "Page number" }),
  ).toHaveValue(0);
  expect(
    screen.getByRole("spinbutton", { name: "Page number" }),
  ).toBeDisabled();
  expect(screen.queryByTestId("media")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "More actions" }));
  expect(
    screen.getByRole("menuitem", { name: "Configure file set" }),
  ).toBeEnabled();
  expect(fetcher.mock.calls.some(([url]) => url === "/api/v1/files/set")).toBe(
    false,
  );
});
it("navigates between a regular file and a combination without leaving the gallery", async () => {
  fixture();
  const user = userEvent.setup();
  function Harness() {
    const [id, setID] = useState("other");
    return (
      <Gallery
        id={id}
        filters={emptyFilters}
        tags={[]}
        change={setID}
        close={vi.fn()}
        history={vi.fn()}
      />
    );
  }
  render(<Harness />, { wrapper });
  expect(
    await screen.findByRole("heading", { name: "other.jpg" }),
  ).toBeVisible();
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Previous file" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Previous file" }));
  expect(
    await screen.findByRole("heading", { name: "Travel documents" }),
  ).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Next file" }));
  expect(
    await screen.findByRole("heading", { name: "other.jpg" }),
  ).toBeVisible();
});

it("closes the configuration dialog without dismissing the gallery", async () => {
  fixture();
  const user = userEvent.setup();
  const close = vi.fn();
  render(
    <Gallery
      id="set"
      filters={emptyFilters}
      tags={[]}
      change={vi.fn()}
      close={close}
      history={vi.fn()}
    />,
    { wrapper },
  );
  await screen.findByRole("heading", { name: "Travel documents" });
  await user.click(screen.getByRole("button", { name: "More actions" }));
  await user.click(
    screen.getByRole("menuitem", { name: "Configure file set" }),
  );
  expect(await screen.findByRole("dialog")).toBeVisible();
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(close).not.toHaveBeenCalled();
  expect(
    screen.getByRole("heading", { name: "Travel documents" }),
  ).toBeVisible();
});

it("restores the URL page and preserves it on reload without changing the gallery identity", async () => {
  fixture();
  window.history.replaceState(
    { kistGallery: true },
    "",
    "/boxes/box?file=set&page=2",
  );
  localStorage.setItem("kist.gallery.details.v1", "open");
  const history = vi.fn();
  function Harness() {
    const { route, go } = useNavigation();
    return (
      <Gallery
        id={route.file!}
        page={route.page}
        changePage={(page) => go({ ...route, page }, true)}
        filters={emptyFilters}
        tags={[]}
        change={(file) => go({ ...route, file, page: undefined }, true)}
        close={vi.fn()}
        history={history}
      />
    );
  }
  const view = render(<Harness />, { wrapper });
  expect(await screen.findByTestId("media")).toHaveTextContent("second.jpg");
  expect(await screen.findByText("second metadata")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Previous page" }));
  expect(await screen.findByText("first metadata")).toBeVisible();
  expect(new URL(location.href).searchParams.get("page")).toBe("1");
  expect(window.history.state).toEqual({ kistGallery: true });
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(new URL(location.href).searchParams.get("file")).toBe("set");
  expect(new URL(location.href).searchParams.get("page")).toBe("2");
  view.unmount();
  render(<Harness />, { wrapper });
  expect(await screen.findByTestId("media")).toHaveTextContent("second.jpg");
  expect(await screen.findByText("second metadata")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "File versions" }));
  expect(history).toHaveBeenLastCalledWith(undefined, "second");
  fireEvent.click(screen.getByRole("button", { name: "Next file" }));
  expect(
    await screen.findByRole("heading", { name: "other.jpg" }),
  ).toBeVisible();
  expect(new URL(location.href).searchParams.has("page")).toBe(false);
});
