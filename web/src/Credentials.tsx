import {
  Alert,
  Button,
  Group,
  Stack,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { useContext, useEffect, useMemo, useRef, useState } from "react";
import type { components } from "./api-schema";
import { EmptyHint } from "./EmptyHint";
import {
  SettingsErrorsContext,
  useSettingsError,
  type SettingsSectionDraft,
} from "./SettingsFields";
import { settingsQueries } from "./settingsQueries";
import { errorMessage } from "./mutations";
import { formatInstant, useI18n } from "./i18n";

type Credential = components["schemas"]["SyncCredential"];
type Entry = Omit<Credential, "created_at"> & {
  key: string;
  created_at?: string;
};
const values = (items: Entry[]) =>
  items.map(({ id, note }) => ({ id, note: (note || "").trim() }));

// Credentials remain a page draft until the single settings request succeeds.
export function Credentials({
  busy,
  onDraftChange,
  reloadVersion,
  created,
}: {
  busy: boolean;
  onDraftChange: (draft: SettingsSectionDraft | null) => void;
  reloadVersion: number;
  created: components["schemas"]["NewSyncCredential"][];
}) {
  const { t, prefs } = useI18n();
  const query = useQuery(settingsQueries.credentials);
  const [draft, setDraft] = useState<Entry[]>();
  const [touched, setTouched] = useState(false);
  const [creating, setCreating] = useState(false);
  const [note, setNote] = useState("");
  const [editing, setEditing] = useState<{ key: string; before: string }>();
  const serial = useRef(0);
  const { errors, clear } = useContext(SettingsErrorsContext);
  useEffect(() => {
    if (query.data && !touched)
      setDraft(query.data.map((item) => ({ ...item, key: item.id })));
  }, [query.data, touched]);
  useEffect(() => {
    setTouched(false);
    setCreating(false);
    setNote("");
    setEditing(undefined);
  }, [reloadVersion]);
  const pending = useMemo(
    () =>
      values([
        ...(draft || []),
        ...(creating ? [{ id: "", key: "creating", note }] : []),
      ]),
    [draft, creating, note],
  );
  const dirty =
    !!query.data &&
    JSON.stringify(pending) !==
      JSON.stringify(
        values(query.data.map((item) => ({ ...item, key: item.id }))),
      );
  useEffect(() => {
    onDraftChange(dirty ? { value: { items: pending } } : null);
  }, [dirty, pending, onDraftChange]);
  useEffect(() => {
    const field = Object.keys(errors).find((key) =>
      key.startsWith("credentials.items."),
    );
    const index = Number(field?.split(".")[2]);
    const item = draft?.[index];
    if (item)
      setEditing((old) =>
        old?.key === item.key
          ? old
          : { key: item.key, before: item.note || "" },
      );
  }, [errors, draft]);
  function update(items: Entry[]) {
    Object.keys(errors)
      .filter((key) => key.startsWith("credentials."))
      .forEach(clear);
    setTouched(true);
    setDraft(items);
  }
  return (
    <Stack gap="md">
      {!creating && (
        <Group justify={!draft?.length ? "space-between" : "flex-end"}>
          {!draft?.length && !query.isPending && !query.error && (
            <EmptyHint>{t("settingCredentialsEmpty")}</EmptyHint>
          )}
          <Button
            disabled={busy || !!editing || !draft}
            onClick={() => setCreating(true)}
          >
            {t("settingCreateCredential")}
          </Button>
        </Group>
      )}
      {creating && (
        <form
          className="credential-create-form"
          onSubmit={(event) => {
            event.preventDefault();
            update([
              ...(draft || []),
              { id: "", key: `new-${++serial.current}`, note },
            ]);
            setCreating(false);
            setNote("");
          }}
        >
          <CredentialNote
            index={draft?.length || 0}
            value={note}
            onChange={setNote}
            disabled={busy}
            label
            autoFocus
          />
          <Group gap="xs">
            <Button type="submit" variant="filled" disabled={busy}>
              {t("settingCreateCredential")}
            </Button>
            <Button
              disabled={busy}
              onClick={() => {
                setCreating(false);
                setNote("");
              }}
            >
              {t("cancel")}
            </Button>
          </Group>
        </form>
      )}
      {query.error && <Text role="alert">{errorMessage(query.error, t)}</Text>}
      {!!created.length && (
        <Alert>
          <Text size="sm">{t("settingCredentialOnce")}</Text>
          {created.map((item) => (
            <div key={item.id}>
              <Text size="sm">
                {item.note || t("settingCredentialUnnamed")}
              </Text>
              <code className="setting-secret">{item.token}</code>
            </div>
          ))}
        </Alert>
      )}
      {!!draft?.length && (
        <Table.ScrollContainer minWidth={720}>
          <Table
            highlightOnHover
            verticalSpacing="xs"
            horizontalSpacing="sm"
            className="settings-table"
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t("settingCredentialNoteHeading")}</Table.Th>
                <Table.Th>{t("settingCredentialDetails")}</Table.Th>
                <Table.Th>{t("settingCredentialCreated")}</Table.Th>
                <Table.Th>
                  <span className="sr-only">
                    {t("settingCredentialActions")}
                  </span>
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {draft.map((item, index) => (
                <Table.Tr key={item.key} className="credential-row">
                  <Table.Td>
                    {editing?.key === item.key ? (
                      <CredentialNote
                        index={index}
                        value={item.note || ""}
                        disabled={busy}
                        autoFocus
                        onChange={(note) =>
                          update(
                            draft.map((row) =>
                              row.key === item.key ? { ...row, note } : row,
                            ),
                          )
                        }
                      />
                    ) : (
                      <Text size="sm" className="credential-note">
                        {item.note || t("settingCredentialUnnamed")}
                      </Text>
                    )}
                  </Table.Td>
                  <Table.Td className="credential-id">
                    <Text
                      size="xs"
                      c="dimmed"
                      ff={item.id ? "monospace" : undefined}
                    >
                      {item.id || t("settingCredentialPending")}
                    </Text>
                  </Table.Td>
                  <Table.Td className="credential-created">
                    <Text size="xs" c="dimmed">
                      {item.created_at
                        ? formatInstant(item.created_at, prefs)
                        : "—"}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" justify="flex-end" wrap="nowrap">
                      {editing?.key === item.key ? (
                        <>
                          <Button
                            size="compact-sm"
                            variant="subtle"
                            disabled={busy}
                            onClick={() => setEditing(undefined)}
                          >
                            {t("settingFinishCredential")}
                          </Button>
                          <Button
                            size="compact-sm"
                            variant="subtle"
                            disabled={busy}
                            onClick={() => {
                              update(
                                draft.map((row) =>
                                  row.key === item.key
                                    ? { ...row, note: editing.before }
                                    : row,
                                ),
                              );
                              setEditing(undefined);
                            }}
                          >
                            {t("cancel")}
                          </Button>
                        </>
                      ) : (
                        <>
                          <Button
                            size="compact-sm"
                            variant="subtle"
                            disabled={busy || !!editing || creating}
                            onClick={() =>
                              setEditing({
                                key: item.key,
                                before: item.note || "",
                              })
                            }
                          >
                            {t("settingEditCredentialNote")}
                          </Button>
                          <Button
                            size="compact-sm"
                            variant="subtle"
                            color="red"
                            disabled={busy || !!editing || creating}
                            onClick={() =>
                              update(
                                draft.filter((row) => row.key !== item.key),
                              )
                            }
                          >
                            {t("settingRevoke")}
                          </Button>
                        </>
                      )}
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Stack>
  );
}

function CredentialNote({
  index,
  value,
  onChange,
  disabled,
  label = false,
  autoFocus,
}: {
  index: number;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  label?: boolean;
  autoFocus?: boolean;
}) {
  const { t } = useI18n();
  const { clear } = useContext(SettingsErrorsContext);
  const path = `credentials.items.${index}.note`;
  const error = useSettingsError(path);
  return (
    <div className="setting-field" data-setting-field={path}>
      <TextInput
        label={label ? t("settingCredentialNote") : undefined}
        aria-label={t("settingCredentialNote")}
        placeholder={label ? t("settingCredentialNoteExample") : undefined}
        value={value}
        error={error}
        disabled={disabled}
        autoFocus={autoFocus}
        onChange={(event) => {
          clear(path);
          onChange(event.currentTarget.value);
        }}
      />
    </div>
  );
}
