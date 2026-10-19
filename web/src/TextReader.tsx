import { Button, TextInput } from "@mantine/core";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import {
  HighlightStyle,
  LanguageDescription,
  syntaxHighlighting,
} from "@codemirror/language";
import { languages } from "@codemirror/language-data";
import {
  findNext,
  findPrevious,
  openSearchPanel,
  SearchQuery,
  search,
  setSearchQuery,
} from "@codemirror/search";
import { tags } from "@lezer/highlight";
import { useI18n } from "./i18n";
import { textLineSelection } from "./textLineSelection";

const highlight = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--code-keyword)" },
  {
    tag: [tags.string, tags.special(tags.string)],
    color: "var(--code-string)",
  },
  { tag: [tags.number, tags.bool, tags.null], color: "var(--code-number)" },
  { tag: tags.propertyName, color: "var(--code-number)" },
  { tag: tags.comment, color: "var(--muted)", fontStyle: "italic" },
  {
    tag: [tags.function(tags.variableName), tags.definition(tags.variableName)],
    color: "var(--code-function)",
  },
  {
    tag: [tags.typeName, tags.className, tags.tagName],
    color: "var(--code-type)",
  },
]);

export default function TextReader({
  text,
  name,
  encoding,
  truncated,
  encodingControl,
  toolbarControl,
}: {
  text: string;
  name: string;
  encoding: string;
  truncated: boolean;
  encodingControl: ReactNode;
  toolbarControl?: ReactNode;
}) {
  const { t } = useI18n();
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const wrapping = useRef(new Compartment());
  const searchInput = useRef<HTMLInputElement>(null);
  const [wrap, setWrap] = useState(/\.(txt|log)$/i.test(name));
  const [query, setQuery] = useState("");
  const [language, setLanguage] = useState("");
  const [missing, setMissing] = useState(false);
  useEffect(() => {
    const syntax = new Compartment();
    const instance = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: text,
        extensions: [
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
          EditorView.contentAttributes.of({
            tabindex: "0",
            role: "textbox",
            "aria-readonly": "true",
            "aria-label": name,
          }),
          textLineSelection(),
          search({
            createPanel: () => {
              // React owns the toolbar; keep CodeMirror's match decorations.
              const dom = document.createElement("div");
              dom.hidden = true;
              return { dom };
            },
          }),
          syntaxHighlighting(highlight),
          syntax.of([]),
          wrapping.current.of(wrap ? EditorView.lineWrapping : []),
          EditorView.theme({
            "&": {
              height: "100%",
              backgroundColor: "var(--surface)",
              color: "var(--text)",
            },
            ".cm-scroller": {
              overflow: "auto",
              fontFamily: "var(--font-mono, monospace)",
              fontSize: "14px",
              lineHeight: "1.8",
            },
            ".cm-content": { padding: "12px 0" },
            ".cm-line": { padding: "0 12px" },
            ".cm-gutters": {
              backgroundColor: "var(--surface)",
              color: "var(--muted)",
              borderRight: "1px solid var(--border)",
            },
            ".cm-lineNumbers .cm-gutterElement": { padding: "0 14px" },
            ".cm-searchMatch": {
              backgroundColor: "var(--mantine-color-yellow-light)",
            },
            ".cm-searchMatch-selected": {
              backgroundColor: "var(--mantine-color-blue-light)",
            },
            "&.cm-focused": {
              outline: "2px solid var(--mantine-primary-color-filled)",
              outlineOffset: "-2px",
            },
          }),
        ],
      }),
    });
    view.current = instance;
    let disposed = false;
    const description = LanguageDescription.matchFilename(languages, name);
    setLanguage(description?.name || "");
    // Bound parsing work for huge single-line files; plain text stays readable.
    if (description && !text.split("\n").some((line) => line.length > 20000))
      void description
        .load()
        .then((support) => {
          if (!disposed)
            instance.dispatch({
              effects: syntax.reconfigure([
                support,
                // YAML's unquoted values use content, not string tokens.
                // Scope this rule so prose in other languages stays neutral.
                ...(description.name === "YAML"
                  ? [
                      syntaxHighlighting(
                        HighlightStyle.define(
                          [{ tag: tags.content, color: "var(--code-string)" }],
                          { scope: support.language },
                        ),
                      ),
                    ]
                  : []),
              ]),
            });
        })
        .catch(() => {
          if (!disposed) setLanguage("");
        });
    return () => {
      disposed = true;
      instance.destroy();
      view.current = null;
    };
    // The parent remounts this reader when the file, content or encoding changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, name]);
  function toggleWrap() {
    const next = !wrap;
    setWrap(next);
    view.current?.dispatch({
      effects: wrapping.current.reconfigure(
        next ? EditorView.lineWrapping : [],
      ),
    });
  }
  function find(previous = false) {
    const editor = view.current;
    if (!editor || !query) return;
    editor.dispatch({
      effects: setSearchQuery.of(
        new SearchQuery({ search: query, literal: true }),
      ),
    });
    openSearchPanel(editor);
    setMissing(!(previous ? findPrevious : findNext)(editor));
  }
  return (
    <section
      className="text-reader"
      aria-label={t("textReader")}
      onKeyDown={(event) => {
        if (
          (event.ctrlKey || event.metaKey) &&
          event.key.toLowerCase() === "f"
        ) {
          event.preventDefault();
          searchInput.current?.focus();
        }
        if (
          event.key.startsWith("Arrow") ||
          event.key === "Home" ||
          event.key === "End"
        )
          event.stopPropagation();
      }}
    >
      <div className="text-toolbar">
        {toolbarControl}
        <span className="text-language">
          {language || t("plainText")} · {encoding}
        </span>
        <Button aria-pressed={wrap} onClick={toggleWrap}>
          {t("textWrap")}
        </Button>
        {encodingControl}
        <div className="text-search">
          <TextInput
            ref={searchInput}
            aria-label={t("textSearch")}
            placeholder={t("textSearch")}
            value={query}
            onChange={(event) => {
              setQuery(event.currentTarget.value);
              setMissing(false);
              view.current?.dispatch({
                effects: setSearchQuery.of(
                  new SearchQuery({
                    search: event.currentTarget.value,
                    literal: true,
                  }),
                ),
              });
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                find(event.shiftKey);
              }
            }}
          />
          <Button
            disabled={!query}
            aria-label={t("pdfPreviousMatch")}
            onClick={() => find(true)}
          >
            ↑
          </Button>
          <Button
            disabled={!query}
            aria-label={t("pdfNextMatch")}
            onClick={() => find()}
          >
            ↓
          </Button>
        </div>
      </div>
      {truncated && (
        <p className="text-notice" role="status">
          {t("textTruncated")}
        </p>
      )}
      {missing && (
        <p className="text-notice" role="status">
          {t("textNoMatches")}
        </p>
      )}
      {!text && <p className="text-notice">{t("emptyText")}</p>}
      <div className="text-editor" ref={host} />
    </section>
  );
}
