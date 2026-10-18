import { Button, TextInput } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { AnnotationMode, getDocument, GlobalWorkerOptions } from "pdfjs-dist";
import {
  EventBus,
  PDFFindController,
  PDFLinkService,
  PDFViewer,
} from "pdfjs-dist/web/pdf_viewer.mjs";
import workerURL from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import "pdfjs-dist/web/pdf_viewer.css";
import { useI18n } from "./i18n";

GlobalWorkerOptions.workerSrc = workerURL;

export default function PDFReader({
  src,
  name,
}: {
  src: string;
  name: string;
}) {
  const { t } = useI18n();
  const container = useRef<HTMLDivElement>(null);
  const viewer = useRef<PDFViewer | null>(null);
  const bus = useRef<EventBus | null>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(0);
  const [query, setQuery] = useState("");
  const [matches, setMatches] = useState({ current: 0, total: 0 });
  const [status, setStatus] = useState("loading");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const element = container.current!;
    const abort = new AbortController();
    const eventBus = new EventBus();
    const linkService = new PDFLinkService({
      eventBus,
      externalLinkTarget: 2,
      externalLinkRel: "noopener noreferrer",
    });
    const findController = new PDFFindController({ eventBus, linkService });
    const options = {
      container: element,
      eventBus,
      linkService,
      findController,
      annotationMode: AnnotationMode.ENABLE,
      abortSignal: abort.signal,
    };
    const instance = new PDFViewer(options);
    linkService.setViewer(instance);
    viewer.current = instance;
    bus.current = eventBus;
    setStatus("loading");
    setPages(0);
    setPage(1);
    setQuery("");
    setMatches({ current: 0, total: 0 });
    eventBus.on("pagesinit", () => {
      if (abort.signal.aborted) return;
      instance.currentScaleValue = "auto";
    });
    eventBus.on("pagechanging", ({ pageNumber }: { pageNumber: number }) => {
      if (!abort.signal.aborted) setPage(pageNumber);
    });
    eventBus.on(
      "updatefindmatchescount",
      ({
        matchesCount,
      }: {
        matchesCount: { current: number; total: number };
      }) => {
        if (!abort.signal.aborted) setMatches(matchesCount);
      },
    );
    eventBus.on(
      "updatefindcontrolstate",
      ({
        matchesCount,
      }: {
        matchesCount: { current: number; total: number };
      }) => {
        if (!abort.signal.aborted) setMatches(matchesCount);
      },
    );
    eventBus.on("pagerendered", ({ error }: { error?: unknown }) => {
      if (abort.signal.aborted) return;
      if (error) setStatus("error");
      else {
        setPages(instance.pagesCount);
        setStatus((current) => (current === "error" ? current : "ready"));
      }
    });
    const task = getDocument({
      url: src,
      cMapUrl: "/pdfjs/cmaps/",
      cMapPacked: true,
      standardFontDataUrl: "/pdfjs/standard_fonts/",
      wasmUrl: "/pdfjs/wasm/",
      enableXfa: false,
    });
    void task.promise
      .then((document) => {
        if (abort.signal.aborted) return;
        linkService.setDocument(document);
        instance.setDocument(document);
      })
      .catch(() => {
        if (!abort.signal.aborted) setStatus("error");
      });
    const resize = new ResizeObserver(() => {
      const scale = instance.currentScaleValue;
      if (scale === "auto" || scale === "page-width")
        instance.currentScaleValue = scale;
    });
    resize.observe(element);
    return () => {
      abort.abort();
      resize.disconnect();
      instance.setDocument(null);
      linkService.setDocument(null);
      void instance.l10n?.destroy();
      void task.destroy();
      viewer.current = null;
      bus.current = null;
    };
  }, [src, attempt]);
  function find(value: string, again = false, previous = false) {
    setQuery(value);
    bus.current?.dispatch("find", {
      source: null,
      type: again ? "again" : "",
      query: value,
      caseSensitive: false,
      entireWord: false,
      highlightAll: true,
      findPrevious: previous,
      matchDiacritics: false,
    });
  }
  return (
    <div
      className="pdf-reader"
      aria-label={name}
      aria-busy={status === "loading"}
      data-state={status}
      onKeyDown={(event) => {
        if (
          (event.ctrlKey || event.metaKey) &&
          event.key.toLowerCase() === "f"
        ) {
          event.preventDefault();
          searchInput.current?.focus();
        }
        // Reader navigation and text selection must not switch gallery files.
        if (event.key !== "Escape") event.stopPropagation();
      }}
    >
      <div className="pdf-toolbar" role="toolbar" aria-label={t("pdfTools")}>
        <Button
          disabled={page <= 1 || !pages}
          onClick={() => {
            if (viewer.current) viewer.current.currentPageNumber = page - 1;
          }}
        >
          {t("pdfPrevious")}
        </Button>
        <label className="pdf-page-number">
          <span className="sr-only">{t("pdfPage")}</span>
          <input
            aria-label={t("pdfPage")}
            type="number"
            min={1}
            max={pages || 1}
            value={page}
            disabled={!pages}
            onChange={(event) => {
              const number = Number(event.target.value);
              if (
                viewer.current &&
                Number.isInteger(number) &&
                number >= 1 &&
                number <= pages
              )
                viewer.current.currentPageNumber = number;
            }}
          />
          <span>/ {pages || "—"}</span>
        </label>
        <Button
          disabled={!pages || page >= pages}
          onClick={() => {
            if (viewer.current) viewer.current.currentPageNumber = page + 1;
          }}
        >
          {t("pdfNext")}
        </Button>
        <Button
          aria-label={t("pdfZoomOut")}
          disabled={!pages}
          onClick={() => {
            if (viewer.current)
              viewer.current.currentScale = Math.max(
                0.25,
                viewer.current.currentScale / 1.25,
              );
          }}
        >
          −
        </Button>
        <Button
          aria-label={t("pdfZoomIn")}
          disabled={!pages}
          onClick={() => {
            if (viewer.current)
              viewer.current.currentScale = Math.min(
                5,
                viewer.current.currentScale * 1.25,
              );
          }}
        >
          +
        </Button>
        <Button
          disabled={!pages}
          onClick={() => {
            if (viewer.current) viewer.current.currentScaleValue = "page-width";
          }}
        >
          {t("pdfFitWidth")}
        </Button>
        <TextInput
          ref={searchInput}
          className="pdf-search"
          aria-label={t("pdfSearch")}
          placeholder={t("pdfSearch")}
          value={query}
          disabled={!pages}
          onChange={(event) => find(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              find(query, true, event.shiftKey);
            }
          }}
        />
        <Button
          disabled={!query || !pages}
          aria-label={t("pdfPreviousMatch")}
          onClick={() => find(query, true, true)}
        >
          ↑
        </Button>
        <Button
          disabled={!query || !pages}
          aria-label={t("pdfNextMatch")}
          onClick={() => find(query, true)}
        >
          ↓
        </Button>
        {query && (
          <span role="status">
            {matches.current} / {matches.total}
          </span>
        )}
      </div>
      {status === "error" && (
        <div className="media-placeholder">
          <p>{t("pdfUnavailable")}</p>
          <Button onClick={() => setAttempt(attempt + 1)}>{t("retry")}</Button>
        </div>
      )}
      <div className="pdf-viewport-wrap" hidden={status === "error"}>
        {status === "loading" && (
          <p className="pdf-loading" role="status">
            {t("loading")}
          </p>
        )}
        <div
          ref={container}
          className="pdf-viewport"
          tabIndex={status === "ready" ? 0 : -1}
          aria-hidden={status !== "ready"}
          aria-label={t("pdfPages")}
        >
          <div className="pdfViewer" />
        </div>
      </div>
    </div>
  );
}
