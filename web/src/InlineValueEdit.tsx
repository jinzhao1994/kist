import { Button, Group, Textarea, TextInput } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback, type Write } from "./mutations";

export function InlineValueEdit({
  initialValue: initialName,
  label,
  multiline = false,
  required = false,
  maxLength,
  write,
  close,
}: {
  initialValue: string;
  label: string;
  multiline?: boolean;
  required?: boolean;
  maxLength?: number;
  write: (name: string) => Write;
  close: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState(initialName);
  const input = useRef<HTMLInputElement & HTMLTextAreaElement>(null);
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
        if (e.nativeEvent.isComposing) return;
        if (multiline && e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
          e.preventDefault();
          e.currentTarget.requestSubmit();
        }
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
      {multiline ? (
        <Textarea
          ref={input}
          autoFocus
          autosize
          minRows={3}
          maxRows={12}
          size="xs"
          aria-label={label}
          value={name}
          disabled={state.busy || state.locked}
          onChange={(e) => setName(e.target.value)}
        />
      ) : (
        <TextInput
          ref={input}
          autoFocus
          size="xs"
          aria-label={label}
          required={required}
          maxLength={maxLength}
          value={name}
          disabled={state.busy || state.locked}
          onKeyDown={(e) => {
            if (e.key === "Enter" && e.nativeEvent.isComposing)
              e.preventDefault();
          }}
          onChange={(e) => setName(e.target.value)}
        />
      )}
      {multiline && (
        <Group gap="xs">
          <Button
            type="submit"
            variant="subtle"
            size="compact-xs"
            disabled={state.busy || state.locked}
          >
            {t("save")}
          </Button>
          <Button
            type="button"
            variant="subtle"
            size="compact-xs"
            disabled={state.busy}
            onClick={() => {
              cancelled.current = true;
              close();
            }}
          >
            {t("cancel")}
          </Button>
        </Group>
      )}
      <WriteFeedback state={state} />
    </form>
  );
}
