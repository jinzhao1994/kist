import { act, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import PDFReader from "./PDFReader";

const pdf = vi.hoisted(() => ({
  listeners: new Map<string, (event: Record<string, unknown>) => void>(),
}));
vi.mock("pdfjs-dist", () => ({
  AnnotationMode: { ENABLE: 1 },
  GlobalWorkerOptions: {},
  getDocument: () => ({ promise: Promise.resolve({}), destroy: vi.fn() }),
}));
vi.mock("pdfjs-dist/web/pdf_viewer.mjs", () => ({
  EventBus: class {
    on(name: string, listener: (event: Record<string, unknown>) => void) {
      pdf.listeners.set(name, listener);
    }
  },
  PDFLinkService: class {
    setViewer() {}
    setDocument() {}
  },
  PDFFindController: class {},
  PDFViewer: class {
    pagesCount = 2;
    currentScaleValue = "";
    setDocument() {}
  },
}));
beforeEach(() => pdf.listeners.clear());
function mount() {
  return render(
    <ThemeProvider env="test">
      <I18nProvider>
        <PDFReader src="/sample.pdf" name="Slides" />
      </I18nProvider>
    </ThemeProvider>,
  );
}
function event(name: string, payload: Record<string, unknown> = {}) {
  act(() => pdf.listeners.get(name)?.(payload));
}
it("keeps the loading viewport until a page is painted, not merely initialized", async () => {
  const view = mount();
  await act(async () => {});
  event("pagesinit");
  expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
  expect(screen.getByLabelText("Slides")).toHaveAttribute("aria-busy", "true");
  expect(view.container.querySelector(".pdf-viewport")).toHaveAttribute(
    "aria-hidden",
    "true",
  );
  event("pagerendered", { pageNumber: 1 });
  expect(screen.getByLabelText("Slides")).toHaveAttribute("aria-busy", "false");
  expect(screen.getByRole("button", { name: "Next page" })).toBeEnabled();
  expect(view.container.querySelector(".pdf-viewport")).not.toHaveAttribute(
    "aria-hidden",
    "true",
  );
  expect(screen.queryByRole("status")).toBeNull();
});
it("leaves loading on render failure and ignores late events after unmount", async () => {
  const view = mount();
  await act(async () => {});
  event("pagesinit");
  event("pagerendered", { error: new Error("decode failed") });
  expect(screen.getByText(/Unable to read this PDF/)).toBeVisible();
  expect(screen.getByLabelText("Slides")).toHaveAttribute("aria-busy", "false");
  view.unmount();
  event("pagerendered", { pageNumber: 1 });
  expect(view.container).toBeEmptyDOMElement();
});
