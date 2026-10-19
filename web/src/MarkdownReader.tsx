import { Button } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { markdownHeadings } from "./markdownHeadings";
import type { ArchiveFile } from "./api";
import { useI18n } from "./i18n";
import { markdownURL, resolveMarkdownFile } from "./markdownLinks";

const TextReader = lazy(() => import("./TextReader"));
const hasLineLink = () =>
  /^#L[1-9]\d*(?:-(?:L)?[1-9]\d*)?$/.test(window.location.hash);
const imageTypes = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/bmp",
  "image/x-icon",
  "image/vnd.microsoft.icon",
]);

function LocalReference({
  file,
  href,
  image,
  children,
}: {
  file: ArchiveFile;
  href: string;
  image?: boolean;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  const query = useQuery({
    queryKey: ["markdown-reference", file.id, file.folder_id, href],
    retry: false,
    queryFn: ({ signal }) => resolveMarkdownFile(file, href, signal),
  });
  if (query.isError || failed)
    return (
      <span className="markdown-unavailable">
        {children} — {t("markdownMissing")}
      </span>
    );
  if (!query.data)
    return (
      <span className="markdown-unavailable">
        {children} — {t("loading")}
      </span>
    );
  const link = `/files?file=${encodeURIComponent(query.data.id)}`;
  if (image && imageTypes.has(query.data.media_type || ""))
    return (
      <img
        src={`/api/v1/files/${encodeURIComponent(query.data.id)}/content`}
        alt={typeof children === "string" ? children : ""}
        loading="lazy"
        onError={() => setFailed(true)}
      />
    );
  return (
    <a href={link} target="_blank" rel="noopener noreferrer">
      {children}
      {image ? ` · ${t("markdownOpenImage")}` : ""}
    </a>
  );
}

export default function MarkdownReader({
  file,
  text,
  encoding,
  truncated,
  encodingControl,
}: {
  file: ArchiveFile;
  text: string;
  encoding: string;
  truncated: boolean;
  encodingControl: ReactNode;
}) {
  const { t } = useI18n();
  const [source, setSource] = useState(hasLineLink);
  useEffect(() => {
    const restore = () => {
      if (hasLineLink()) setSource(true);
    };
    window.addEventListener("hashchange", restore);
    window.addEventListener("popstate", restore);
    return () => {
      window.removeEventListener("hashchange", restore);
      window.removeEventListener("popstate", restore);
    };
  }, []);
  const modes = (
    <div className="markdown-modes" role="group" aria-label={t("markdownMode")}>
      <Button
        aria-pressed={!source}
        onClick={() => {
          if (hasLineLink()) {
            const url = new URL(location.href);
            url.hash = "";
            history.replaceState(history.state, "", url);
          }
          setSource(false);
        }}
      >
        {t("markdownPreview")}
      </Button>
      <Button aria-pressed={source} onClick={() => setSource(true)}>
        {t("markdownSource")}
      </Button>
    </div>
  );
  if (source)
    return (
      <Suspense
        fallback={<div className="media-placeholder">{t("loading")}</div>}
      >
        <TextReader
          text={text}
          name={file.name}
          encoding={encoding}
          truncated={truncated}
          encodingControl={encodingControl}
          toolbarControl={modes}
        />
      </Suspense>
    );
  return (
    <section
      className="text-reader markdown-reader"
      aria-label={t("markdownPreview")}
      onKeyDown={(event) => {
        if (
          event.key.startsWith("Arrow") ||
          event.key === "Home" ||
          event.key === "End"
        )
          event.stopPropagation();
      }}
    >
      <div className="text-toolbar">
        {modes}
        <span className="text-language">Markdown · {encoding}</span>
        {encodingControl}
      </div>
      {truncated && (
        <p className="text-notice" role="status">
          {t("textTruncated")}
        </p>
      )}
      <div className="markdown-scroll" tabIndex={0}>
        <article className="markdown-body">
          {!text && <p>{t("emptyText")}</p>}
          <Markdown
            remarkPlugins={[remarkGfm]}
            rehypePlugins={[markdownHeadings]}
            skipHtml
            urlTransform={(url) => (markdownURL(url) === "blocked" ? "" : url)}
            components={{
              a: ({ href = "", children }) => {
                const kind = markdownURL(href);
                if (kind === "external")
                  return (
                    <a href={href} target="_blank" rel="noopener noreferrer">
                      {children}
                    </a>
                  );
                if (kind === "relative")
                  return (
                    <LocalReference file={file} href={href}>
                      {children}
                    </LocalReference>
                  );
                if (kind === "anchor")
                  return (
                    <a
                      href={href}
                      onClick={(event) => {
                        event.preventDefault();
                        let id;
                        try {
                          id = decodeURIComponent(href.slice(1));
                        } catch {
                          return;
                        }
                        const article = event.currentTarget.closest("article");
                        const target =
                          article &&
                          Array.from(article.querySelectorAll("[id]")).find(
                            (el) => el.id === `markdown-${id}` || el.id === id,
                          );
                        target?.scrollIntoView({ block: "start" });
                      }}
                    >
                      {children}
                    </a>
                  );
                return <span>{children}</span>;
              },
              img: ({ src, alt = "" }) => {
                const href = typeof src === "string" ? src : "";
                if (markdownURL(href) === "relative")
                  return (
                    <LocalReference file={file} href={href} image>
                      {alt}
                    </LocalReference>
                  );
                if (markdownURL(href) === "external" && !/^mailto:/i.test(href))
                  return (
                    <span className="markdown-unavailable">
                      {alt || t("markdownExternalImage")} ·{" "}
                      {t("markdownImageBlocked")}{" "}
                      <a href={href} target="_blank" rel="noopener noreferrer">
                        {t("markdownOpenImage")}
                      </a>
                    </span>
                  );
                return (
                  <span className="markdown-unavailable">
                    {alt} — {t("markdownMissing")}
                  </span>
                );
              },
            }}
          >
            {text}
          </Markdown>
        </article>
      </div>
    </section>
  );
}
