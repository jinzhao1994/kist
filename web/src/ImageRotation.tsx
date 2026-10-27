import { Button, Group, Stack, Text } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { createWrite, requestKey, result, type ArchiveFile } from "./api";
import { Dialog } from "./Dialog";
import { Select } from "./Select";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

export function ImageRotation({
  file,
  close,
}: {
  file: ArchiveFile;
  close: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const [angle, setAngle] = useState(String(file.image_rotation || 0));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const pending = useRef<
    | {
        value: number | null;
        write: ReturnType<typeof createWrite>;
        request: string;
      }
    | undefined
  >(undefined);
  useEffect(() => {
    setAngle(String(file.image_rotation || 0));
  }, [file.id, file.blob_key, file.image_rotation]);
  useEffect(() => {
    pending.current = undefined;
    setError("");
  }, [file.id, file.blob_key]);
  async function save(value: number | null) {
    setBusy(true);
    setError("");
    try {
      if (!pending.current || pending.current.value !== value)
        pending.current = {
          value,
          write: createWrite("PUT", `/files/${file.id}/image-rotation`, {
            blob_key: file.blob_key,
            clockwise_degrees: value,
          }),
          request: requestKey(),
        };
      const operation = pending.current;
      await operation.write.run();
      if (value === null)
        await result(
          await fetch(`/api/v1/files/${file.id}/orientation`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              force: true,
              retry: false,
              request_id: operation.request,
            }),
          }),
        );
      pending.current = undefined;
      close();
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
      await queries.invalidateQueries();
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(value) => {
        if (!busy && !value) close();
      }}
      title={t("orientationEdit")}
    >
      <Stack>
        <Select
          label={t("orientationAngle")}
          value={angle}
          onValueChange={setAngle}
          disabled={busy}
          options={[0, 90, 180, 270].map((n) => ({
            value: String(n),
            label: t("orientationDegrees", { degrees: n }),
          }))}
        />
        <Text size="sm" c="dimmed">
          {t(
            file.image_rotation_manual
              ? "orientationManualHint"
              : "orientationEditHint",
          )}
        </Text>
        {error && (
          <Text role="alert" size="sm">
            {error}
          </Text>
        )}
        <Group>
          <Button disabled={busy} onClick={() => void save(Number(angle))}>
            {t("orientationSave")}
          </Button>
          {(file.image_rotation_manual || pending.current?.value === null) && (
            <Button disabled={busy} onClick={() => void save(null)}>
              {t("orientationRestoreAuto")}
            </Button>
          )}
        </Group>
      </Stack>
    </Dialog>
  );
}
