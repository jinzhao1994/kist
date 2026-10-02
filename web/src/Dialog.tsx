import * as Primitive from "@radix-ui/react-dialog";
import { useState, type ReactNode } from "react";
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
  const [opener] = useState(() =>
    document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null,
  );
  return (
    <Primitive.Root open={open} onOpenChange={onOpenChange}>
      <Primitive.Portal>
        <Primitive.Overlay className="dialog-overlay" />
        <Primitive.Content
          className={`dialog ${className}`}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            opener?.focus();
          }}
        >
          <div className="dialog-heading">
            <Primitive.Title>
              <bdi>{title}</bdi>
            </Primitive.Title>
            <Primitive.Close className="icon-button" aria-label={t("close")}>
              ×
            </Primitive.Close>
          </div>
          <Primitive.Description className={description ? "" : "sr-only"}>
            {description || title}
          </Primitive.Description>
          {children}
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  );
}
