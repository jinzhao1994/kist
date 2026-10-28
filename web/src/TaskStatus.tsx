import { Text } from "@mantine/core";
import type { ReactNode } from "react";
import type { Translate, MessageKey } from "./i18n";

export function TaskStatus({ children }: { children: ReactNode }) {
  return (
    <Text size="xs" c="dimmed" role="status">
      {children}
    </Text>
  );
}

export function taskErrorMessage(message: string | undefined, t: Translate) {
  // Queue records may outlive the process that was canceled.
  return message === "context canceled" ? t("taskCanceled") : message;
}

export function taskRunningMessage(kind: string | undefined): MessageKey {
  if (kind === "desc") return "descGenerating";
  if (kind === "ocr") return "ocrRecognizing";
  if (kind === "orientation") return "orientationChecking";
  return "extracting";
}
