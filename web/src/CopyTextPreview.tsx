import { CopyButton, Tooltip } from "@mantine/core";
import { useLayoutEffect, useRef, useState } from "react";
import { useI18n, type MessageKey } from "./i18n";

function parts(text: string) {
  return text.split(/(↵)/u);
}

// Measure the inline action together with the text so only the final line
// reserves space for copying. The stored and copied value remains untouched.
export function CopyTextPreview({
  text,
  copyLabel = "ocrCopyText",
}: {
  text: string;
  copyLabel?: MessageKey;
}) {
  const { t } = useI18n();
  const preview = text
    .trim()
    .replace(/\r\n|[\r\u2028\u2029]/gu, "\n")
    .replace(/[^\S\n]+/gu, " ")
    .replace(/ *\n(?: *\n)* */gu, "↵ ");
  const root = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(preview);
  const label = t("copy");
  useLayoutEffect(() => {
    const element = root.current;
    if (!element) return;
    let disposed = false;
    const measure = () => {
      if (disposed || !element.clientWidth) return;
      const clone = element.cloneNode(true) as HTMLSpanElement;
      clone.classList.add("copy-text-measure");
      clone.setAttribute("aria-hidden", "true");
      clone.style.width = `${element.clientWidth}px`;
      const button = clone.querySelector("button")!;
      button.tabIndex = -1;
      // Reserve enough room for both action states to avoid a fourth line.
      button.textContent = label;
      element.after(clone);
      const actionWidth = button.getBoundingClientRect().width;
      button.textContent = t("copied");
      button.style.minWidth = `${Math.max(actionWidth, button.getBoundingClientRect().width)}px`;
      button.textContent = label;
      const value = clone.querySelector("bdi")!;
      const lineHeight = parseFloat(getComputedStyle(element).lineHeight);
      const fits = (candidate: string) => {
        const content = document.createDocumentFragment();
        for (const part of parts(candidate)) {
          if (part !== "↵") content.append(document.createTextNode(part));
          else {
            const marker = document.createElement("span");
            marker.className = "text-break-marker";
            marker.textContent = part;
            content.append(marker);
          }
        }
        value.replaceChildren(content);
        return clone.getBoundingClientRect().height <= lineHeight * 3 + 0.5;
      };
      let result = preview;
      if (!fits(preview)) {
        const chars = Array.from(preview);
        let low = 0,
          high = chars.length;
        while (low < high) {
          const mid = Math.ceil((low + high) / 2);
          if (fits(chars.slice(0, mid).join("").trimEnd() + "…")) low = mid;
          else high = mid - 1;
        }
        result = chars.slice(0, low).join("").trimEnd() + "…";
      }
      clone.remove();
      setVisible(result);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    void document.fonts?.ready?.then(measure);
    return () => {
      disposed = true;
      observer.disconnect();
    };
  }, [preview, label, t]);
  return (
    <Tooltip
      label={text}
      multiline
      interactive
      position="bottom-start"
      offset={8}
      classNames={{ tooltip: "metadata-tooltip metadata-tooltip--text" }}
      openDelay={0}
      closeDelay={100}
      transitionProps={{ duration: 0 }}
      events={{ hover: true, focus: true, touch: true }}
    >
      <span className="copy-text-preview" ref={root} tabIndex={0}>
        <bdi>
          {parts(visible).map((part, index) =>
            part === "↵" ? (
              <span className="text-break-marker" key={index}>
                {part}
              </span>
            ) : (
              part
            ),
          )}
        </bdi>
        <CopyButton value={text}>
          {({ copied, copy }) => (
            <button
              type="button"
              onClick={copy}
              aria-label={t(copied ? "copied" : copyLabel)}
            >
              {t(copied ? "copied" : "copy")}
            </button>
          )}
        </CopyButton>
      </span>
    </Tooltip>
  );
}
