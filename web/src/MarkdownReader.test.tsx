import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import MarkdownReader from "./MarkdownReader";
import type { ArchiveFile } from "./api";
vi.mock("./TextReader", () => ({
  default: ({
    text,
    toolbarControl,
  }: {
    text: string;
    toolbarControl: React.ReactNode;
  }) => (
    <div data-testid="source">
      {toolbarControl}
      <pre>{text}</pre>
    </div>
  ),
}));
const file = {
  id: "md",
  name: "README.md",
  box_id: "box",
  folder_id: "root",
} as ArchiveFile;
function mount(text: string) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <MarkdownReader
            file={file}
            text={text}
            encoding="UTF-8"
            truncated={false}
            encodingControl={null}
          />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
afterEach(() => {
  vi.unstubAllGlobals();
  history.replaceState(null, "", "/");
});
it("renders GFM without executing HTML or requesting external images", () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const { container } = mount(
    "# Notes\n\n- [x] done\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert%281%29)\n\n![remote](https://example.org/track.png)",
  );
  expect(screen.getByRole("heading", { name: "Notes" })).toBeVisible();
  expect(screen.getByRole("table")).toBeVisible();
  expect(screen.getByRole("checkbox")).toBeDisabled();
  expect(container.querySelector("script, iframe, img")).toBeNull();
  expect(screen.queryByRole("link", { name: "unsafe" })).toBeNull();
  expect(screen.getByRole("link", { name: "Open image" })).toHaveAttribute(
    "rel",
    "noopener noreferrer",
  );
  expect(fetch).not.toHaveBeenCalled();
});
it("opens line links in source and clears them when switching to preview", async () => {
  history.replaceState({ kistGallery: true }, "", "/files?file=md#L2-3");
  mount("# Notes\n\nhello");
  expect(await screen.findByTestId("source")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Preview" }));
  expect(screen.getByRole("heading", { name: "Notes" })).toBeVisible();
  expect(location.hash).toBe("");
  expect(history.state).toEqual({ kistGallery: true });
  await userEvent.click(screen.getByRole("button", { name: "Source" }));
  expect(await screen.findByTestId("source")).toHaveTextContent("# Notes");
});
it("resolves local images and reports missing references", async () => {
  const fetch = vi.fn(async (url: string) => {
    if (url.includes("/folders"))
      return Response.json({
        folders: [{ id: "root", name: "", parent_id: null }],
      });
    const params = new URL(url, "http://local").searchParams;
    return Response.json({
      files:
        params.get("name_equals") === "photo.png"
          ? [
              {
                id: "photo",
                folder_id: "root",
                name: "photo.png",
                media_type: "image/png",
              },
            ]
          : [],
    });
  });
  vi.stubGlobal("fetch", fetch);
  mount("![photo](photo.png)\n\n[missing](missing.md)");
  expect(await screen.findByRole("img", { name: "photo" })).toHaveAttribute(
    "src",
    "/api/v1/files/photo/content",
  );
  await waitFor(() =>
    expect(screen.getByText(/Referenced file is unavailable/)).toBeVisible(),
  );
});
