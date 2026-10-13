import {
  Anchor,
  Breadcrumbs,
  Button,
  TextInput,
  UnstyledButton,
} from "@mantine/core";
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
  folders,
  current,
  choose,
}: {
  folders: Folder[];
  current: string;
  choose: (id: string) => void;
}) {
  const { t } = useI18n();
  return (
    <Breadcrumbs className="folder-breadcrumbs" aria-label={t("folderPath")}>
      {folderChain(folders, current).map((folder) =>
        folder.id === current ? (
          <span key={folder.id} aria-current="page">
            <bdi>{folder.name || t("boxRoot")}</bdi>
          </span>
        ) : (
          <Anchor
            component="button"
            key={folder.id}
            onClick={() => choose(folder.id)}
          >
            <bdi>{folder.name || t("boxRoot")}</bdi>
          </Anchor>
        ),
      )}
    </Breadcrumbs>
  );
}
export function FolderCards({
  folders,
  choose,
}: {
  folders: Folder[];
  choose: (id: string) => void;
}) {
  return (
    <div className="folder-grid">
      {folders.map((folder) => (
        <UnstyledButton
          className="folder-card"
          key={folder.id}
          onClick={() => choose(folder.id)}
        >
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
          >
            <path d="M3 7V5a1 1 0 0 1 1-1h5l2 3h9a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V7Z" />
          </svg>
          <bdi>{folder.name}</bdi>
        </UnstyledButton>
      ))}
    </div>
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
