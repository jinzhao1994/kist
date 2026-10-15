import { SidebarMenuButton } from "./SidebarMenuButton";
import {
  ActionIcon,
  Button,
  Menu,
  TextInput,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, createWrite, get, type Box, type Folder, type Tag } from "./api";
import type { components } from "./api-schema";
import { ContextMenu, useContextMenu } from "./ContextMenu";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { errorMessage, useWrites, WriteFeedback } from "./mutations";
import { flattenTags } from "./TagPath";
import type { BoxFilters, Filters } from "./Filters";
import type { Route } from "./navigation";

export type SearchDefinition = components["schemas"]["SearchDefinition"];
export type SavedSearch = components["schemas"]["SavedSearchView"];

export function searchDefinition(
  entity: "files" | "boxes",
  filters: Filters | BoxFilters,
  box?: string,
  folder?: string,
): SearchDefinition {
  return {
    version: 1,
    entity,
    ...(box && { box_id: box }),
    ...(box && folder && { folder_id: folder }),
    filters: {
      name: filters.name || "",
      sort: filters.sort || "updated",
      asc: !!filters.asc,
      from: filters.from || "",
      to: filters.to || "",
      tag_all: filters.tag_all || [],
      tag_any: filters.tag_any || [],
      tag_not: filters.tag_not || [],
      ...(entity === "files" && {
        kind: filters.kind || [],
        box: box ? [] : (filters as Filters).box || [],
      }),
    },
  };
}

export function savedSearchRoute(d: SearchDefinition): Route {
  if (d.entity === "boxes")
    return { files: false, boxFilters: d.filters as BoxFilters };
  return {
    files: true,
    box: d.box_id,
    folder: d.folder_id,
    filters: d.filters as Filters,
  };
}

// Never drop missing references: particularly NOT-tag filters would broaden.
export function searchReferencesExist(
  d: SearchDefinition,
  tags: Tag[],
  boxes: Box[],
  folders?: Folder[],
) {
  const tagIDs = new Set(flattenTags(tags).map((tag) => tag.id));
  const boxIDs = new Set(boxes.map((box) => box.id));
  return (
    d.version === 1 &&
    (d.entity === "files" || d.entity === "boxes") &&
    [
      ...(d.filters.tag_all || []),
      ...(d.filters.tag_any || []),
      ...(d.filters.tag_not || []),
    ].every((id) => tagIDs.has(id)) &&
    [...(d.filters.box || []), ...(d.box_id ? [d.box_id] : [])].every((id) =>
      boxIDs.has(id),
    ) &&
    (!d.folder_id || !!folders?.some((folder) => folder.id === d.folder_id))
  );
}

function StarIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2L12 17.3l-5.6 2.9 1.1-6.2L3 9.6l6.2-.9Z" />
    </svg>
  );
}

