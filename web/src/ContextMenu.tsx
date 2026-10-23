import { Menu, Portal } from "@mantine/core";
import {
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
} from "react";

export function useContextMenu<T>() {
  const [menu, setMenu] = useState<{ item: T; x: number; y: number } | null>(
    null,
  );
  const close = () => setMenu(null);
  const atElement = (item: T, target: HTMLElement) => {
    const rect = target.getBoundingClientRect();
    // A fractional fixed anchor can rasterize text on adjacent pixel rows
    // when the fade layer is composited and then removed.
    const scale = window.devicePixelRatio || 1;
    const snap = (value: number) => Math.round(value * scale) / scale;
    setMenu({ item, x: snap(rect.left), y: snap(rect.bottom) });
  };
  return {
    menu,
    close,
    atElement,
    bindings: (item: T) => ({
      onContextMenu: (event: MouseEvent<HTMLElement>) => {
        event.preventDefault();
        event.stopPropagation();
        setMenu({ item, x: event.clientX, y: event.clientY });
      },
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
        if (
          event.key === "ContextMenu" ||
          (event.shiftKey && event.key === "F10")
        ) {
          event.preventDefault();
          event.stopPropagation();
          atElement(item, event.currentTarget);
        }
      },
    }),
  };
}

export function ContextMenu({
  anchor,
  close,
  children,
}: {
  anchor: { x: number; y: number } | null;
  close: () => void;
  children: ReactNode;
}) {
  // Keep the exit frame anchored to the last trigger, not the viewport origin.
  const [position, setPosition] = useState(anchor);
  if (anchor && (anchor.x !== position?.x || anchor.y !== position?.y)) {
    setPosition(anchor);
  }
  return (
    <Menu
      transitionProps={{ transition: "fade", duration: 100, exitDuration: 0 }}
      returnFocus={false}
      withinPortal
      position="bottom-start"
      floatingStrategy="fixed"
      offset={0}
      opened={anchor !== null}
      onChange={(open) => {
        if (!open) close();
      }}
      styles={{ item: { fontSize: "var(--mantine-font-size-xs)" } }}
    >
      <Portal>
        <Menu.Target>
          <span
            aria-hidden="true"
            style={{
              position: "fixed",
              left: position?.x ?? 0,
              top: position?.y ?? 0,
              width: 1,
              height: 1,
              pointerEvents: "none",
            }}
          />
        </Menu.Target>
      </Portal>
      <Menu.Dropdown
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => event.stopPropagation()}
      >
        {children}
      </Menu.Dropdown>
    </Menu>
  );
}
