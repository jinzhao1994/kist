import { useReducedMotion } from "@mantine/hooks";
import { Modal, useMantineEnv } from "@mantine/core";
import {
  createContext,
  useContext,
  useId,
  useState,
  type ReactNode,
} from "react";
import { useI18n } from "./i18n";
// Preserve the mounted form until Modal finishes its exit transition.
const Presence = createContext<{ present: boolean; exited: () => void } | null>(
  null,
);
export function DialogPresence({ children }: { children: ReactNode }) {
  const [retained, setRetained] = useState(children);
  if (children && children !== retained) setRetained(children);
  return (
    <Presence
      value={{
        present: !!children,
        exited: () => {
          if (!children) setRetained(null);
        },
      }}
    >
      {children || retained}
    </Presence>
  );
}
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  className = "",
  size,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
  size?: "sm" | "md";
}) {
  const { t } = useI18n();
  const presence = useContext(Presence);
  const env = useMantineEnv();
  const reducedMotion = useReducedMotion();
  const visible = open && (presence?.present ?? true);
  const descriptionId = useId();
  const [opener] = useState(() =>
    document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null,
  );
  return (
    <Modal
      opened={visible}
      onExitTransitionEnd={() => {
        presence?.exited();
        requestAnimationFrame(() => {
          if (opener?.isConnected) opener.focus();
        });
      }}
      transitionProps={{
        transition: "fade",
        duration: reducedMotion || env === "test" ? 0 : 180,
      }}
      classNames={
        visible
          ? { content: "dialog-enter", overlay: "dialog-overlay-enter" }
          : undefined
      }
      onClose={() => onOpenChange(false)}
      title={<bdi>{title}</bdi>}
      size={
        size ??
        (className === "detail-dialog"
          ? "min(1100px, calc(100vw - 2rem))"
          : "md")
      }
      centered
      closeButtonProps={{ "aria-label": t("close") }}
      aria-describedby={description ? descriptionId : undefined}
    >
      {description && <p id={descriptionId}>{description}</p>}
      {children}
    </Modal>
  );
}
