import { Button } from "@mantine/core";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { APIError, createWrite } from "./api";
import { useI18n, type Translate } from "./i18n";
export function errorMessage(error: unknown, t: Translate) {
  if (error instanceof APIError) {
    if (error.body.code === "request_conflict") return t("requestConflict");
    if (error.status === 404) return t("notFound");
    if (error.body.code === "tag_cycle") return t("tagCycle");
    if (error.status === 400 || error.status === 422) return t("invalidInput");
    if (error.status === 409) return t("conflict");
    return t("errorDetails", {
      message: error.body.message || t("error"),
      code: error.body.code || String(error.status),
    });
  }
  return t("error");
}
export type Write = ReturnType<typeof createWrite>;
// Keep the same immutable operations after a failed response. A multi-step
// edit retries completed steps with their original keys before continuing.
export function useWrites(onSuccess?: () => void) {
  const client = useQueryClient();
  const [operations, setOperations] = useState<Write[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  async function run(next = operations) {
    if (busy || !next.length) return;
    setOperations(next);
    setBusy(true);
    setError(undefined);
    try {
      for (const operation of next) await operation.run();
      await client.invalidateQueries();
      setOperations([]);
      onSuccess?.();
    } catch (error) {
      setError(error);
    } finally {
      setBusy(false);
    }
  }
  return {
    run,
    busy,
    error,
    locked: operations.length > 0,
    reset: () => {
      operations.forEach((operation) => operation.discard());
      setOperations([]);
      setError(undefined);
    },
  };
}
export function WriteFeedback({
  state,
}: {
  state: ReturnType<typeof useWrites>;
}) {
  const { t } = useI18n();
  if (!state.error) return null;
  return (
    <div className="write-error" role="alert">
      <p>{errorMessage(state.error, t)}</p>
      <div className="actions">
        <Button type="button" disabled={state.busy} onClick={() => state.run()}>
          {t("retry")}
        </Button>
        <Button type="button" disabled={state.busy} onClick={state.reset}>
          {t("newOperation")}
        </Button>
      </div>
    </div>
  );
}
