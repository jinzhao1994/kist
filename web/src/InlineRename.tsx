import { InlineValueEdit } from "./InlineValueEdit";
import { useI18n } from "./i18n";
import type { Write } from "./mutations";

export function InlineRename({
  initialName,
  write,
  close,
}: {
  initialName: string;
  write: (name: string) => Write;
  close: () => void;
}) {
  const { t } = useI18n();
  return (
    <InlineValueEdit
      initialValue={initialName}
      label={t("rename")}
      required
      maxLength={255}
      write={write}
      close={close}
    />
  );
}
