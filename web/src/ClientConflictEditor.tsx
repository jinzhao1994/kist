import { Button, Group, Stack, Text, TextInput, Textarea } from "@mantine/core";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get } from "./api";
import type { components } from "./client-api-schema";
import { describeHistory } from "./History";
import { useI18n } from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";

type Draft = components["schemas"]["ClientConflictEdit"];
export function ClientConflictEditor({
  id,
  action,
  close,
  onLocked,
}: {
  id: string;
  action: string;
  close: () => void;
  onLocked: (locked: boolean) => void;
}) {
  const { t } = useI18n();
  const [snapshot, setSnapshot] = useState<Draft>();
  const draft = useQuery({
    queryKey: ["client", "conflict-edit", id],
    queryFn: ({ signal }) =>
      get<Draft>(`/client/pending/${encodeURIComponent(id)}/edit`, signal),
    enabled: !snapshot,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
  });
  useEffect(() => {
    if (!snapshot && !draft.isFetching && draft.data) setSnapshot(draft.data);
  }, [snapshot, draft.isFetching, draft.data]);
  const [values, setValues] = useState<Record<string, string>>({});
  const writes = useWrites(close);
  useEffect(() => {
    onLocked(writes.busy || writes.locked);
    return () => onLocked(false);
  }, [writes.busy, writes.locked, onLocked]);
  function submit() {
    const data = snapshot;
    if (!data?.replacement) return;
    const replacement = structuredClone(data.replacement);
    // Paths are supplied by the server's domain editor. Only intended values
    // change; reviewed before-values, IDs and timestamp stay fixed on retries.
    for (const field of data.fields || []) {
      let parent = replacement.payload as Record<string, unknown>;
      for (const part of (field.path || []).slice(0, -1))
        parent = parent[part] as Record<string, unknown>;
      parent[(field.path || []).at(-1)!] = values[field.key] ?? field.proposed;
    }
    writes.run([
      createWrite("POST", `/client/pending/${encodeURIComponent(id)}/resolve`, {
        replacement,
        reason: "User reviewed and edited the conflicting local change",
      }),
    ]);
  }
  return (
    <Stack gap="md">
      <Group>
        <Button disabled={writes.busy || writes.locked} onClick={close}>
          {t("clientHistoryBack")}
        </Button>
        <Button
          disabled={writes.busy || writes.locked || draft.isFetching}
          onClick={() => {
            setValues({});
            setSnapshot(undefined);
            draft.refetch();
          }}
        >
          {t("refresh")}
        </Button>
      </Group>
      {draft.isPending && <Text role="status">{t("loading")}</Text>}
      {draft.error && <Text role="alert">{errorMessage(draft.error, t)}</Text>}
      {snapshot && (
        <>
          <Text fw={600}>{describeHistory(action, snapshot.summary, t)}</Text>
          <Text size="sm" c="dimmed">
            {t("clientConflictReviewHint")}
          </Text>
          {!snapshot.replacement && (
            <Text>{t("clientConflictUnsupported")}</Text>
          )}
          {(snapshot.fields || []).map((field) => (
            <Stack gap="xs" key={field.key}>
              <Text size="sm" fw={600}>
                {t(field.key === "name" ? "name" : "description")}
              </Text>
              <Text size="sm" c="dimmed">
                {t("clientConflictBefore")} · {field.before || "—"}
              </Text>
              <Text size="sm">
                {t("clientConflictCurrent")} · {field.current || "—"}
              </Text>
              {field.key === "description" ? (
                <Textarea
                  label={t("clientConflictChosen")}
                  value={values[field.key] ?? field.proposed}
                  disabled={writes.busy || writes.locked}
                  onChange={(e) =>
                    setValues({ ...values, [field.key]: e.currentTarget.value })
                  }
                  autosize
                  minRows={2}
                />
              ) : (
                <TextInput
                  label={t("clientConflictChosen")}
                  value={values[field.key] ?? field.proposed}
                  disabled={writes.busy || writes.locked}
                  onChange={(e) =>
                    setValues({ ...values, [field.key]: e.currentTarget.value })
                  }
                />
              )}
            </Stack>
          ))}
          {snapshot.replacement && (
            <Group>
              <Button disabled={writes.busy || writes.locked} onClick={submit}>
                {t("clientConflictSave")}
              </Button>
            </Group>
          )}
        </>
      )}
      <WriteFeedback state={writes} />
    </Stack>
  );
}
