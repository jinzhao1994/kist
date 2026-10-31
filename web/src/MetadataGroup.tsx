import { TaskError, taskPollInterval, taskErrorMessage } from "./TaskStatus";
import { OCRAction } from "./AI";
import { HelpTooltip } from "./HelpTooltip";
import { Button, Group } from "@mantine/core";
import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { api, createWrite, get, requestKey, result } from "./api";
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
            {help && <HelpTooltip label={task.title}>{help}</HelpTooltip>}
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
      task.extractor === "facts" ||
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
    extractor === "facts" ||
    extractor === "orientation" ||
    extractor === "geo";
  const local = !!extractor && !ai;
  const instance = useQuery({
    queryKey: ["info"],
    queryFn: ({ signal }) => api.info(signal),
    enabled: local,
    staleTime: 10_000,
  });
  const queued = ai || (local && !!instance.data && !instance.data.client);
  const task = local ? "metadata" : extractor;
  const key = ["metadata-refresh", fileID, extractor];
  const busy = useIsMutating({ mutationKey: key }) > 0;
  const [error, setError] = useState("");
  const write = useRef<ReturnType<typeof createWrite> | undefined>(undefined);
  const request = useRef<string | undefined>(undefined);
  const job = useQuery<Job>({
    queryKey: [`${task}-job`, fileID],
    queryFn: ({ signal }) => get<Job>(`/files/${fileID}/${task}`, signal),
    enabled: queued,
    retry: false,
    staleTime: 10_000,
    refetchInterval: taskPollInterval,
  });
  const backgroundQueued =
    queued && !empty && job.data?.state === "queued" && !job.data.manual;
  const queue = useQuery({
    queryKey: [`${task}-queue`],
    queryFn: ({ signal }) =>
      get<{ paused: boolean }>(
        task === "metadata"
          ? "/metadata"
          : extractor === "geo"
            ? "/geo"
            : `/ai/${extractor}`,
        signal,
      ),
    enabled: backgroundQueued,
    retry: false,
    refetchInterval: (query) =>
      query.state.error || query.state.data?.paused ? 15000 : 2000,
  });
  const pending =
    queued && (job.data?.state === "queued" || job.data?.state === "running");
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
      if (queued) {
        request.current ??= requestKey();
        const body = {
          force: !missing,
          retry: job.data?.state === "failed",
          request_id: request.current,
          ...(local ? { extractor } : {}),
        };
        const value = await result<Job>(
          await fetch(`/api/v1/files/${fileID}/${task}`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
          }),
        );
        queries.setQueryData([`${task}-job`, fileID], value);
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
    (queued &&
      (taskErrorMessage(job.data?.error, t) ||
        (!job.data && job.error && errorMessage(job.error, t))));
  return {
    hidden:
      empty &&
      queued &&
      (job.isPending ||
        (!!job.data && job.data.state !== "missing" && !job.data.manual)),
    action: extractor ? (
      <Button
        variant="subtle"
        size="compact-xs"
        className="metadata-refresh-action"
        data-active={missing || busy || pending || !!issue || undefined}
        disabled={
          busy ||
          !!waiting ||
          (queued && job.isPending) ||
          (local && instance.isPending)
        }
        aria-label={
          extractor === "geo"
            ? t(missing ? "geoExtract" : "geoRefresh")
            : t("reextractGroup", { group: title })
        }
        onClick={() => mutation.mutate()}
      >
        {t(
          busy || (queued && job.data?.state === "running")
            ? "extracting"
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
    status: <>{issue && <TaskError>{issue}</TaskError>}</>,
  };
}
