import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get, type Tag } from "./api";
import { Dialog } from "./Dialog";
import { formatNumber, useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
export function flattenTags(
  tags: Tag[],
  depth = 0,
): (Tag & { depth: number })[] {
  return tags.flatMap((tag) => [
    { ...tag, depth },
    ...flattenTags(tag.children || [], depth + 1),
  ]);
}
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
          <button onClick={() => choose(tag.id)}>
            <span aria-hidden="true">#</span> <bdi>{tag.name}</bdi>
            <small>
              {tag.count == null ? "" : formatNumber(tag.count, prefs)}
            </small>
          </button>
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
          <select
            id={`${id}-tag`}
            value={selected}
            onChange={(e) => select(e.target.value)}
          >
            <option value="">{t("newTag")}</option>
            {flat.map((tag) => (
              <option key={tag.id} value={tag.id}>
                {"　".repeat(tag.depth)}
                {tag.name}
              </option>
            ))}
          </select>
          <label htmlFor={`${id}-name`}>{t("name")}</label>
          <input
            id={`${id}-name`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={255}
          />
          <label htmlFor={`${id}-parent`}>{t("parentTag")}</label>
          <select
            id={`${id}-parent`}
            value={parent}
            onChange={(e) => setParent(e.target.value)}
          >
            <option value="">{t("rootTag")}</option>
            {flat
              .filter((tag) => !descendants.has(tag.id))
              .map((tag) => (
                <option key={tag.id} value={tag.id}>
                  {"　".repeat(tag.depth)}
                  {tag.name}
                </option>
              ))}
          </select>
        </fieldset>
        <WriteFeedback state={state} />
        <div className="actions">
          {selected && (
            <button
              type="button"
              disabled={state.busy || state.locked}
              onClick={() => setConfirm(true)}
            >
              {t("remove")}
            </button>
          )}
          <button className="primary" disabled={state.busy || state.locked}>
            {t("save")}
          </button>
        </div>
        {confirm && (
          <div className="notice">
            <p>
              {t("deleteTagConfirm")}: <bdi>{current?.name}</bdi>
            </p>
            <div className="actions">
              <button
                type="button"
                disabled={state.busy}
                onClick={() => setConfirm(false)}
              >
                {t("cancel")}
              </button>
              <button
                type="button"
                disabled={state.busy || state.locked}
                onClick={() =>
                  state.run([createWrite("DELETE", `/tags/${selected}`)])
                }
              >
                {t("remove")}
              </button>
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
            <button
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
            </button>
          </li>
        ))}
      </ul>
      <div className="inline-form">
        <label className="sr-only" htmlFor={id}>
          {t("selectTag")}
        </label>
        <select
          id={id}
          disabled={state.busy || state.locked}
          value={selected}
          onChange={(e) => setSelected(e.target.value)}
        >
          <option value="">{t("selectTag")}</option>
          {flat
            .filter((tag) => !assigned.includes(tag.id))
            .map((tag) => (
              <option key={tag.id} value={tag.id}>
                {tag.name}
              </option>
            ))}
        </select>
        <button
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
        </button>
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
