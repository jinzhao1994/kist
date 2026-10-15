import { TextInput } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback, type Write } from "./mutations";

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
  const [name, setName] = useState(initialName);
  const input = useRef<HTMLInputElement>(null);
  const submitting = useRef(false);
  const cancelled = useRef(false);
  const state = useWrites(close);
  useEffect(() => {
    input.current?.select();
  }, []);
  return (
    <form
      className="inline-rename"
      onClick={(e) => e.stopPropagation()}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) {
          e.currentTarget.requestSubmit();
        }
      }}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === "Escape" && !state.busy) {
          e.preventDefault();
          cancelled.current = true;
          close();
        }
      }}
      onSubmit={(e) => {
        e.preventDefault();
        if (
          cancelled.current ||
          submitting.current ||
          state.busy ||
          state.locked
        )
          return;
        submitting.current = true;
        if (name === initialName) close();
        else
          void state.run([write(name)]).finally(() => {
            submitting.current = false;
          });
      }}
    >
      <TextInput
        ref={input}
        autoFocus
        size="xs"
        aria-label={t("rename")}
        required
        maxLength={255}
        value={name}
        disabled={state.busy || state.locked}
        onChange={(e) => setName(e.target.value)}
      />
      <WriteFeedback state={state} />
    </form>
  );
}
