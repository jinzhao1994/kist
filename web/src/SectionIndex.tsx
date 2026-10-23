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
  useEffect(() => {
    if (!ready) return;
    let frame = 0;
    const restore = () => {
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
    // Sections mount after their data loads, too late for native fragment scrolling.
    restore();
    window.addEventListener("hashchange", restore);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("hashchange", restore);
    };
  }, [ready]);
  return (
    <div className="section-layout">
      <nav className="section-index" aria-label={t("pageContents")}>
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
