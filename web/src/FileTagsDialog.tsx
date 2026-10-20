import { Button } from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { TagAssignment } from "./Tags";

export function FileTagsDialog({
  file,
  close,
}: {
  file: string;
  close: () => void;
}) {
  const { t } = useI18n();
  const detail = useQuery({
    queryKey: ["file", file],
    queryFn: ({ signal }) => api.file(file, signal),
  });
  const tags = useQuery({
    queryKey: ["tags"],
    queryFn: ({ signal }) => api.tags(signal),
  });
  return (
    <Dialog
      open
      size="sm"
      title={t("addTag")}
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      {detail.isError || tags.isError ? (
        <div role="alert">
          {t("error")}
          <Button
            variant="subtle"
            onClick={() => {
              void detail.refetch();
              void tags.refetch();
            }}
          >
            {t("retry")}
          </Button>
        </div>
      ) : detail.data && tags.data ? (
        <TagAssignment
          inDialog
          autoFocus
          target={file}
          kind="files"
          tags={tags.data.tags || []}
          assigned={detail.data.tags || []}
        />
      ) : (
        <p role="status">{t("loading")}</p>
      )}
    </Dialog>
  );
}
