import { Button, Group, Text } from "@mantine/core";
import { InlineRename } from "./InlineRename";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get } from "./api";
import type { components } from "./api-schema";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";

export type EntryActionTarget = {
  id: string;
  name: string;
  box: string;
  folder: boolean;
  action: "rename" | "trash";
};
export function EntryRename({
  entry,
  close,
}: {
  entry: EntryActionTarget;
  close: () => void;
}) {
  return (
    <InlineRename
      initialName={entry.name}
      close={close}
      write={(name) =>
        createWrite(
          "PUT",
          `/boxes/${entry.box}/entries/${entry.id}/name${entry.folder ? "?folder=true" : ""}`,
          { before: entry.name, name },
        )
      }
    />
  );
}
export function EntryActions({
  entry,
  close,
}: {
  entry: EntryActionTarget;
  close: () => void;
}) {
  const { t } = useI18n();
  const state = useWrites(close);
  const path = `/boxes/${entry.box}/entries/${entry.id}`;
  const suffix = entry.folder ? "?folder=true" : "";
  const preview = useQuery({
    queryKey: ["entry-deletion-preview", entry.box, entry.id, entry.folder],
    queryFn: ({ signal }) =>
      get<components["schemas"]["EntryDeletionPreview"]>(
        `${path}/deletion-preview${suffix}`,
        signal,
      ),
    enabled: entry.action === "trash",
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    retry: false,
  });
  return (
    <Dialog
      open
      size="sm"
      title={t("trashEntry")}
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (state.busy || state.locked) return;
          if (preview.data && !preview.isFetching && !preview.isError) {
            void state.run([
              createWrite("DELETE", `${path}${suffix}`, {
                revision: preview.data.revision,
              }),
            ]);
          }
        }}
      >
        <>
          {preview.isFetching && (
            <Text size="sm" role="status">
              {t("loading")}
            </Text>
          )}
          {preview.isError && (
            <div role="alert">
              {errorMessage(preview.error, t)}{" "}
              <Button variant="subtle" onClick={() => preview.refetch()}>
                {t("retry")}
              </Button>
            </div>
          )}
          {preview.data && (
            <>
              <Text size="sm">
                {t("trashEntryConfirm", {
                  name: `\u2068${preview.data.name}\u2069`,
                })}
              </Text>
              {entry.folder && (
                <Text size="sm" c="dimmed" mt="xs">
                  {t("trashFolderContents", {
                    files: preview.data.file_count,
                    folders: preview.data.folder_count,
                  })}
                </Text>
              )}
              <Text size="sm" c="dimmed" mt="xs">
                {t("trashEntryRestoreHint")}
              </Text>
            </>
          )}
        </>
        <WriteFeedback state={state} />
        <Group justify="flex-end" gap="sm" mt="md">
          <Button
            variant="subtle"
            color="gray"
            disabled={state.busy}
            onClick={close}
          >
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            variant="filled"
            color="red"
            disabled={
              state.busy ||
              state.locked ||
              !preview.data ||
              preview.isFetching ||
              preview.isError
            }
          >
            {t("trashEntry")}
          </Button>
        </Group>
      </form>
    </Dialog>
  );
}
