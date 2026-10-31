import { Group, Table, Text } from "@mantine/core";
import type { ReactNode } from "react";
import type { components } from "./api-schema";
import { ConnectionTest } from "./ConnectionTest";
import { useI18n, type MessageKey } from "./i18n";

type Probe = components["schemas"]["ProxyTestResult"];
const reasons: Record<Exclude<Probe["error"], "">, MessageKey> = {
  connection: "aiProxyTestConnection",
  timeout: "aiProxyTestTimeout",
  rate_limited: "aiProxyTestRateLimited",
  service: "aiProxyTestService",
};

// Results belong to the saved configuration snapshot, not the archive or draft.
export function AIProxyRow({
  number,
  name,
  type,
  address,
  revision,
  saved,
  disabled,
  children,
}: {
  number: number;
  name: string;
  type: string;
  address: string;
  revision: string;
  saved: boolean;
  disabled: boolean;
  children: ReactNode;
}) {
  const { t } = useI18n();
  return (
    <ConnectionTest<Probe>
      path={`/ai/proxies/${number}/test`}
      revision={revision}
      saved={saved}
      disabled={disabled}
      hint={t("aiProxyTestHint")}
      saveHint={t("aiProxyTestSaveFirst")}
      reasons={reasons}
      success={(probe) =>
        t("aiProxyTestResult", {
          ip: probe.ip,
          location:
            [
              ...new Set(
                [probe.country, probe.region, probe.city].filter(Boolean),
              ),
            ].join(" · ") || t("aiProxyTestUnknownLocation"),
          ms: probe.elapsed_ms,
        })
      }
    >
      {({ action, feedback, testing }) => (
        <>
          <Table.Td className="settings-number-column">
            {number > 0 ? number : t("aiPendingNumber")}
          </Table.Td>
          <Table.Th scope="row">{name}</Table.Th>
          <Table.Td>{type}</Table.Td>
          <Table.Td>
            <Text size="sm" truncate>
              {address}
            </Text>
            {feedback}
          </Table.Td>
          <Table.Td>
            <Group
              gap="md"
              justify="flex-end"
              wrap="nowrap"
              className="table-row-actions"
              data-busy={testing || undefined}
            >
              {action}
              {children}
            </Group>
          </Table.Td>
        </>
      )}
    </ConnectionTest>
  );
}
