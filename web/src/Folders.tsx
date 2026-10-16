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
  folders,
  current,
  choose,
}: {
  rootLabel?: string;
  folders: Folder[];
  current: string;
  choose: (id: string) => void;
}) {
  const { t } = useI18n();
  const chain = folderChain(folders, current);
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
        {chain.map((folder) =>
          !folder.parent_id ? (
            <Anchor
              component="button"
              className="breadcrumb-root"
              key={folder.id}
              onClick={() => choose(folder.id)}
              aria-label={t("boxRoot")}
              title={
                rootLabel
                  ? `${t("returnToBoxRoot")} · ${rootLabel}`
                  : t("returnToBoxRoot")
              }
            >
              <NavigationIcon kind="boxes" />
            </Anchor>
          ) : folder.id === current ? (
            <span key={folder.id} aria-current="page">
              <bdi>{folder.name}</bdi>
            </span>
          ) : (
            <Anchor
              component="button"
              key={folder.id}
              onClick={() => choose(folder.id)}
            >
              <bdi>{folder.name}</bdi>
            </Anchor>
          ),
        )}
      </Breadcrumbs>
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
