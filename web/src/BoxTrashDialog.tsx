import { Button } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { api, createWrite, type Box } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";

export function BoxTrashDialog({
  box,
  close,
  done,
}: {
  box: Box;
  close: () => void;
  done: () => void;
}) {
  const { t } = useI18n();
  const writes = useWrites(done);
  const preview = useQuery({
    queryKey: ["box-deletion", box.id],
    queryFn: ({ signal }) => api.boxDeletionPreview(box.id, signal),
    staleTime: 0,
    gcTime: 0,
    refetchOnWindowFocus: false,
  });
  return (
    <Dialog
      open
      title={t("moveBoxToTrash")}
      onOpenChange={(open) => {
        if (!open && !writes.busy) close();
      }}
    >
      {preview.isPending && <p role="status">{t("loading")}</p>}
      {preview.isError && (
        <div role="alert">
          <p>{errorMessage(preview.error, t)}</p>
          <Button onClick={() => preview.refetch()}>{t("retry")}</Button>
        </div>
      )}
      {preview.data && (
        <p>
          {t("trashBoxConfirm", {
            name: preview.data.name,
            count: preview.data.file_count,
          })}
        </p>
      )}
      <WriteFeedback state={writes} />
      {!!writes.error && <p>{t("trashBoxChanged")}</p>}
      <div className="actions">
        <Button data-autofocus disabled={writes.busy} onClick={close}>
          {t("cancel")}
        </Button>
        <Button
          color="red"
          variant="filled"
          disabled={
            !preview.data ||
            preview.isFetching ||
            preview.isError ||
            writes.busy ||
            writes.locked
          }
          onClick={() => {
            if (preview.data)
              writes.run([
                createWrite("DELETE", `/boxes/${box.id}`, {
                  revision: preview.data.revision,
                }),
              ]);
          }}
        >
          {t(writes.busy ? "saving" : "moveBoxToTrash")}
        </Button>
      </div>
    </Dialog>
  );
}
