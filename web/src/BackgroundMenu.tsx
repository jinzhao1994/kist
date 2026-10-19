import { useEffect, useRef, useState, type ReactNode } from "react";
import { ContextMenu } from "./ContextMenu";

// Only the page background participates; item menus and native controls keep
// their own context menus, including controls rendered through portals.
const controls =
  'button, a, input, textarea, select, [contenteditable], [role="button"], [role="dialog"], [role="menu"], [data-item-id], header, .filters-bar, .upload-panel';

export function BackgroundMenu({ children }: { children: ReactNode }) {
  const marker = useRef<HTMLSpanElement>(null);
  const [anchor, setAnchor] = useState<{ x: number; y: number } | null>(null);
  useEffect(() => {
    const main = marker.current?.closest("main");
    if (!main) return;
    const close = () => setAnchor(null);
    const context = (event: MouseEvent) => {
      if (!(event.target instanceof Element)) return;
      if (event.target.closest(controls)) {
        close();
        return;
      }
      event.preventDefault();
      setAnchor({ x: event.clientX, y: event.clientY });
    };
    main.addEventListener("contextmenu", context);
    main.addEventListener("scroll", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close);
    return () => {
      main.removeEventListener("contextmenu", context);
      main.removeEventListener("scroll", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close);
    };
  }, []);
  return (
    <>
      <span ref={marker} hidden />
      <ContextMenu anchor={anchor} close={() => setAnchor(null)}>
        {children}
      </ContextMenu>
    </>
  );
}
