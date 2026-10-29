import { Anchor } from "@mantine/core";
import { useEffect, useRef, type ReactNode } from "react";
import { useI18n } from "./i18n";

export function SectionIndex({
  items,
  children,
  ready = true,
}: {
  items: { id: string; label: string }[];
  children: ReactNode;
  ready?: boolean;
}) {
  const { t } = useI18n();
  const body = useRef<HTMLDivElement>(null);
  const navigate = useRef<(() => void) | undefined>(undefined);
  useEffect(() => {
    if (!ready) return;
    let frame = 0;
    let following = true;
    const restore = () => {
      if (!following) return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        let id: string;
        try {
          id = decodeURIComponent(window.location.hash.slice(1));
        } catch {
          return;
        }
        const target = document.getElementById(id);
        if (target && body.current?.contains(target))
          target.scrollIntoView({ block: "start", behavior: "instant" });
      });
    };
    // Async sections above the target can move it after native fragment
    // scrolling. Follow layout changes, but yield permanently to user input
    // until the next explicit index/fragment navigation.
    const observer = new ResizeObserver(restore);
    const resume = () => {
      following = true;
      if (body.current) observer.observe(body.current);
      restore();
    };
    const stop = () => {
      following = false;
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
    navigate.current = resume;
    resume();
    window.addEventListener("hashchange", resume);
    const inputs = ["wheel", "touchstart", "pointerdown", "keydown"] as const;
    for (const event of inputs)
      window.addEventListener(event, stop, { capture: true, passive: true });
    return () => {
      stop();
      navigate.current = undefined;
      window.removeEventListener("hashchange", resume);
      for (const event of inputs) window.removeEventListener(event, stop, true);
    };
  }, [ready]);
  return (
    <div className="section-layout">
      <nav
        className="section-index"
        aria-label={t("pageContents")}
        onClick={() => navigate.current?.()}
      >
        {items.map(({ id, label }) => (
          <Anchor key={id} href={`#${id}`} size="sm">
            {label}
          </Anchor>
        ))}
      </nav>
      <div ref={body} className="section-body">
        {children}
      </div>
    </div>
  );
}
