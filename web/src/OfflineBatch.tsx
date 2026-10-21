import { Button, Group, Stack, Text } from "@mantine/core";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";
import {
  offlineOwner,
  retainOffline,
  prepareOffline,
  type OfflineTarget,
} from "./offline-retention";

export function OfflineBatch({
  targets,
  close,
}: {
  targets: OfflineTarget[];
  close: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [retained, setRetained] = useState(0);
  const [prepared, setPrepared] = useState(0);
  const [current, setCurrent] = useState("");
  const [error, setError] = useState("");
  const [interrupted, setInterrupted] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function run() {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError("");
    setInterrupted(false);
    setRetained(0);
    setPrepared(0);
    try {
      // Persist every selection before transferring any content. Each owner is
      // independently retryable and manageable in the cache panel.
      for (const [index, target] of targets.entries()) {
        controller.signal.throwIfAborted();
        setCurrent(target.name);
        await retainOffline(target, controller.signal);
        setRetained(index + 1);
      }
      await queries.invalidateQueries({ queryKey: ["client", "offline"] });
      for (const [index, target] of targets.entries()) {
        controller.signal.throwIfAborted();
        setCurrent(target.name);
        const result = await prepareOffline(target, controller.signal);
        if (!result.ready) {
          setError(t("clientBatchChanged"));
          return;
        }
        setPrepared(index + 1);
      }
      setCurrent("");
    } catch (err) {
      if (controller.signal.aborted) setInterrupted(true);
      else setError(errorMessage(err, t));
    } finally {
      active.current = null;
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["client"] });
    }
  }
  return (
    <Dialog
      open
      size="md"
      title={t("clientKeepOffline")}
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <Stack gap="md">
        <Text size="sm" c="dimmed">
          {t("clientBatchHint")}
        </Text>
        <Stack gap="xs" mah={200} style={{ overflowY: "auto" }}>
          {targets.map((target) => (
            <Text
              key={offlineOwner(target)}
              style={{ overflowWrap: "anywhere" }}
            >
              <bdi>{target.name}</bdi>
              {target.fileset && (
                <Text component="span" size="xs" c="dimmed">
                  {" "}
                  · {t("filesetLabel")}
                </Text>
              )}
            </Text>
          ))}
        </Stack>
        <Text role="status">
          {t("clientBatchProgress", {
            retained,
            prepared,
            total: targets.length,
          })}
        </Text>
        {current && (
          <Text size="sm" style={{ overflowWrap: "anywhere" }}>
            <bdi>{current}</bdi>
          </Text>
        )}
        {busy && (
          <Text size="sm" c="dimmed">
            {t("clientPreparing")}
          </Text>
        )}
        {interrupted && <Text size="sm">{t("clientBatchInterrupted")}</Text>}
        {error && <Text role="alert">{error}</Text>}
        <Text size="xs" c="dimmed">
          {t("clientRetentionScope")}
        </Text>
        <Group justify="flex-end">
          {busy ? (
            <Button onClick={() => active.current?.abort()}>
              {t("cancel")}
            </Button>
          ) : (
            <Button onClick={run} disabled={!targets.length}>
              {t(retained ? "clientPrepareAgain" : "clientKeepAndPrepare")}
            </Button>
          )}
          <Button disabled={busy} onClick={close}>
            {t("close")}
          </Button>
        </Group>
      </Stack>
    </Dialog>
  );
}
