import { Modal } from "@mantine/core";
import { useId, useState, type ReactNode } from "react";
import { useI18n } from "./i18n";
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
  const descriptionId = useId();
  const [opener] = useState(() =>
    document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null,
  );
  return (
    <Modal
      opened={open}
      transitionProps={{ transition: "fade", duration: 180 }}
      classNames={
        open
          ? { content: "dialog-enter", overlay: "dialog-overlay-enter" }
          : undefined
      }
      onClose={() => {
        onOpenChange(false);
        requestAnimationFrame(() => {
          if (opener?.isConnected) opener.focus();
        });
      }}
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
