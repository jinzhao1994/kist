import { Anchor } from "@mantine/core";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { result } from "./api";
import { FullTextTooltip } from "./FullTextTooltip";
import { TaskError, TaskStatus } from "./TaskStatus";
import { errorMessage } from "./mutations";
import { useI18n, type MessageKey } from "./i18n";

// A test is explicit, cancelable and scoped to the displayed saved snapshot.
export function ConnectionTest<T extends { error: string }>({
  path,
  revision,
  saved,
  disabled,
  hint,
  saveHint,
  reasons,
  success,
  children,
}: {
  path: string;
  revision: string;
  saved: boolean;
  disabled: boolean;
  hint: string;
  saveHint: string;
  reasons: Record<string, MessageKey>;
  success: (value: T) => ReactNode;
  children: (state: {
    action: ReactNode;
    feedback: ReactNode;
    testing: boolean;
  }) => ReactNode;
}) {
  const { t } = useI18n();
  const [probe, setProbe] = useState<T>();
  const [error, setError] = useState("");
  const [testing, setTesting] = useState(false);
  const active = useRef<AbortController | undefined>(undefined);
  useEffect(() => {
    setProbe(undefined);
    setError("");
    setTesting(false);
    return () => {
      active.current?.abort();
      active.current = undefined;
    };
  }, [path, revision, saved]);
  async function test() {
    if (active.current || disabled || !saved) return;
    const controller = new AbortController();
    active.current = controller;
    setTesting(true);
    setProbe(undefined);
    setError("");
    try {
      const value = await result<T>(
        await fetch(`/api/v1${path}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
          signal: controller.signal,
        }),
      );
      if (!controller.signal.aborted) setProbe(value);
    } catch (error) {
      if (!controller.signal.aborted) setError(errorMessage(error, t));
    } finally {
      if (active.current === controller) active.current = undefined;
      if (!controller.signal.aborted) setTesting(false);
    }
  }
  const action = (
    <FullTextTooltip text={saved ? hint : saveHint}>
      <span className="tooltip-target" tabIndex={!saved ? 0 : undefined}>
        <Anchor
          component="button"
          type="button"
          className="text-action-link"
          size="xs"
          disabled={disabled || !saved || testing}
          onClick={test}
        >
          {t(testing ? "connectionTesting" : "connectionTest")}
        </Anchor>
      </span>
    </FullTextTooltip>
  );
  const feedback = (
    <>
      {testing && <TaskStatus>{t("connectionTesting")}</TaskStatus>}
      {saved &&
        probe &&
        (probe.error ? (
          <TaskError>{t(reasons[probe.error] || "error")}</TaskError>
        ) : (
          <TaskStatus>{success(probe)}</TaskStatus>
        ))}
      {saved && error && <TaskError>{error}</TaskError>}
    </>
  );
  return children({ action, feedback, testing });
}
