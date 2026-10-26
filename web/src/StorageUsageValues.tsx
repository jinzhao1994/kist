import { Text } from "@mantine/core";
import { formatBytes, useI18n, type MessageKey } from "./i18n";

// Storage summaries share spacing and typography across settings and sync.
export function StorageUsageValues({
  items,
}: {
  items: readonly (readonly [MessageKey, number | undefined])[];
}) {
  const { t, prefs } = useI18n();
  return (
    <div className="storage-usage-values">
      {items.map(([label, bytes]) => (
        <div key={label}>
          <Text size="xs" c="dimmed">
            {t(label)}
          </Text>
          <Text>{bytes == null ? "—" : formatBytes(bytes, prefs)}</Text>
        </div>
      ))}
    </div>
  );
}
