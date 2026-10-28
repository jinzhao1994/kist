import { Button, Group, Stack, Textarea, Text } from "@mantine/core";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createWrite, get, requestKey, type FileDetail } from "./api";
import { useI18n } from "./i18n";
import { MetadataLabel } from "./MetadataLabel";
import { MetadataValue } from "./MetadataValue";
import { FullTextTooltip } from "./FullTextTooltip";
import { useWrites, WriteFeedback } from "./mutations";

type Row = NonNullable<FileDetail["metadata"]>[number];
type Props = {
  fileID: string;
  blobKey: string;
  kind: "ocr" | "desc" | "facts";
  row: Row;
  manual: boolean;
  canRun: boolean;
};

export function AISupplement(props: Props) {
  const { t } = useI18n();
  const { row } = props;
  return (
    <div>
      <MetadataLabel
        metadataKey={row.key}
        label={t("aiHint")}
        help={t("aiHintHelp")}
      />
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
            <FullTextTooltip text={row.value}>
              <div tabIndex={0}>
                <bdi>{row.value}</bdi>
              </div>
            </FullTextTooltip>
          ) : (
            <span className="metadata-help">{t("aiHintPlaceholder")}</span>
          )}
        </MetadataValue>
      </dd>
    </div>
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
}: Props & { close: () => void }) {
  const { t } = useI18n();
  const [value, setValue] = useState(row.value);
  const [before, setBefore] = useState(row);
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
    const write = createWrite("PUT", `/files/${fileID}/metadata`, {
      blob_key: blobKey,
      key: row.key,
      before_value: before.value,
      before_source: before.source,
      before_type: before.type,
      value,
    });
    const operations = [
      {
        ...write,
        run: async (signal?: AbortSignal) => {
          const result = await write.run(signal);
          setBefore({
            ...before,
            value,
            source: value === before.value ? before.source : "user",
          });
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
