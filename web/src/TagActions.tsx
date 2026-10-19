import { InlineRename } from "./InlineRename";
import { Button, Group, InputWrapper, Text, TextInput } from "@mantine/core";
import { useId, useState } from "react";
import { createWrite, type Tag } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
import { flattenTags, tagOption } from "./TagPath";
import { TagSelect } from "./TagSelect";

export type TagAction = "rename" | "addChildTag" | "changeParentTag" | "remove";

export function TagRename({ tag, close }: { tag: Tag; close: () => void }) {
  return (
    <InlineRename
      initialName={tag.name}
      close={close}
      write={(name) => createWrite("PUT", `/tags/${tag.id}/name`, { name })}
    />
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
  const editing = action !== "remove";
  return (
    <Dialog
      open
      title={t(editing ? action : "deleteTagTitle")}
      size="sm"
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
          size="sm"
          c={editing ? "dimmed" : undefined}
          mb={editing ? "md" : 0}
          style={{ overflowWrap: "anywhere" }}
        >
          {editing ? (
            <>
              {t(action === "addChildTag" ? "parentTag" : "tags")}:{" "}
              <bdi>
                {
                  tagOption(
                    flattenTags(tags).find((item) => item.id === tag.id) ?? {
                      ...tag,
                      ancestors: [],
                    },
                  ).label
                }
              </bdi>
            </>
          ) : (
            t("deleteNamedTagConfirm", { name: `\u2068${tag.name}\u2069` })
          )}
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
          <InputWrapper id={id} label={t("parentTag")}>
            <TagSelect
              autoFocus
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
          </InputWrapper>
        )}
        <WriteFeedback state={state} />
        <Group justify="flex-end" gap="sm" mt={editing ? "lg" : "md"}>
          <Button
            variant="subtle"
            color="gray"
            data-autofocus={action === "remove" || undefined}
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
        </Group>
      </form>
    </Dialog>
  );
}
