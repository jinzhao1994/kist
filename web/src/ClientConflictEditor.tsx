import { Button, Group, Stack, Text, TextInput, Textarea } from "@mantine/core";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get } from "./api";
import type { components } from "./client-api-schema";
import { FilesetEditor, type Fileset } from "./Filesets";
import { PartialDateInput } from "./PartialDateInput";
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
  const [domainLocked, setDomainLocked] = useState(false);
  useEffect(() => {
    onLocked(writes.busy || writes.locked || domainLocked);
    return () => onLocked(false);
  }, [writes.busy, writes.locked, domainLocked, onLocked]);
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
        <Button
          disabled={writes.busy || writes.locked || domainLocked}
          onClick={close}
        >
          {t("clientHistoryBack")}
        </Button>
        <Button
          disabled={
            writes.busy || writes.locked || domainLocked || draft.isFetching
          }
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
          {snapshot.fileset && snapshot.replacement && (
            <FilesetConflictEditor
              key={snapshot.replacement.id}
              draft={snapshot}
              id={id}
              close={close}
              onLocked={setDomainLocked}
            />
          )}
          {(snapshot.fields || []).map((field) => (
            <Stack gap="xs" key={field.key}>
              <Text size="sm" fw={600}>
                {t(
                  (
                    {
                      name: "name",
                      description: "description",
                      start_at: "startAt",
                      end_at: "endAt",
                    } as const
                  )[field.key],
                )}
              </Text>
              <Text size="sm" c="dimmed">
                {t("clientConflictBefore")} · {field.before || "—"}
              </Text>
              <Text size="sm">
                {t("clientConflictCurrent")} · {field.current || "—"}
              </Text>
              {field.key === "start_at" || field.key === "end_at" ? (
                <fieldset disabled={writes.busy || writes.locked}>
                  <PartialDateInput
                    id={`conflict-${id}-${field.key}`}
                    label={t("clientConflictChosen")}
                    value={values[field.key] ?? field.proposed}
                    onChange={(value) =>
                      setValues({ ...values, [field.key]: value })
                    }
                  />
                  <Text size="xs" c="dimmed">
                    {t("dateHint")}
                  </Text>
                </fieldset>
              ) : field.key === "description" ? (
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
          {snapshot.replacement && !snapshot.fileset && (
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

function FilesetConflictEditor({
  draft,
  id,
  close,
  onLocked,
}: {
  draft: Draft;
  id: string;
  close: () => void;
  onLocked: (locked: boolean) => void;
}) {
  const { t } = useI18n();
  const sets = useQuery({
    queryKey: ["filesets"],
    queryFn: ({ signal }) => get<{ filesets: Fileset[] }>("/filesets", signal),
  });
  if (sets.isPending) return <Text role="status">{t("loading")}</Text>;
  if (sets.error)
    return <Text role="alert">{errorMessage(sets.error, t)}</Text>;
  const config = draft.fileset!;
  const currentSets = (sets.data.filesets || []).filter(
    (set) => set.folder_id !== config.folder_id,
  );
  if (config.current) currentSets.push(config.current);
  return (
    <FilesetEditor
      folder={{ id: config.folder_id, name: config.name }}
      box={config.box_id}
      sets={currentSets}
      close={close}
      edit={{
        initial: config.proposed || null,
        comparison: [
          { label: t("clientConflictBefore"), value: config.before || null },
          { label: t("clientConflictCurrent"), value: config.current || null },
          ...(!config.proposed
            ? [{ label: t("clientConflictChosen"), value: null }]
            : []),
        ],
        onLocked,
        createWrite: (after) => {
          const replacement = structuredClone(draft.replacement!);
          const payload = replacement.payload as Record<string, unknown>;
          if (after) payload.after = after;
          else delete payload.after;
          return createWrite(
            "POST",
            `/client/pending/${encodeURIComponent(id)}/resolve`,
            {
              replacement,
              reason:
                "User reviewed and edited the conflicting fileset configuration",
            },
          );
        },
      }}
    />
  );
}
