import {
  UnstyledButton,
  TextInput,
  Button,
  Tree,
  useTree,
  type TreeNodeData,
} from "@mantine/core";
import { useId, useMemo, useState } from "react";
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
  const { prefs, t } = useI18n();
  const tree = useTree();
  const data = useMemo(() => {
    const nodes = (items: Tag[]): TreeNodeData[] =>
      items.map((tag) => ({
        value: tag.id,
        label: tag.name,
        children: nodes(tag.children || []),
      }));
    return nodes(tags);
  }, [tags]);
  const counts = useMemo(
    () => new Map(flattenTags(tags).map((tag) => [tag.id, tag.count])),
    [tags],
  );
  return (
    <Tree
      className="tag-tree"
      aria-label={t("tags")}
      data={data}
      tree={tree}
      levelOffset={16}
      expandOnClick={false}
      allowRangeSelection={false}
      onKeyDownCapture={(event) => {
        const target = event.target as HTMLElement;
        if (
          event.key === "Enter" &&
          target.getAttribute("role") === "treeitem"
        ) {
          event.preventDefault();
          event.stopPropagation();
          if (target.dataset.value) choose(target.dataset.value);
        }
      }}
      renderNode={({ node, hasChildren, expanded, elementProps }) => (
        <div
          {...elementProps}
          className={`${elementProps.className} tag-tree-row`}
          onClick={(event) => {
            elementProps.onClick(event);
            choose(node.value);
          }}
        >
          {hasChildren ? (
            <UnstyledButton
              className="tag-tree-toggle"
              tabIndex={-1}
              aria-label={t(expanded ? "collapseTag" : "expandTag", {
                name: String(node.label),
              })}
              aria-expanded={expanded}
              onClick={(event) => {
                event.stopPropagation();
                tree.toggleExpanded(node.value);
                event.currentTarget
                  .closest<HTMLElement>('[role="treeitem"]')
                  ?.focus();
              }}
            >
              <svg
                width="12"
                height="12"
                viewBox="0 0 16 16"
                fill="none"
                aria-hidden="true"
                className="tag-tree-chevron"
                data-expanded={expanded}
              >
                <path
                  d="m6 3 5 5-5 5"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </UnstyledButton>
          ) : (
            <span className="tag-tree-spacer" aria-hidden="true" />
          )}
          <bdi title={String(node.label)}>{node.label}</bdi>
          {!!counts.get(node.value) && (
            <small>{formatNumber(counts.get(node.value)!, prefs)}</small>
          )}
        </div>
      )}
    />
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
