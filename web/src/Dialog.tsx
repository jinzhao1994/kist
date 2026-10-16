import { useReducedMotion } from "@mantine/hooks";
import { Modal, useMantineEnv } from "@mantine/core";
import {
  createContext,
  useContext,
  useEffect,
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
  // Mount closed first so dialogs created by an async result also enter through
  // Mantine's transition lifecycle instead of starting in the entered state.
  const [mounted, setMounted] = useState(env === "test" || reducedMotion);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setMounted(true));
    return () => cancelAnimationFrame(frame);
  }, []);
  const visible = !!mounted && open && (presence?.present ?? true);
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
        transition: {
          in: { opacity: 1, transform: "translateY(0) scale(1)" },
          out: { opacity: 0, transform: "translateY(6px) scale(0.98)" },
          common: { transformOrigin: "center" },
          transitionProperty: "opacity, transform",
        },
        timingFunction: "cubic-bezier(0.2, 0.8, 0.2, 1)",
        duration: reducedMotion || env === "test" ? 0 : 180,
      }}
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
