import { useEffect, useState } from "react";
import { Button, UnstyledButton } from "@mantine/core";
import { useInfiniteQuery } from "@tanstack/react-query";
import { get } from "./api";
import type { components } from "./client-api-schema";
import { LocalHistory, stateLabel } from "./LocalHistory";
import { describeHistory } from "./History";
import { Dialog, DialogPresence } from "./Dialog";
import { NavigationIcon } from "./NavigationIcon";
import { formatInstant, useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Entry = components["schemas"]["ClientLocalHistoryEntry"];
export function PendingHistory({
  box,
  file,
  fileset,
  trash,
  onEmpty,
}: {
  box?: string;
  file?: string;
  fileset?: string;
  trash?: boolean;
  onEmpty: (empty: boolean) => void;
}) {
  const { t, prefs } = useI18n();
  const [selected, setSelected] = useState<string>();
  const [locked, setLocked] = useState(false);
  const list = useInfiniteQuery({
    queryKey: ["client", "pending-history", { box, file, fileset, trash }],
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) => {
      const params = new URLSearchParams({ before: pageParam, limit: "50" });
      if (box) params.set("box", box);
      if (file) params.set("file", file);
      if (fileset) params.set("fileset", fileset);
      if (trash) params.set("trash", "true");
      return get<{ entries: Entry[]; before?: string }>(
        `/client/pending-history?${params}`,
        signal,
      );
    },
    getNextPageParam: (page) => page.before || undefined,
  });
  const entries = list.data?.pages.flatMap((page) => page.entries) || [];
  useEffect(
    () => onEmpty(list.isSuccess && entries.length === 0),
    [onEmpty, list.isSuccess, entries.length],
  );
  return (
    <>
      {list.isPending && <p role="status">{t("loading")}</p>}
      {list.error && (
        <div role="alert">
          <p>{errorMessage(list.error, t)}</p>
          <Button onClick={() => list.refetch()}>{t("retry")}</Button>
        </div>
      )}
      {entries.length > 0 && (
        <ol className="history-list">
          {entries.map((entry) => (
            <li key={entry.id}>
              <UnstyledButton
                className="history-entry"
                onClick={() => setSelected(entry.id)}
              >
                <NavigationIcon kind={trash ? "trash" : "history"} />
                <span>
                  <strong className="history-description">
                    <span>
                      {entry.action === "undo"
                        ? t("undo")
                        : describeHistory(entry.action, entry.summary, t)}
                    </span>
                  </strong>
                  <small>{formatInstant(entry.occurred_at, prefs)}</small>
                </span>
                <span className="history-status">
                  {entry.undone_by
                    ? t("clientHistoryUndone")
                    : stateLabel(entry.state, t)}
                </span>
              </UnstyledButton>
              {trash && (
                <Button
                  variant="subtle"
                  disabled={
                    !!entry.undone_by ||
                    !["pending", "accepted"].includes(entry.state)
                  }
                  onClick={() => setSelected(entry.id)}
                >
                  {t("restore")}
                </Button>
              )}
            </li>
          ))}
        </ol>
      )}
      {list.hasNextPage && (
        <Button
          loading={list.isFetchingNextPage}
          onClick={() => list.fetchNextPage()}
        >
          {t("loadMore")}
        </Button>
      )}
      <DialogPresence>
        {selected && (
          <Dialog
            open
            title={t(trash ? "restore" : "clientHistory")}
            onOpenChange={(open) => {
              if (!open && !locked) setSelected(undefined);
            }}
          >
            <LocalHistory
              key={selected}
              active
              initialSelected={selected}
              restore={trash}
              onLocked={setLocked}
            />
          </Dialog>
        )}
      </DialogPresence>
    </>
  );
}
