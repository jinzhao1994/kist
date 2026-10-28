import { taskErrorMessage, taskRunningMessage } from "./TaskStatus";
import { OCRAction } from "./AI";
import { HelpTooltip } from "./HelpTooltip";
import { Button, Text, Group } from "@mantine/core";
import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { createWrite, get, requestKey, result } from "./api";
import type { components } from "./api-schema";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Job = { state: string; error?: string; manual?: boolean };

type TaskOptions = {
  title: string;
  fileID: string;
  extractor?: string;
  missing?: boolean;
  empty?: boolean;
};
type TaskControls = { action: ReactNode; status: ReactNode; hidden: boolean };

// One display group may have several independently owned extraction tasks.
export function MetadataGroup({
  children,
  secondary,
  help,
  ...task
}: TaskOptions & {
  children?: ReactNode;
  help?: string;
  secondary?: Omit<TaskOptions, "fileID">;
}) {
  const primary = useMetadataTask(task);
  const extra = useMetadataTask({
    fileID: task.fileID,
    title: "",
    ...secondary,
  });
  if (primary.hidden) return null;
  const render = (action: ReactNode, status: ReactNode) => (
    <section className="file-metadata-group">
      {task.title && (
        <div className="metadata-group-heading">
          <Group gap="xs">
            <h4>
              <bdi>{task.title}</bdi>
            </h4>
            {help && (
              <HelpTooltip label={task.title} muted>
                {help}
              </HelpTooltip>
            )}
          </Group>
          <Group gap="xs" className="metadata-group-actions">
            {action}
            {extra.action}
          </Group>
        </div>
      )}
      {status}
      {extra.status}
      {children}
    </section>
  );
  if (
    task.empty &&
    (task.extractor === "ocr" ||
      task.extractor === "desc" ||
      task.extractor === "orientation")
  ) {
    return (
      <OCRAction fileID={task.fileID} kind={task.extractor}>
        {({ action, status }) => render(action, status)}
      </OCRAction>
    );
  }
  return render(primary.action, primary.status);
}

function useMetadataTask({
  title,
  fileID,
  extractor,
  missing = false,
  empty = false,
}: TaskOptions): TaskControls {
  const { t } = useI18n();
  const queries = useQueryClient();
  const ai =
    extractor === "ocr" ||
    extractor === "desc" ||
    extractor === "orientation" ||
    extractor === "geo";
  const key = ["metadata-refresh", fileID, extractor];
  const busy = useIsMutating({ mutationKey: key }) > 0;
  const [error, setError] = useState("");
  const write = useRef<ReturnType<typeof createWrite> | undefined>(undefined);
  const request = useRef<string | undefined>(undefined);
  const job = useQuery({
    queryKey: [`${extractor}-job`, fileID],
    queryFn: ({ signal }) => get<Job>(`/files/${fileID}/${extractor}`, signal),
    enabled: ai,
    retry: false,
    refetchInterval: ai ? 2000 : false,
  });
  const backgroundQueued =
    ai && !empty && job.data?.state === "queued" && !job.data.manual;
  const queue = useQuery({
    queryKey: [`${extractor}-queue`],
    queryFn: ({ signal }) =>
      get<{ paused: boolean }>(
        extractor === "geo" ? "/geo" : `/ai/${extractor}`,
        signal,
      ),
    enabled: backgroundQueued,
    retry: false,
    refetchInterval: backgroundQueued ? 2000 : false,
  });
  const pending =
    ai && (job.data?.state === "queued" || job.data?.state === "running");
  const waiting =
    pending &&
    (job.data?.state === "running" ||
      job.data?.manual ||
      queue.data?.paused !== true);
  const wasPending = useRef(false);
  useEffect(() => {
    if (wasPending.current && !pending) void queries.invalidateQueries();
    wasPending.current = pending;
  }, [pending, queries]);
  const mutation = useMutation({
    mutationKey: key,
    mutationFn: async () => {
      setError("");
      if (ai) {
        request.current ??= requestKey();
        const body: components["schemas"]["Post-api-v1-files-by-id-descRequest"] &
          components["schemas"]["Post-api-v1-files-by-id-ocrRequest"] = {
          force: !missing,
          retry: job.data?.state === "failed",
          request_id: request.current,
        };
        const value = await result<Job>(
          await fetch(`/api/v1/files/${fileID}/${extractor}`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
          }),
        );
        queries.setQueryData([`${extractor}-job`, fileID], value);
        request.current = undefined;
      } else {
        write.current ??= createWrite(
          "POST",
          `/files/${fileID}/extractions/${extractor}`,
          { force: true },
        );
        await write.current.run();
        write.current = undefined;
      }
      await queries.invalidateQueries();
    },
    onError: (e) => setError(errorMessage(e, t)),
  });
  const issue =
    error ||
    (ai &&
      (taskErrorMessage(job.data?.error, t) ||
        (job.error && errorMessage(job.error, t))));
  return {
    hidden:
      empty &&
      ai &&
      (job.isPending ||
        (!!job.data && job.data.state !== "missing" && !job.data.manual)),
    action: extractor ? (
      <Button
        variant="subtle"
        size="compact-xs"
        className="metadata-refresh-action"
        data-active={missing || busy || pending || !!issue || undefined}
        disabled={busy || !!waiting || (ai && job.isPending)}
        aria-label={
          extractor === "geo"
            ? t(missing ? "geoExtract" : "geoRefresh")
            : t("reextractGroup", { group: title })
        }
        onClick={() => mutation.mutate()}
      >
        {t(
          busy || (ai && job.data?.state === "running")
            ? taskRunningMessage(extractor)
            : waiting
              ? "reextractQueued"
              : missing
                ? "geoExtract"
                : extractor === "geo"
                  ? "geoRefresh"
                  : "reextract",
        )}
      </Button>
    ) : null,
    status: (
      <>
        {issue && (
          <Text role="alert" size="sm">
            {issue}
          </Text>
        )}
      </>
    ),
  };
}
