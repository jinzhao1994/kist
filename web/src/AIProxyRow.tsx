import { Anchor, Group, Table, Text } from "@mantine/core";
import { useEffect, useRef, useState, type ReactNode } from "react";
import type { components } from "./api-schema";
import { result } from "./api";
import { TaskError, TaskStatus } from "./TaskStatus";
import { errorMessage } from "./mutations";
import { useI18n, type MessageKey } from "./i18n";
import { FullTextTooltip } from "./FullTextTooltip";

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
  const [probe, setProbe] = useState<Probe>();
  const [error, setError] = useState("");
  const [testing, setTesting] = useState(false);
  const active = useRef<AbortController | undefined>(undefined);
  useEffect(() => {
    setProbe(undefined);
    setError("");
    setTesting(false);
    return () => active.current?.abort();
  }, [revision, saved]);
  async function test() {
    if (active.current || disabled || !saved) return;
    const controller = new AbortController();
    active.current = controller;
    setTesting(true);
    setProbe(undefined);
    setError("");
    try {
      const value = await result<Probe>(
        await fetch(`/api/v1/ai/proxies/${number}/test`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
          signal: controller.signal,
        }),
      );
      if (!controller.signal.aborted) setProbe(value);
    } catch (error) {
      if (!controller.signal.aborted) setError(errorMessage(error, t));
    } finally {
      if (active.current === controller) active.current = undefined;
      if (!controller.signal.aborted) setTesting(false);
    }
  }
  const location = probe
    ? [
        ...new Set([probe.country, probe.region, probe.city].filter(Boolean)),
      ].join(" · ") || t("aiProxyTestUnknownLocation")
    : "";
  return (
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
        {testing && <TaskStatus>{t("aiProxyTesting")}</TaskStatus>}
        {saved && probe && !probe.error && (
          <TaskStatus>
            {t("aiProxyTestResult", {
              ip: probe.ip,
              location,
              ms: probe.elapsed_ms,
            })}
          </TaskStatus>
        )}
        {saved && probe?.error && (
          <TaskError>{t(reasons[probe.error])}</TaskError>
        )}
        {saved && error && <TaskError>{error}</TaskError>}
      </Table.Td>
      <Table.Td>
        <Group
          gap="md"
          justify="flex-end"
          wrap="nowrap"
          className="table-row-actions"
        >
          <FullTextTooltip
            text={t(saved ? "aiProxyTestHint" : "aiProxyTestSaveFirst")}
          >
            <span tabIndex={!saved ? 0 : undefined}>
              <Anchor
                component="button"
                type="button"
                className="text-action-link"
                size="xs"
                disabled={disabled || !saved || testing}
                onClick={test}
              >
                {t(testing ? "aiProxyTesting" : "aiProxyTest")}
              </Anchor>
            </span>
          </FullTextTooltip>
          {children}
        </Group>
      </Table.Td>
    </>
  );
}
