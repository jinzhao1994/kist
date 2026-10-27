import { Button, Text } from "@mantine/core";
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

type Job = { state: string; error?: string };

// Display groups can share an extractor (EXIF and GPS).
export function MetadataGroup({
  title,
  fileID,
  extractor,
  children,
}: {
  title: string;
  fileID: string;
  extractor?: string;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const ai =
    extractor === "ocr" || extractor === "desc" || extractor === "orientation";
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
  const queue = useQuery({
    queryKey: [`${extractor}-queue`],
    queryFn: ({ signal }) =>
      get<{ paused: boolean }>(`/ai/${extractor}`, signal),
    enabled: ai,
    retry: false,
    refetchInterval: ai ? 2000 : false,
  });
  const pending =
    ai && (job.data?.state === "queued" || job.data?.state === "running");
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
          force: true,
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
    (ai && (job.data?.error || (job.error && errorMessage(job.error, t))));
  return (
    <section className="file-metadata-group">
      {title && (
        <div className="metadata-group-heading">
          <h4>
            <bdi>{title}</bdi>
          </h4>
          {extractor && (
            <Button
              variant="subtle"
              size="compact-xs"
              className="metadata-refresh-action"
              data-active={busy || pending || !!issue || undefined}
              disabled={busy || pending || (ai && job.isPending)}
              aria-label={t("reextractGroup", { group: title })}
              onClick={() => mutation.mutate()}
            >
              {t(
                busy || (ai && job.data?.state === "running")
                  ? "extracting"
                  : pending
                    ? "reextractQueued"
                    : "reextract",
              )}
            </Button>
          )}
        </div>
      )}
      {issue && (
        <Text role="alert" size="sm">
          {issue}
        </Text>
      )}
      {ai && queue.data?.paused && (
        <Text size="sm">
          {t(
            extractor === "orientation"
              ? "orientationQueuePaused"
              : extractor === "desc"
                ? "descQueuePaused"
                : "ocrQueuePaused",
          )}{" "}
          <a href="/settings#settings-ai">{t("ocrSettings")}</a>
        </Text>
      )}
      {children}
    </section>
  );
}
