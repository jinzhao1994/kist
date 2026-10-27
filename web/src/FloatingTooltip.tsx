import { Tooltip, type TooltipFloatingProps } from "@mantine/core";
import {
  cloneElement,
  isValidElement,
  useEffect,
  useRef,
  useState,
  type MouseEvent,
} from "react";

type HoverHandlers = {
  onMouseEnter?: (event: MouseEvent<HTMLElement>) => void;
  onMouseLeave?: (event: MouseEvent<HTMLElement>) => void;
};

// A keyed target can move during an async update without emitting mouseleave.
// Keep Mantine's positioning and styling, but also dismiss from outside the target.
export function FloatingTooltip({
  children,
  disabled,
  ...props
}: Omit<TooltipFloatingProps, "defaultOpened">) {
  const target = useRef<HTMLDivElement>(null);
  const [hovered, setHovered] = useState(false);
  useEffect(() => {
    if (!hovered || disabled) return;
    const close = () => setHovered(false);
    const move = (event: globalThis.MouseEvent) => {
      if (
        !(event.target instanceof Node) ||
        !target.current?.contains(event.target)
      ) {
        close();
      }
    };
    const leaveWindow = (event: globalThis.MouseEvent) => {
      if (!event.relatedTarget) close();
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        close();
      }
    };
    document.addEventListener("mousemove", move, true);
    document.addEventListener("mouseout", leaveWindow, true);
    document.addEventListener("pointerdown", close, true);
    document.addEventListener("keydown", key, true);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    window.addEventListener("blur", close);
    return () => {
      document.removeEventListener("mousemove", move, true);
      document.removeEventListener("mouseout", leaveWindow, true);
      document.removeEventListener("pointerdown", close, true);
      document.removeEventListener("keydown", key, true);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
      window.removeEventListener("blur", close);
    };
  }, [hovered, disabled]);
  if (!isValidElement<HoverHandlers>(children)) {
    return null;
  }
  return (
    <Tooltip.Floating {...props} ref={target} disabled={disabled || !hovered}>
      {cloneElement(children, {
        onMouseEnter(event) {
          children.props.onMouseEnter?.(event);
          setHovered(true);
        },
        onMouseLeave(event) {
          children.props.onMouseLeave?.(event);
          setHovered(false);
        },
      })}
    </Tooltip.Floating>
  );
}
