import { Collapse, useMantineEnv } from "@mantine/core";
import { useReducedMotion } from "@mantine/hooks";
import { useEffect, useRef, useState, type ReactNode } from "react";

// Retain disappearing content until its measured height has finished closing.
export function InlineReveal({
  children,
  onEntered,
  onExited,
}: {
  children: ReactNode;
  onEntered?: () => void;
  onExited?: () => void;
}) {
  const reduced = useReducedMotion();
  const immediate = useMantineEnv() === "test" || reduced;
  const opened = !!children;
  const wasOpen = useRef(false);
  const [retained, setRetained] = useState(children);
  const [expanded, setExpanded] = useState(false);
  if (children && children !== retained) setRetained(children);
  useEffect(() => {
    const previous = wasOpen.current;
    wasOpen.current = opened;
    if (immediate) {
      setExpanded(opened);
      if (opened) onEntered?.();
      else {
        setRetained(null);
        if (previous) onExited?.();
      }
      return;
    }
    const frame = requestAnimationFrame(() => setExpanded(opened));
    return () => cancelAnimationFrame(frame);
  }, [opened, immediate, onEntered, onExited]);
  if (immediate) return children;
  if (!children && !retained) return null;
  return (
    <Collapse
      expanded={expanded}
      keepMounted={false}
      transitionDuration={180}
      transitionTimingFunction="cubic-bezier(0.2, 0.8, 0.2, 1)"
      onTransitionEnd={() => {
        if (opened) onEntered?.();
        else {
          setRetained(null);
          onExited?.();
        }
      }}
    >
      <div inert={!opened} aria-hidden={!opened}>
        {children || retained}
      </div>
    </Collapse>
  );
}
