import { afterEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { Media } from "./Media";
import type { ArchiveFile } from "./api";
vi.mock("./TextReader", () => ({
  default: ({ text, name }: { text: string; name: string }) => (
    <div aria-label={name}>{text}</div>
  ),
}));
const file: ArchiveFile = {
  id: "text",
  name: "README.custom",
  blob_key: "sha256:text",
  folder_id: "folder",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  captured_at: null,
};
function mount(props: Partial<Parameters<typeof Media>[0]> = {}) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ThemeProvider env="test">
        <I18nProvider>
          <Media file={file} {...props} />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
afterEach(() => vi.unstubAllGlobals());
it("reads unknown suffixes only when visible and renders escaped thumbnail text", async () => {
  const fetch = vi.fn().mockResolvedValue(
    Response.json({
      supported: true,
      text: "<script>hello</script>",
      encoding: "UTF-8",
      truncated: false,
    }),
  );
  vi.stubGlobal("fetch", fetch);
  const inactive = mount({ active: false });
  expect(fetch).not.toHaveBeenCalled();
  inactive.unmount();
  const paused = mount({ paused: true });
  expect(fetch).not.toHaveBeenCalled();
  paused.unmount();
  const view = mount();
  expect(await screen.findByLabelText("Text preview")).toHaveTextContent(
    "<script>hello</script>",
  );
  expect(view.container.querySelector("script")).toBeNull();
  expect(fetch.mock.calls[0][0]).toBe(
    "/api/v1/files/text/text-preview?detail=false&encoding=auto",
  );
});
it("uses the same reader for extensionless text details without fetching original bytes", async () => {
  const fetch = vi.fn().mockResolvedValue(
    Response.json({
      supported: true,
      text: "plain words",
      encoding: "UTF-8",
      truncated: false,
    }),
  );
  vi.stubGlobal("fetch", fetch);
  mount({ detail: true, file: { ...file, name: "README" } });
  expect(await screen.findByLabelText("README")).toHaveTextContent(
    "plain words",
  );
  expect(fetch.mock.calls[0][0]).toBe(
    "/api/v1/files/text/text-preview?detail=true&encoding=auto",
  );
});
it("offers an explicit legacy encoding retry instead of showing corrupt text", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(
      Response.json({
        supported: false,
        text: "",
        encoding: "UTF-8",
        truncated: false,
        reason: "encoding",
      }),
    )
    .mockResolvedValueOnce(
      Response.json({
        supported: true,
        text: "中文",
        encoding: "GB18030",
        truncated: false,
      }),
    );
  vi.stubGlobal("fetch", fetch);
  mount({ detail: true });
  expect(await screen.findByText(/Cannot decode as Unicode/)).toBeVisible();
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Text encoding" }),
    "gb18030",
  );
  expect(await screen.findByLabelText(file.name)).toHaveTextContent("中文");
  expect(fetch.mock.calls[1][0]).toContain("encoding=gb18030");
});
