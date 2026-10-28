import { Anchor, Text } from "@mantine/core";
import type { ReactNode } from "react";
import type { Translate } from "./i18n";

export function TaskStatus({
  children,
  settings,
}: {
  children: ReactNode;
  settings?: { href: string; label: string };
}) {
  return (
    <Text size="xs" c="dimmed" role="status">
      {children}
      {settings && (
        <>
          {" "}
          <Anchor href={settings.href} inherit>
            {settings.label}
          </Anchor>
        </>
      )}
    </Text>
  );
}

export function taskErrorMessage(message: string | undefined, t: Translate) {
  // Queue records may outlive the process that was canceled.
  return message === "context canceled" ? t("taskCanceled") : message;
}
