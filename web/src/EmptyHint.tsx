import { Text } from "@mantine/core";
import type { ReactNode } from "react";

export function EmptyHint({ children }: { children: ReactNode }) {
  return (
    <Text size="sm" c="dimmed">
      {children}
    </Text>
  );
}
