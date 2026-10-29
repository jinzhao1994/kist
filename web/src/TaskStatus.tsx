import { Text } from "@mantine/core";
import type { ReactNode } from "react";
import type { Translate } from "./i18n";

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

export function TaskError({ children }: { children: ReactNode }) {
  return (
    <Text size="xs" c="red" role="alert">
      {children}
    </Text>
  );
}

// Idle jobs are refreshed by the shared catalog revision observer. Keep polling
// only while there is work, and back off after a lost status response.
export function taskPollInterval(query: {
  state: { data?: { state: string }; error: unknown };
}) {
  const state = query.state.data?.state;
  if (state !== "queued" && state !== "running") return false;
  return query.state.error ? 15000 : 2000;
}
