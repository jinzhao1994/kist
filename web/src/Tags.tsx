import { UnstyledButton, TextInput, Button } from "@mantine/core";
import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get, type Tag } from "./api";
import { TagSelect } from "./TagSelect";
import { flattenTags, tagOption } from "./TagPath";
import { Dialog } from "./Dialog";
import { formatNumber, useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
export function TagTree({
  tags,
  choose,
}: {
  tags: Tag[];
  choose: (id: string) => void;
}) {
  const { prefs } = useI18n();
  return (
    <ul className="tag-tree">
      {tags.map((tag) => (
        <li key={tag.id}>
          <UnstyledButton onClick={() => choose(tag.id)}>
            <span aria-hidden="true">#</span> <bdi>{tag.name}</bdi>
            <small>
              {tag.count == null ? "" : formatNumber(tag.count, prefs)}
            </small>
          </UnstyledButton>
          {tag.children?.length ? (
            <TagTree tags={tag.children} choose={choose} />
          ) : null}
        </li>
      ))}
    </ul>
  );
}
export function TagManager({
  tags,
  close,
}: {
  tags: Tag[];
  close: () => void;
}) {
  const { t } = useI18n();
  const id = useId();
  const flat = flattenTags(tags);
  const [selected, setSelected] = useState("");
  const [name, setName] = useState("");
  const [parent, setParent] = useState("");
  const [confirm, setConfirm] = useState(false);
  const state = useWrites(() => {
    setSelected("");
    setName("");
    setParent("");
    setConfirm(false);
  });
  const current = flat.find((tag) => tag.id === selected);
  const descendants = new Set(
    current ? flattenTags([current]).map((tag) => tag.id) : [],
  );
  function select(value: string) {
    const tag = flat.find((tag) => tag.id === value);
    setSelected(value);
    setName(tag?.name || "");
    setParent(tag?.parent_id || "");
    setConfirm(false);
    state.reset();
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
      title={t("manageTags")}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (state.locked) return;
          const writes = selected
            ? [
                createWrite("PUT", `/tags/${selected}/name`, { name }),
                createWrite("PUT", `/tags/${selected}/parent`, {
                  parent_id: parent || null,
                }),
              ]
            : [
                createWrite("POST", "/tags", {
                  name,
                  ...(parent && { parent_id: parent }),
                }),
              ];
          state.run(writes);
        }}
      >
        <fieldset disabled={state.busy || state.locked}>
          <label htmlFor={`${id}-tag`}>{t("selectTag")}</label>
          <TagSelect
            id={`${id}-tag`}
            value={selected}
            onValueChange={select}
            disabled={state.busy || state.locked}
            options={[
              { value: "", label: t("newTag") },
              ...flat.map(tagOption),
            ]}
          />
          <label htmlFor={`${id}-name`}>{t("name")}</label>
          <TextInput
            id={`${id}-name`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={255}
          />
          <label htmlFor={`${id}-parent`}>{t("parentTag")}</label>
          <TagSelect
            id={`${id}-parent`}
            value={parent}
            onValueChange={setParent}
            disabled={state.busy || state.locked}
            options={[
              { value: "", label: t("rootTag") },
              ...flat.filter((tag) => !descendants.has(tag.id)).map(tagOption),
            ]}
          />
        </fieldset>
        <WriteFeedback state={state} />
        <div className="actions">
          {selected && (
            <Button
              type="button"
              disabled={state.busy || state.locked}
              onClick={() => setConfirm(true)}
            >
              {t("remove")}
            </Button>
          )}
          <Button
            variant="filled"
            type="submit"
            disabled={state.busy || state.locked}
          >
            {t("save")}
          </Button>
        </div>
        {confirm && (
          <div className="notice">
            <p>
              {t("deleteTagConfirm")}: <bdi>{current?.name}</bdi>
            </p>
            <div className="actions">
              <Button
                type="button"
                disabled={state.busy}
                onClick={() => setConfirm(false)}
              >
                {t("cancel")}
              </Button>
              <Button
                type="button"
                disabled={state.busy || state.locked}
                onClick={() =>
                  state.run([createWrite("DELETE", `/tags/${selected}`)])
                }
              >
                {t("remove")}
              </Button>
            </div>
          </div>
        )}
      </form>
    </Dialog>
  );
}
export function TagAssignment({
  target,
  kind,
  tags,
  assigned,
}: {
  target: string;
  kind: "files" | "boxes";
  tags: Tag[];
  assigned: string[];
}) {
  const { t } = useI18n();
  const id = useId();
  const flat = flattenTags(tags);
  const [selected, setSelected] = useState("");
  const state = useWrites(() => setSelected(""));
  return (
    <section className="tag-assignment">
      <h3>{t("tags")}</h3>
      <ul className="tag-chips">
        {assigned.map((tagID) => (
          <li key={tagID}>
            <bdi>{flat.find((tag) => tag.id === tagID)?.name || tagID}</bdi>
            <Button
              disabled={state.busy || state.locked}
              aria-label={`${t("removeTag")}: ${flat.find((tag) => tag.id === tagID)?.name || tagID}`}
              onClick={() =>
                state.run([
                  createWrite("DELETE", `/tags/${tagID}/${kind}`, {
                    ids: [target],
                  }),
                ])
              }
            >
              ×
            </Button>
          </li>
        ))}
      </ul>
      <div className="inline-form">
        <label className="sr-only" htmlFor={id}>
          {t("selectTag")}
        </label>
        <TagSelect
          id={id}
          disabled={state.busy || state.locked}
          value={selected}
          onValueChange={setSelected}
          options={[
            { value: "", label: t("selectTag") },
            ...flat.filter((tag) => !assigned.includes(tag.id)).map(tagOption),
          ]}
        />
        <Button
          disabled={!selected || state.busy || state.locked}
          onClick={() =>
            state.run([
              createWrite("PUT", `/tags/${selected}/${kind}`, {
                ids: [target],
              }),
            ])
          }
        >
          {t("addTag")}
        </Button>
      </div>
      <WriteFeedback state={state} />
    </section>
  );
}
export function BoxTags({ box, tags }: { box: string; tags: Tag[] }) {
  const query = useQuery({
    queryKey: ["box-tags", box],
    queryFn: ({ signal }) =>
      get<{ tags: string[] }>(`/boxes/${box}/tags`, signal),
  });
  const { t } = useI18n();
  if (query.isError) return <p role="alert">{t("error")}</p>;
  return query.data ? (
    <TagAssignment
      target={box}
      kind="boxes"
      tags={tags}
      assigned={query.data.tags || []}
    />
  ) : (
    <p role="status">{t("loading")}</p>
  );
}
