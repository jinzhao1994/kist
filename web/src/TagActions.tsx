import { Button, Text, TextInput } from "@mantine/core";
import { useEffect, useId, useRef, useState } from "react";
import { createWrite, type Tag } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
import { flattenTags, tagOption } from "./TagPath";
import { TagSelect } from "./TagSelect";

export type TagAction = "rename" | "addChildTag" | "changeParentTag" | "remove";

export function TagRename({ tag, close }: { tag: Tag; close: () => void }) {
  const { t } = useI18n();
  const [name, setName] = useState(tag.name);
  const input = useRef<HTMLInputElement>(null);
  const submitting = useRef(false);
  const cancelled = useRef(false);
  const state = useWrites(close);
  useEffect(() => {
    input.current?.select();
  }, []);
  return (
    <form
      className="tag-rename"
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
        if (name === tag.name) close();
        else
          void state
            .run([createWrite("PUT", `/tags/${tag.id}/name`, { name })])
            .finally(() => {
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

export function TagActionDialog({
  tag,
  action,
  tags,
  close,
}: {
  tag: Tag;
  action: Exclude<TagAction, "rename">;
  tags: Tag[];
  close: () => void;
}) {
  const { t } = useI18n();
  const id = useId();
  const [name, setName] = useState("");
  const [parent, setParent] = useState(tag.parent_id || "");
  const state = useWrites(close);
  const descendants = new Set(flattenTags([tag]).map((item) => item.id));
  return (
    <Dialog
      open
      title={t(action)}
      size={action === "addChildTag" ? "sm" : undefined}
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (state.busy || state.locked) return;
          state.run([
            action === "addChildTag"
              ? createWrite("POST", "/tags", { name, parent_id: tag.id })
              : action === "changeParentTag"
                ? createWrite("PUT", `/tags/${tag.id}/parent`, {
                    parent_id: parent || null,
                  })
                : createWrite("DELETE", `/tags/${tag.id}`),
          ]);
        }}
      >
        <Text
          size={action === "addChildTag" ? "sm" : undefined}
          c={action === "addChildTag" ? "dimmed" : undefined}
          mb="md"
          style={{ overflowWrap: "anywhere" }}
        >
          {action === "remove"
            ? t("deleteTagConfirm")
            : t(action === "addChildTag" ? "parentTag" : "tags") + ":"}{" "}
          <bdi>
            {action === "addChildTag"
              ? tagOption(
                  flattenTags(tags).find((item) => item.id === tag.id) ?? {
                    ...tag,
                    ancestors: [],
                  },
                ).label
              : tag.name}
          </bdi>
        </Text>
        {action === "addChildTag" && (
          <TextInput
            autoFocus
            data-autofocus
            label={t("name")}
            required
            withAsterisk={false}
            maxLength={255}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={state.busy || state.locked}
          />
        )}
        {action === "changeParentTag" && (
          <>
            <label htmlFor={id}>{t("parentTag")}</label>
            <TagSelect
              id={id}
              value={parent}
              onValueChange={setParent}
              disabled={state.busy || state.locked}
              options={[
                { value: "", label: t("rootTag") },
                ...flattenTags(tags)
                  .filter((item) => !descendants.has(item.id))
                  .map(tagOption),
              ]}
            />
          </>
        )}
        <WriteFeedback state={state} />
        <div className="actions">
          <Button
            variant="subtle"
            color="gray"
            onClick={close}
            disabled={state.busy}
          >
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            variant="filled"
            color={action === "remove" ? "red" : undefined}
            disabled={state.busy || state.locked}
          >
            {t(
              action === "remove"
                ? "remove"
                : action === "addChildTag"
                  ? "addTag"
                  : "save",
            )}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
