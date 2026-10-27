import { Button, Group, Stack, Textarea, Text } from "@mantine/core";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createWrite, get, requestKey, type FileDetail } from "./api";
import { useI18n } from "./i18n";
import { MetadataLabel } from "./MetadataLabel";
import { MetadataValue } from "./MetadataValue";
import { CopyTextPreview } from "./CopyTextPreview";
import { HelpTooltip } from "./HelpTooltip";
import { useWrites, WriteFeedback } from "./mutations";

type Row = NonNullable<FileDetail["metadata"]>[number];
type Props = {
  fileID: string;
  blobKey: string;
  kind: "ocr" | "desc";
  row?: Row;
  manual: boolean;
  canRun: boolean;
};

export function AISupplement(props: Props) {
  const { t } = useI18n();
  const row = props.row || {
    key: `${props.kind}:hint`,
    value: "",
    type: "text",
    source: "",
  };
  return (
    <dl className="metadata-list">
      <div>
        <MetadataLabel metadataKey={row.key} label={t("aiHint")}>
          <HelpTooltip label={t("aiHint")} muted>
            {t("aiHintHelp")}
          </HelpTooltip>
        </MetadataLabel>
        <dd>
          <MetadataValue
            fileID={props.fileID}
            blobKey={props.blobKey}
            row={row}
            label={t("aiHint")}
            editor={(snapshot, close) => (
              <HintEditor {...props} row={snapshot} close={close} />
            )}
          >
            {row.value ? (
              <CopyTextPreview text={row.value} copyLabel="copy" />
            ) : (
              <span className="metadata-help">{t("aiHintPlaceholder")}</span>
            )}
          </MetadataValue>
        </dd>
      </div>
    </dl>
  );
}

function HintEditor({
  fileID,
  blobKey,
  kind,
  row,
  manual,
  canRun,
  close,
}: Props & { row: Row; close: () => void }) {
  const { t } = useI18n();
  const [value, setValue] = useState(row.value);
  const [before, setBefore] = useState<string | null>(
    row.source === "user" ? row.value : null,
  );
  const state = useWrites(close);
  const queries = useQueryClient();
  const job = useQuery({
    queryKey: [`${kind}-job`, fileID],
    queryFn: ({ signal }) =>
      get<{ state: string }>(`/files/${fileID}/${kind}`, signal),
    enabled: canRun,
    retry: false,
    refetchInterval: canRun ? 2000 : false,
  });
  const pending = job.data?.state === "queued" || job.data?.state === "running";
  const tooLong = new TextEncoder().encode(value).length > 1024;
  const save = (run: boolean) => {
    if (state.busy || state.locked || tooLong) return;
    const write = createWrite(
      "PUT",
      `/files/${fileID}/metadata/hints/${kind}`,
      {
        blob_key: blobKey,
        before,
        value,
      },
    );
    const operations = [
      {
        ...write,
        run: async (signal?: AbortSignal) => {
          const result = await write.run(signal);
          setBefore(before === null && value === "" ? null : value);
          await queries.invalidateQueries({ queryKey: ["file", fileID] });
          return result;
        },
      },
    ];
    if (run && canRun && !manual && !pending)
      operations.push(
        createWrite("POST", `/files/${fileID}/${kind}`, {
          force: true,
          retry: true,
          request_id: requestKey(),
        }),
      );
    void state.run(operations);
  };
  return (
    <form
      className="inline-rename"
      onSubmit={(e) => {
        e.preventDefault();
        save(false);
      }}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === "Escape" && !state.busy) {
          e.preventDefault();
          close();
        }
      }}
    >
      <Stack gap="xs">
        <Textarea
          autoFocus
          size="xs"
          autosize
          minRows={1}
          maxRows={8}
          aria-label={t("aiHint")}
          placeholder={t("aiHintPlaceholder")}
          value={value}
          disabled={state.busy || state.locked}
          error={tooLong ? t("aiHintTooLong") : undefined}
          onChange={(e) => setValue(e.target.value)}
        />
        {manual && (
          <Text size="xs" c="dimmed">
            {t("aiHintManual")}
          </Text>
        )}
        <Group gap="xs">
          <Button
            type="submit"
            variant="subtle"
            size="compact-xs"
            disabled={state.busy || state.locked || tooLong}
          >
            {t("save")}
          </Button>
          {canRun && (
            <Button
              type="button"
              variant="subtle"
              size="compact-xs"
              disabled={
                state.busy ||
                state.locked ||
                tooLong ||
                manual ||
                pending ||
                job.isPending ||
                !!job.error
              }
              onClick={() => save(true)}
            >
              {t("aiHintSaveRun")}
            </Button>
          )}
          <Button
            type="button"
            variant="subtle"
            size="compact-xs"
            disabled={state.busy}
            onClick={close}
          >
            {t("cancel")}
          </Button>
        </Group>
        <WriteFeedback state={state} />
      </Stack>
    </form>
  );
}
