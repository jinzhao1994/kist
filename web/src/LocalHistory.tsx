import { EmptyHint } from "./EmptyHint";
import { Button, Group, Stack, Text, UnstyledButton } from "@mantine/core";
import { useEffect, useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { createWrite, get } from "./api";
import type { components } from "./client-api-schema";
import { describeHistory } from "./History";
import { formatInstant, useI18n, type Translate } from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";

type Entry = components["schemas"]["ClientLocalHistoryEntry"];
type Detail = components["schemas"]["ClientLocalHistoryDetail"];
type Page = { entries: Entry[]; before?: string };
function stateLabel(state: Entry["state"], t: Translate) {
  const labels = {
    pending: "clientHistoryPending",
    awaiting_receipt: "clientHistoryAwaiting",
    awaiting_root: "clientHistoryAwaitingRoot",
    conflict: "clientHistoryConflict",
    accepted: "clientHistoryAccepted",
    discarded: "clientHistoryDiscarded",
    replaced: "clientHistoryReplaced",
  } as const;
  return t(labels[state]);
}

export function LocalHistory({
  active,
  onLocked,
}: {
  active: boolean;
  onLocked: (locked: boolean) => void;
}) {
  const { t, prefs } = useI18n();
  const [selected, setSelected] = useState<string>();
  const [confirm, setConfirm] = useState(false);
  const writes = useWrites(() => setConfirm(false));
  useEffect(() => {
    onLocked(writes.busy || writes.locked);
  }, [writes.busy, writes.locked, onLocked]);
  const list = useInfiniteQuery({
    queryKey: ["client", "history"],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) =>
      get<Page>(
        `/client/history?limit=25&before=${encodeURIComponent(pageParam)}`,
        signal,
      ),
    getNextPageParam: (page) => page.before || undefined,
    enabled: active,
  });
  const detail = useQuery({
    queryKey: ["client", "history", selected],
    queryFn: ({ signal }) =>
      get<Detail>(`/client/history/${encodeURIComponent(selected!)}`, signal),
    enabled: active && !!selected,
  });
  function select(id?: string) {
    if (writes.busy || writes.locked) return;
    setSelected(id);
    setConfirm(false);
  }
  const op = detail.data;
  const error = selected ? detail.error : list.error;
  return (
    <Stack gap="md">
      {selected && (
        <Group>
          <Button
            variant="subtle"
            disabled={writes.busy || writes.locked}
            onClick={() => select()}
          >
            {t("clientHistoryBack")}
          </Button>
        </Group>
      )}
      {error && (
        <div role="alert">
          <p>{errorMessage(error, t)}</p>
          <Button
            onClick={() => (selected ? detail.refetch() : list.refetch())}
          >
            {t("retry")}
          </Button>
        </div>
      )}
      {(selected ? detail.isPending : list.isPending) && (
        <Text role="status">{t("loading")}</Text>
      )}
      {!selected && (
        <>
          {list.data?.pages[0]?.entries.length === 0 && (
            <EmptyHint>{t("clientHistoryEmpty")}</EmptyHint>
          )}
          {list.data?.pages
            .flatMap((page) => page.entries)
            .map((entry) => (
              <Stack key={entry.id} gap="xs" className="client-pending-item">
                <Group justify="space-between">
                  <UnstyledButton
                    className="text-button"
                    style={{
                      flex: "1 1 12rem",
                      minWidth: 0,
                      overflowWrap: "anywhere",
                      textAlign: "start",
                    }}
                    onClick={() => select(entry.id)}
                  >
                    {entry.action === "undo"
                      ? t("undo")
                      : describeHistory(entry.action, entry.summary, t)}
                  </UnstyledButton>
                  <Text size="sm">{stateLabel(entry.state, t)}</Text>
                </Group>
                {entry.summary?.current_names && (
                  <Text size="sm" c="dimmed">
                    {t("historyCurrentNames")}
                  </Text>
                )}
                <Text size="sm" c="dimmed">
                  {formatInstant(entry.occurred_at, prefs)}
                </Text>
                {entry.receipt && (
                  <Text size="sm" c="dimmed">
                    {t("clientFormalOperation", { id: entry.receipt.op_id })}
                  </Text>
                )}
              </Stack>
            ))}
          {list.hasNextPage && (
            <Button
              loading={list.isFetchingNextPage}
              onClick={() => list.fetchNextPage()}
            >
              {t("loadMore")}
            </Button>
          )}
        </>
      )}
      {selected && op && (
        <>
          <Text fw={600}>
            {op.action === "undo"
              ? t("undo")
              : describeHistory(op.action, op.summary, t)}
          </Text>
          {op.summary?.current_names && (
            <Text size="sm" c="dimmed">
              {t("historyCurrentNames")}
            </Text>
          )}
          <Text size="sm">
            {stateLabel(op.state, t)} · {formatInstant(op.occurred_at, prefs)}
          </Text>
          <Text size="xs" c="dimmed" style={{ overflowWrap: "anywhere" }}>
            {op.id}
          </Text>
          {op.receipt && (
            <Text size="sm">
              {t("clientFormalOperation", { id: op.receipt.op_id })} ·{" "}
              {t("historyAcceptedAt")} ·{" "}
              {formatInstant(op.receipt.accepted_at, prefs)}
            </Text>
          )}
          {op.reason && (
            <Text size="sm" style={{ overflowWrap: "anywhere" }}>
              {op.reason}
            </Text>
          )}
          {(
            [
              ["clientHistoryUndoTarget", op.undo],
              ["clientHistoryGroup", op.group],
            ] as const
          ).map(
            ([label, reference]) =>
              reference && (
                <Group key={label}>
                  <Text size="sm">{t(label)}</Text>
                  {reference.local_id ? (
                    <Button
                      variant="subtle"
                      disabled={writes.busy || writes.locked}
                      onClick={() => select(reference.local_id)}
                    >
                      {t("clientHistoryRelated")}
                    </Button>
                  ) : (
                    <Text size="sm">
                      {t("clientFormalOperation", { id: reference.op_id || 0 })}
                    </Text>
                  )}
                </Group>
              ),
          )}
          {op.replaced_by && (
            <Group>
              <Button
                variant="subtle"
                disabled={writes.busy || writes.locked}
                onClick={() => select(op.replaced_by)}
              >
                {t("clientHistoryReplacement")}
              </Button>
            </Group>
          )}
          {(op.state === "pending" || op.state === "accepted") && (
            <>
              {confirm && <Text>{t("undoConfirm")}</Text>}
              <Group>
                <Button
                  disabled={writes.busy || writes.locked}
                  onClick={() =>
                    confirm
                      ? writes.run([
                          createWrite(
                            "POST",
                            `/client/history/${encodeURIComponent(op.id)}/undo`,
                            {},
                          ),
                        ])
                      : setConfirm(true)
                  }
                >
                  {t(writes.busy ? "saving" : "undo")}
                </Button>
                {confirm && (
                  <Button
                    disabled={writes.busy || writes.locked}
                    onClick={() => setConfirm(false)}
                  >
                    {t("cancel")}
                  </Button>
                )}
              </Group>
            </>
          )}
          <WriteFeedback state={writes} />
        </>
      )}
    </Stack>
  );
}
