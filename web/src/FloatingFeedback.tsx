import { Alert, Portal } from "@mantine/core";
import { useI18n } from "./i18n";

// Persist until dismissed; scrolling must not hide an operation failure.
export function FloatingFeedback({
  message,
  onClose,
}: {
  message: string;
  onClose: () => void;
}) {
  const { t } = useI18n();
  return (
    <Portal>
      <Alert
        className="floating-feedback"
        role="alert"
        color="red"
        withCloseButton
        closeButtonLabel={t("close")}
        onClose={onClose}
      >
        {message}
      </Alert>
    </Portal>
  );
}