function SearchEditor({
  item,
  definition,
  remove = false,
  close,
}: {
  item?: SavedSearch;
  definition: SearchDefinition;
  remove?: boolean;
  close: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState(item?.name || "");
  const state = useWrites(close);
  return (
    <Dialog
      open
      title={t(
        remove ? "removeSavedSearch" : item ? "editSavedSearch" : "saveSearch",
      )}
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (state.busy || state.locked) return;
          state.run([
            remove && item
              ? createWrite("DELETE", `/saved-searches/${item.id}`)
              : createWrite(
                  item ? "PUT" : "POST",
                  item ? `/saved-searches/${item.id}` : "/saved-searches",
                  {
                    name,
                    definition,
                    ...(item && {
                      before: { name: item.name, definition: item.definition },
                    }),
                  },
                ),
          ]);
        }}
      >
        {remove ? (
          <p>
            {t("removeSavedSearchHint")} <bdi>{item?.name}</bdi>
          </p>
        ) : (
          <>
            <TextInput
              data-autofocus
              label={t("name")}
              required
              maxLength={255}
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={state.busy || state.locked}
            />
            <p className="muted">
              {t(definition.entity === "boxes" ? "boxes" : "allFiles")} ·{" "}
              {t("savedSearchLiveHint")}
            </p>
            {item && definition !== item.definition && (
              <p>{t("replaceSearchHint")}</p>
            )}
          </>
        )}
        <WriteFeedback state={state} />
        <div className="actions">
          <Button onClick={close} disabled={state.busy}>
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            variant="filled"
            color={remove ? "red" : undefined}
            disabled={state.busy || state.locked}
          >
            {t(remove ? "remove" : "save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export function SaveSearchButton({
  definition,
}: {
  definition: SearchDefinition;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState<SearchDefinition>();
  return (
    <>
      <Tooltip label={t("saveSearch")}>
        <ActionIcon
          size="lg"
          variant="default"
          aria-label={t("saveSearch")}
          onClick={() => setDraft(structuredClone(definition))}
        >
          <StarIcon />
        </ActionIcon>
      </Tooltip>
      {draft && (
        <SearchEditor definition={draft} close={() => setDraft(undefined)} />
      )}
    </>
  );
}

export function SavedSearches({
  current,
  open,
}: {
  current?: SearchDefinition;
  open: (route: Route) => void;
}) {
  const { t } = useI18n();
  const query = useQuery({
    queryKey: ["saved-searches"],
    queryFn: ({ signal }) =>
      get<components["schemas"]["SavedSearchList"]>("/saved-searches", signal),
  });
  const [editor, setEditor] = useState<{
    item: SavedSearch;
    definition: SearchDefinition;
    remove?: boolean;
  }>();
  const context = useContextMenu<SavedSearch>();
  const [error, setError] = useState("");
  const [opening, setOpening] = useState(false);
  async function apply(item: SavedSearch) {
    if (opening) return;
    setOpening(true);
    setError("");
    try {
      const [tags, boxes, folders] = await Promise.all([
        api.tags(),
        api.boxes(),
        item.definition.folder_id && item.definition.box_id
          ? api.folders(item.definition.box_id)
          : Promise.resolve(undefined),
      ]);
      if (
        !searchReferencesExist(
          item.definition,
          tags.tags || [],
          boxes.boxes || [],
          folders?.folders || [],
        )
      ) {
        setError(t("savedSearchMissing"));
        return;
      }
      open(savedSearchRoute(item.definition));
    } catch (err) {
      setError(errorMessage(err, t));
    } finally {
      setOpening(false);
    }
  }
  const items = query.data?.searches || [];
  return (
    <>
      {query.isError && (
        <div role="alert">
          {errorMessage(query.error, t)}{" "}
          <Button size="compact-xs" onClick={() => query.refetch()}>
            {t("retry")}
          </Button>
        </div>
      )}
      {!!items.length && (
        <details className="sidebar-tags saved-searches" open>
          <summary>
            <svg
              className="sidebar-tags-chevron"
              width="12"
              height="12"
              viewBox="0 0 16 16"
              fill="none"
              aria-hidden="true"
            >
              <path
                d="m6 3 5 5-5 5"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
            {t("savedSearches")}
          </summary>
          <ul>
            {items.map((item) => (
              <li
                key={item.id}
                className="saved-search-row"
                data-active={
                  context.menu?.item.id === item.id ||
                  editor?.item.id === item.id ||
                  undefined
                }
                {...context.bindings(item)}
              >
                <UnstyledButton
                  className="saved-search-link"
                  disabled={opening}
                  onClick={() => apply(item)}
                  title={item.name}
                >
                  <StarIcon />
                  <bdi>{item.name}</bdi>
                </UnstyledButton>
                <SidebarMenuButton
                  label={`${item.name} · ${t("editSavedSearch")}`}
                  expanded={context.menu?.item.id === item.id}
                  onClick={(event) => {
                    if (context.menu?.item.id === item.id) context.close();
                    else context.atElement(item, event.currentTarget);
                  }}
                />
              </li>
            ))}
          </ul>
          {opening && <p role="status">{t("loading")}</p>}
          {error && <p role="alert">{error}</p>}
        </details>
      )}
      <ContextMenu anchor={context.menu} close={context.close}>
        <Menu.Item
          disabled={opening}
          onClick={() => {
            if (context.menu) void apply(context.menu.item);
          }}
        >
          {t("openItem")}
        </Menu.Item>
        <Menu.Item
          onClick={() => {
            if (context.menu)
              setEditor({
                item: context.menu.item,
                definition: context.menu.item.definition,
              });
          }}
        >
          {t("rename")}
        </Menu.Item>
        <Menu.Item
          disabled={!current}
          onClick={() => {
            if (context.menu && current)
              setEditor({
                item: context.menu.item,
                definition: structuredClone(current),
              });
          }}
        >
          {t("replaceSavedSearch")}
        </Menu.Item>
        <Menu.Item
          color="red"
          onClick={() => {
            if (context.menu)
              setEditor({
                item: context.menu.item,
                definition: context.menu.item.definition,
                remove: true,
              });
          }}
        >
          {t("removeSavedSearch")}
        </Menu.Item>
      </ContextMenu>
      {editor && (
        <SearchEditor {...editor} close={() => setEditor(undefined)} />
      )}
    </>
  );
}
