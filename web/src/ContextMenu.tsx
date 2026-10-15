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
    setMenu({ item, x: rect.left, y: rect.bottom });
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
  return (
    <Menu
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
              left: anchor?.x ?? 0,
              top: anchor?.y ?? 0,
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
