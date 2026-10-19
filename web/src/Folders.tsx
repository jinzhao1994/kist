import { ContextMenu, useContextMenu } from "./ContextMenu";
import { LinkMenuItems } from "./LinkMenuItems";
import { routeURL } from "./navigation";
import { SidebarMenuButton } from "./SidebarMenuButton";
import { NavigationIcon } from "./NavigationIcon";
import { Anchor, Breadcrumbs, Button, TextInput } from "@mantine/core";
import { useState } from "react";
import { createWrite, type Folder } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";

export function folderChain(folders: Folder[], id: string): Folder[] {
  const byID = new Map(folders.map((folder) => [folder.id, folder]));
  const seen = new Set<string>();
  const chain: Folder[] = [];
  let current = byID.get(id);
  while (current && !seen.has(current.id)) {
    seen.add(current.id);
    chain.unshift(current);
    if (!current.parent_id) return chain;
    current = byID.get(current.parent_id);
  }
  return [];
}
export function FolderBreadcrumbs({
  rootLabel,
  box,
  folders,
  current,
  choose,
}: {
  rootLabel?: string;
  box?: string;
  folders: Folder[];
  current: string;
  choose: (id: string) => void;
}) {
  const { t } = useI18n();
  const chain = folderChain(folders, current);
  const context = useContextMenu<string>();
  return (
    <nav className="folder-breadcrumbs" aria-label={t("folderPath")}>
      <Breadcrumbs
        separator={
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="m9 6 6 6-6 6" />
          </svg>
        }
        separatorMargin="xs"
        className="folder-path-segments"
      >
        {chain.map((folder) => (
          <span className="breadcrumb-segment" key={folder.id}>
            <Anchor
              component="button"
              className={!folder.parent_id ? "breadcrumb-root" : undefined}
              {...(box ? context.bindings(folder.id) : {})}
              onClick={() => choose(folder.id)}
              aria-current={folder.id === current ? "page" : undefined}
              aria-label={!folder.parent_id ? t("boxRoot") : undefined}
              title={
                !folder.parent_id
                  ? `${t("returnToBoxRoot")}${rootLabel ? ` · ${rootLabel}` : ""}`
                  : undefined
              }
            >
              {!folder.parent_id ? (
                <NavigationIcon kind="boxes" />
              ) : (
                <bdi>{folder.name}</bdi>
              )}
            </Anchor>
            {box && (
              <SidebarMenuButton
                label={t("moreActions")}
                expanded={context.menu?.item === folder.id}
                onClick={(event) =>
                  context.atElement(folder.id, event.currentTarget)
                }
              />
            )}
          </span>
        ))}
      </Breadcrumbs>
      <ContextMenu anchor={context.menu} close={context.close}>
        {box && context.menu && (
          <LinkMenuItems
            key={context.menu.item}
            href={routeURL({ files: true, box, folder: context.menu.item })}
          />
        )}
      </ContextMenu>
    </nav>
  );
}
export function FolderEditor({
  box,
  parent,
  close,
}: {
  box: string;
  parent: string;
  close: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const state = useWrites(close);
  return (
    <Dialog
      open
      title={t("createFolder")}
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!state.locked)
            void state.run([
              createWrite("POST", `/boxes/${box}/folders`, { parent, name }),
            ]);
        }}
      >
        <TextInput
          label={t("name")}
          value={name}
          onChange={(event) => setName(event.currentTarget.value)}
          required
          maxLength={255}
          autoFocus
          disabled={state.busy || state.locked}
        />
        <WriteFeedback state={state} />
        <div className="actions">
          <Button type="button" disabled={state.busy} onClick={close}>
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            variant="filled"
            disabled={state.busy || state.locked}
          >
            {t(state.busy ? "saving" : "createFolder")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
