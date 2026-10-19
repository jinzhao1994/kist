import { StateEffect, StateField } from "@codemirror/state";
import {
  Decoration,
  EditorView,
  ViewPlugin,
  lineNumbers,
  type DecorationSet,
  type ViewUpdate,
} from "@codemirror/view";

export type LineRange = { from: number; to: number };
export function parseLineHash(hash: string, lines: number): LineRange | null {
  const match = /^#L([1-9]\d*)(?:-(?:L)?([1-9]\d*))?$/.exec(hash);
  if (!match) return null;
  const a = Number(match[1]),
    b = Number(match[2] || match[1]);
  if (!Number.isSafeInteger(a) || !Number.isSafeInteger(b)) return null;
  const from = Math.min(a, b),
    to = Math.min(Math.max(a, b), lines);
  return from <= lines ? { from, to } : null;
}
export function lineHash(range: LineRange) {
  return `#L${range.from}${range.to === range.from ? "" : `-${range.to}`}`;
}
const selectLines = StateEffect.define<LineRange | null>();
const selectedLines = StateField.define<LineRange | null>({
  create: () => null,
  update(value, tr) {
    for (const effect of tr.effects)
      if (effect.is(selectLines)) value = effect.value;
    return value;
  },
});
function decorations(view: EditorView): DecorationSet {
  const range = view.state.field(selectedLines);
  if (!range) return Decoration.none;
  const marks = [];
  for (const visible of view.visibleRanges) {
    let line = view.state.doc.lineAt(visible.from);
    while (line.from <= visible.to) {
      if (line.number >= range.from && line.number <= range.to)
        marks.push(
          Decoration.line({ class: "cm-selected-line" }).range(line.from),
        );
      if (line.to >= view.state.doc.length) break;
      line = view.state.doc.lineAt(line.to + 1);
    }
  }
  return Decoration.set(marks, true);
}

// Line links belong to the current file URL. Replace only the hash, preserving
// the gallery's history state so returning to the list still works normally.
export function textLineSelection() {
  let anchor: number | null = null;
  let stopDrag = () => {};
  const apply = (view: EditorView, range: LineRange | null, scroll = false) => {
    view.dispatch({
      effects: [
        selectLines.of(range),
        ...(scroll && range
          ? [
              EditorView.scrollIntoView(view.state.doc.line(range.from).from, {
                y: "start",
                yMargin: 12,
              }),
            ]
          : []),
      ],
    });
  };
  const writeHash = (range: LineRange | null) => {
    const url = new URL(window.location.href);
    url.hash = range ? lineHash(range) : "";
    window.history.replaceState(window.history.state, "", url);
  };
  const choose = (view: EditorView, line: number) => {
    const range = {
      from: Math.min(anchor ?? line, line),
      to: Math.max(anchor ?? line, line),
    };
    const previous = view.state.field(selectedLines);
    if (previous?.from === range.from && previous.to === range.to) return;
    apply(view, range);
  };
  const plugin = ViewPlugin.fromClass(
    class {
      decorations: DecorationSet;
      restore: () => void;
      alive = true;
      constructor(readonly view: EditorView) {
        this.decorations = decorations(view);
        this.restore = () => {
          const range = parseLineHash(
            window.location.hash,
            view.state.doc.lines,
          );
          anchor = range?.from ?? null;
          apply(view, range, true);
        };
        window.addEventListener("hashchange", this.restore);
        window.addEventListener("popstate", this.restore);
        // Dispatch only after the editor has completed its initial construction.
        queueMicrotask(() => {
          if (this.alive) this.restore();
        });
      }
      update(update: ViewUpdate) {
        if (
          update.viewportChanged ||
          update.docChanged ||
          update.startState.field(selectedLines) !==
            update.state.field(selectedLines)
        )
          this.decorations = decorations(update.view);
      }
      destroy() {
        this.alive = false;
        stopDrag();
        window.removeEventListener("hashchange", this.restore);
        window.removeEventListener("popstate", this.restore);
      }
    },
    { decorations: (plugin) => plugin.decorations },
  );
  return [
    selectedLines,
    plugin,
    lineNumbers({
      domEventHandlers: {
        pointerdown(view, block, event) {
          const pointer = event as PointerEvent;
          if (pointer.button !== 0 || !pointer.isPrimary) return false;
          event.preventDefault();
          stopDrag();
          const line = view.state.doc.lineAt(block.from).number;
          const previous = view.state.field(selectedLines);
          const toggleOff =
            !pointer.shiftKey &&
            previous !== null &&
            line >= previous.from &&
            line <= previous.to;
          let dragged = false;
          if (!pointer.shiftKey || anchor === null) anchor = line;
          choose(view, line);
          writeHash(view.state.field(selectedLines)!);
          let y = pointer.clientY;
          let frame = 0;
          const moveToPointer = () => {
            const rect = view.scrollDOM.getBoundingClientRect();
            const offset =
              y < rect.top
                ? Math.max(-24, y - rect.top)
                : y > rect.bottom
                  ? Math.min(24, y - rect.bottom)
                  : 0;
            if (offset) view.scrollDOM.scrollTop += offset;
            const height = Math.max(
              0,
              Math.max(rect.top, Math.min(y, rect.bottom - 1)) -
                view.documentTop,
            );
            const block = view.lineBlockAtHeight(height);
            choose(view, view.state.doc.lineAt(block.from).number);
            frame = offset ? requestAnimationFrame(moveToPointer) : 0;
          };
          const move = (event: PointerEvent) => {
            if (event.pointerId !== pointer.pointerId) return;
            event.preventDefault();
            y = event.clientY;
            if (
              Math.abs(y - pointer.clientY) > 4 ||
              Math.abs(event.clientX - pointer.clientX) > 4
            )
              dragged = true;
            cancelAnimationFrame(frame);
            moveToPointer();
          };
          const finish = (event: PointerEvent) => {
            if (event.pointerId === pointer.pointerId) {
              if (event.type === "pointerup" && toggleOff && !dragged) {
                anchor = null;
                apply(view, null);
              }
              writeHash(view.state.field(selectedLines));
              stopDrag();
            }
          };
          stopDrag = () => {
            cancelAnimationFrame(frame);
            window.removeEventListener("pointermove", move);
            window.removeEventListener("pointerup", finish);
            window.removeEventListener("pointercancel", finish);
            window.removeEventListener("blur", stopDrag);
          };
          window.addEventListener("pointermove", move, { passive: false });
          window.addEventListener("pointerup", finish);
          window.addEventListener("pointercancel", finish);
          window.addEventListener("blur", stopDrag);
          return true;
        },
      },
    }),
    EditorView.theme({
      ".cm-lineNumbers": {
        cursor: "pointer",
        userSelect: "none",
        touchAction: "none",
      },
      ".cm-lineNumbers .cm-gutterElement:hover": {
        color: "var(--text)",
        backgroundColor: "var(--mantine-color-blue-light)",
      },
      ".cm-selected-line": {
        backgroundColor: "var(--mantine-color-yellow-light)",
      },
    }),
  ];
}
