import { Button, Text } from "@mantine/core";
import { Dialog, DialogPresence } from "./Dialog";
import { useI18n } from "./i18n";

export function UploadPasteDialog({
  files,
  confirm,
  cancel,
}: {
  files?: File[];
  confirm: () => void;
  cancel: () => void;
}) {
  const { t } = useI18n();
  return (
    <DialogPresence>
      {files && (
        <Dialog
          open
          title={t("pasteUploadTitle")}
          size="sm"
          onOpenChange={(open) => {
            if (!open) cancel();
          }}
        >
          <Text size="sm" c="dimmed">
            {t("pasteUploadHint")}
          </Text>
          <ul className="upload-conflict-files">
            {files.map((file, index) => (
              <li key={index}>{file.name}</li>
            ))}
          </ul>
          <div className="actions">
            <Button variant="subtle" onClick={cancel}>
              {t("cancel")}
            </Button>
            <Button onClick={confirm}>{t("uploadFiles")}</Button>
          </div>
        </Dialog>
      )}
    </DialogPresence>
  );
}
