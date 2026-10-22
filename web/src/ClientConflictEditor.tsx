import {
  Anchor,
  Button,
  Checkbox,
  Group,
  Stack,
  Text,
  TextInput,
  Textarea,
} from "@mantine/core";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get } from "./api";
import type { components } from "./client-api-schema";
import { Select } from "./Select";
import { FilesetEditor, zeroID, type Fileset } from "./Filesets";
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
  const [deletionConfirmed, setDeletionConfirmed] = useState(false);
  const writes = useWrites(close);
  const [domainLocked, setDomainLocked] = useState(false);
  useEffect(() => {
    onLocked(writes.busy || writes.locked || domainLocked);
    return () => onLocked(false);
  }, [writes.busy, writes.locked, domainLocked, onLocked]);
  const cover = snapshot?.cover;
  const coverValue = values.cover_file_id ?? cover?.proposed.id;
  const coverCandidates = cover
    ? [cover.proposed, cover.current, cover.before]
    : [];
  const coverValid =
    !cover ||
    coverValue === zeroID ||
    coverCandidates.some((file) => file.id === coverValue && file.available);
  const coverName = (file: NonNullable<Draft["cover"]>["before"]) =>
    file.id === zeroID
      ? t("clientConflictNoCover")
      : `${file.name || file.id}${file.available ? "" : ` · ${t("clientConflictUnavailableCover")}`}`;
  function submit() {
    const data = snapshot;
    if (
      !data?.replacement ||
      !coverValid ||
      (data.deletion && !deletionConfirmed)
    )
      return;
    const replacement = structuredClone(data.replacement);
    // Paths are supplied by the server's domain editor. Only intended values
    // change; reviewed before-values, IDs and timestamp stay fixed on retries.
    for (const field of data.fields || []) {
      let parent = replacement.payload as Record<string, unknown>;
      for (const part of (field.path || []).slice(0, -1))
        parent = parent[part] as Record<string, unknown>;
      parent[(field.path || []).at(-1)!] = values[field.key] ?? field.proposed;
    }
    if (data.cover) {
      const payload = replacement.payload as Record<string, unknown>;
      (payload.cover_file_id as Record<string, unknown>).after = coverValue;
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
            setDeletionConfirmed(false);
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
          {!snapshot.replacement && !snapshot.deletion && (
            <Text>{t("clientConflictUnsupported")}</Text>
          )}
          {snapshot.deletion && (
            <Stack gap="sm">
              <Text size="sm" c="dimmed">
                {t("clientConflictBefore")} ·{" "}
                {t("clientConflictDeletionScope", {
                  name: snapshot.deletion.before.name,
                  files: snapshot.deletion.before.file_count,
                  folders: snapshot.deletion.before.folder_count,
                })}
              </Text>
              {snapshot.deletion.current ? (
                <>
                  <Text size="sm">
                    {t("clientConflictCurrent")} ·{" "}
                    {t("clientConflictDeletionScope", {
                      name: snapshot.deletion.current.name,
                      files: snapshot.deletion.current.file_count,
                      folders: snapshot.deletion.current.folder_count,
                    })}
                  </Text>
                  <Anchor
                    size="sm"
                    href={`/boxes/${snapshot.deletion.box_id}${snapshot.deletion.whole_box ? "" : `?${snapshot.deletion.folder ? "folder" : "file"}=${snapshot.deletion.id}`}`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {t("clientConflictInspectDeletion")}
                  </Anchor>
                  <Text size="sm">{t("clientConflictDeletionWarning")}</Text>
                  <Checkbox
                    label={t("clientConflictConfirmDeletion")}
                    checked={deletionConfirmed}
                    disabled={writes.busy || writes.locked}
                    onChange={(event) =>
                      setDeletionConfirmed(event.currentTarget.checked)
                    }
                  />
                </>
              ) : (
                <Text>{t("clientConflictDeletionGone")}</Text>
              )}
            </Stack>
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
          {cover && (
            <Stack gap="xs">
              <Text size="sm" fw={600}>
                {t("clientConflictCover")}
              </Text>
              <Text size="sm" c="dimmed">
                {t("clientConflictBefore")} · {coverName(cover.before)}
              </Text>
              <Text size="sm">
                {t("clientConflictCurrent")} · {coverName(cover.current)}
              </Text>
              <Select
                label={t("clientConflictChosen")}
                value={coverValue!}
                disabled={writes.busy || writes.locked}
                onValueChange={(value) =>
                  setValues({ ...values, cover_file_id: value })
                }
                options={[
                  {
                    value: zeroID,
                    label: t("clientConflictNoCover"),
                    special: true,
                  },
                  ...coverCandidates
                    .filter(
                      (file, i, all) =>
                        file.id !== zeroID &&
                        all.findIndex((other) => other.id === file.id) === i,
                    )
                    .map((file) => ({
                      value: file.id,
                      label: coverName(file),
                      disabled: !file.available,
                    })),
                ]}
              />
              {!coverValid && (
                <Text size="sm" c="dimmed">
                  {t("clientConflictChooseCover")}
                </Text>
              )}
            </Stack>
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
              <Button
                disabled={
                  writes.busy ||
                  writes.locked ||
                  !coverValid ||
                  (!!snapshot.deletion && !deletionConfirmed)
                }
                onClick={submit}
                color={snapshot.deletion ? "red" : undefined}
              >
                {t(
                  snapshot.deletion
                    ? "clientConflictSaveDeletion"
                    : "clientConflictSave",
                )}
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
