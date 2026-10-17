import { ContextMenu, useContextMenu } from "./ContextMenu";
import { SidebarMenuButton } from "./SidebarMenuButton";
import {
  Menu,
  UnstyledButton,
  TextInput,
  Button,
  Tree,
  useTree,
  type TreeNodeData,
} from "@mantine/core";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createWrite, get, type Tag } from "./api";
import { TagSelect } from "./TagSelect";
import { flattenTags, tagOption } from "./TagPath";
import { Dialog, DialogPresence } from "./Dialog";
import { formatNumber, useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
import { TagActionDialog, TagRename, type TagAction } from "./TagActions";
export function TagTree({
  tags,
  choose,
}: {
  tags: Tag[];
  choose: (id: string, destination?: "tag" | "boxes" | "files") => void;
}) {
  const { prefs, t } = useI18n();
  const tree = useTree();
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(
    null,
  );
  const [action, setAction] = useState<{ tag: Tag; kind: TagAction } | null>(
    null,
  );
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
    () => new Map(flattenTags(tags).map((tag) => [tag.id, tag])),
    [tags],
  );
  return (
    <>
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
            target.getAttribute("role") === "treeitem" &&
            (event.key === "ContextMenu" ||
              (event.shiftKey && event.key === "F10"))
          ) {
            event.preventDefault();
            event.stopPropagation();
            if (!action && target.dataset.value) {
              const rect = target.getBoundingClientRect();
              setMenu({
                id: target.dataset.value,
                x: rect.left,
                y: rect.bottom,
              });
            }
            return;
          }
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
            data-active={
              menu?.id === node.value ||
              action?.tag.id === node.value ||
              undefined
            }
            onContextMenu={(event) => {
              if (action?.kind === "rename") return;
              event.preventDefault();
              event.stopPropagation();
              setMenu({ id: node.value, x: event.clientX, y: event.clientY });
            }}
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
            {action?.kind === "rename" && action.tag.id === node.value ? (
              <TagRename
                key={node.value}
                tag={action.tag}
                close={() => setAction(null)}
              />
            ) : (
              <bdi title={String(node.label)}>{node.label}</bdi>
            )}
            {!!counts.get(node.value)?.count && (
              <small
                title={
                  counts.get(node.value)?.box_count != null &&
                  counts.get(node.value)?.file_count != null
                    ? t("tagResultCounts", {
                        boxes: counts.get(node.value)!.box_count!,
                        files: counts.get(node.value)!.file_count!,
                      })
                    : undefined
                }
              >
                {formatNumber(counts.get(node.value)!.count!, prefs)}
              </small>
            )}
            <SidebarMenuButton
              disabled={action?.kind === "rename"}
              label={t("tagActions", { name: String(node.label) })}
              expanded={menu?.id === node.value}
              onClick={(event) => {
                event.stopPropagation();
                const rect = event.currentTarget.getBoundingClientRect();
                setMenu(
                  menu?.id === node.value
                    ? null
                    : { id: node.value, x: rect.left, y: rect.bottom },
                );
              }}
            />
          </div>
        )}
      />
      <ContextMenu anchor={menu} close={() => setMenu(null)}>
        {(["tag", "boxes", "files"] as const).map((destination) => (
          <Menu.Item
            key={destination}
            onClick={() => {
              if (menu) choose(menu.id, destination);
              setMenu(null);
            }}
          >
            {t(
              destination === "tag"
                ? "openTagPage"
                : destination === "boxes"
                  ? "viewTaggedBoxes"
                  : "viewTaggedFiles",
            )}
          </Menu.Item>
        ))}
        <Menu.Divider />
        {(["rename", "addChildTag", "changeParentTag", "remove"] as const).map(
          (kind) => (
            <Menu.Item
              key={kind}
              color={kind === "remove" ? "red" : undefined}
              onClick={() => {
                setMenu(null);
                const tag = menu && counts.get(menu.id);
                if (tag) setAction({ tag, kind });
              }}
            >
              {t(kind)}
            </Menu.Item>
          ),
        )}
      </ContextMenu>
      <DialogPresence>
        {action && action.kind !== "rename" && (
          <TagActionDialog
            key={`${action.tag.id}:${action.kind}`}
            tag={action.tag}
            action={action.kind}
            tags={tags}
            close={() => setAction(null)}
          />
        )}
      </DialogPresence>
    </>
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
  const nameInput = useRef<HTMLInputElement>(null);
  const [focusName, setFocusName] = useState(false);
  const state = useWrites(() => {
    setSelected("");
    setName("");
    if (selected) setParent("");
    else setFocusName(true);
    setConfirm(false);
  });
  useEffect(() => {
    if (focusName && !state.busy) {
      nameInput.current?.focus();
      setFocusName(false);
    }
  }, [focusName, state.busy]);
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
            ref={nameInput}
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
  compact = false,
  editing = true,
  target,
  kind,
  tags,
  assigned,
}: {
  compact?: boolean;
  editing?: boolean;
  target: string;
  kind: "files" | "boxes";
  tags: Tag[];
  assigned: string[];
}) {
  const { t } = useI18n();
  const id = useId();
  const flat = flattenTags(tags);
  const context = useContextMenu<string>();
  const [selected, setSelected] = useState("");
  const state = useWrites(() => setSelected(""));
  return (
    <section className={`tag-assignment ${compact ? "compact-tags" : ""}`}>
      {!compact && <h3>{t("tags")}</h3>}
      <ul className="tag-chips">
        {assigned.map((tagID) => (
          <li
            key={tagID}
            tabIndex={0}
            aria-haspopup="menu"
            {...context.bindings(tagID)}
          >
            <bdi>{flat.find((tag) => tag.id === tagID)?.name || tagID}</bdi>
          </li>
        ))}
      </ul>
      <ContextMenu anchor={context.menu} close={context.close}>
        <Menu.Item
          disabled={state.busy || state.locked}
          onClick={() => {
            const tagID = context.menu?.item;
            if (!tagID || state.busy || state.locked) return;
            context.close();
            state.run([
              createWrite("DELETE", `/tags/${tagID}/${kind}`, {
                ids: [target],
              }),
            ]);
          }}
        >
          {t("removeTag")}
        </Menu.Item>
      </ContextMenu>
      {editing && (
        <div className="inline-form">
          <label className="sr-only" htmlFor={id}>
            {t("selectTag")}
          </label>
          <TagSelect
            id={id}
            disabled={state.busy || state.locked}
            value={selected}
            onValueChange={(tagID) => {
              if (
                !tagID ||
                state.busy ||
                state.locked ||
                assigned.includes(tagID)
              )
                return;
              setSelected(tagID);
              state.run([
                createWrite("PUT", `/tags/${tagID}/${kind}`, { ids: [target] }),
              ]);
            }}
            options={[
              { value: "", label: t("selectTag") },
              ...flat
                .filter((tag) => !assigned.includes(tag.id))
                .map(tagOption),
            ]}
          />
        </div>
      )}
      <WriteFeedback state={state} />
    </section>
  );
}
export function BoxTags({
  box,
  tags,
  compact = false,
  editing = true,
}: {
  box: string;
  tags: Tag[];
  compact?: boolean;
  editing?: boolean;
}) {
  const query = useQuery({
    queryKey: ["box-tags", box],
    queryFn: ({ signal }) =>
      get<{ tags: string[] }>(`/boxes/${box}/tags`, signal),
  });
  const { t } = useI18n();
  if (query.isError) return <p role="alert">{t("error")}</p>;
  return query.data ? (
    <TagAssignment
      compact={compact}
      editing={editing}
      target={box}
      kind="boxes"
      tags={tags}
      assigned={query.data.tags || []}
    />
  ) : (
    <p role="status">{t("loading")}</p>
  );
}
