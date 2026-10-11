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
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
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
      onClose={() => {
        onOpenChange(false);
        requestAnimationFrame(() => {
          if (opener?.isConnected) opener.focus();
        });
      }}
      title={<bdi>{title}</bdi>}
      size={
        className === "detail-dialog" ? "min(1100px, calc(100vw - 2rem))" : "md"
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
