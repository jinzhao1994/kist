import { HelpTooltip } from "./HelpTooltip";
import { SectionIndex } from "./SectionIndex";
import {
  Alert,
  Button,
  Checkbox,
  ColorSwatch,
  DEFAULT_THEME,
  Group,
  Input,
  NumberInput,
  PasswordInput,
  Radio,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { useEffect, useRef, useState, useId, type CSSProperties } from "react";
import { useElementSize } from "@mantine/hooks";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { APIError, get, result } from "./api";
import type { components } from "./api-schema";
import { useI18n, formatInstant, formatBytes, type MessageKey } from "./i18n";
import { errorMessage } from "./mutations";
import { Select } from "./Select";
import { colors } from "./theme";

type SettingsData = components["schemas"]["SettingsResponse"];
type SettingField = NonNullable<SettingsData["fields"]>[number];
const labels: Record<string, MessageKey> = {
  "appearance.primary_color": "primaryColor",
  "library.timezone": "settingLibraryTimezone",
  "storage.link_mode": "settingLinkMode",
  "client.cache_max_age": "clientMaxAge",
  "client.cache_max_bytes": "settingCacheBytes",
  "server.listen": "settingListen",
  "server.allowed_origins": "settingOrigins",
  "server.max_upload_bytes": "settingUploadBytes",
  "server.max_upload_files": "settingUploadFiles",
  "server.map_style": "settingMapStyle",
  "server.map_terrain_tiles": "settingTerrainTiles",
  "server.map_terrain_attribution": "settingTerrainAttribution",
  "server.map_resource_origins": "settingMapOrigins",
};
function category(field: SettingField) {
  return field.key.startsWith("appearance.")
    ? "appearance"
    : field.key.startsWith("server.map_")
      ? "maps"
      : field.key.startsWith("server.")
        ? "service"
        : "storage";
}
async function request<T>(
  path: string,
  method: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  return result<T>(
    await fetch(`/api/v1${path}`, {
      method,
      signal,
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  );
}

export function InstanceSettings() {
  const { ref: headerRef, height: headerHeight } = useElementSize();
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useQuery({
    queryKey: ["instance-settings"],
    queryFn: ({ signal }) => get<SettingsData>("/settings", signal),
  });
  const [snapshot, setSnapshot] = useState<SettingsData>();
  const [changes, setChanges] = useState<Record<string, unknown>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (!snapshot && query.data) setSnapshot(query.data);
  }, [snapshot, query.data]);
  async function run(command: (signal: AbortSignal) => Promise<void>) {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await command(controller.signal);
    } catch (err) {
      if (!controller.signal.aborted)
        setError(
          err instanceof APIError && err.status === 409
            ? t("settingsConflict")
            : errorMessage(err, t),
        );
    } finally {
      active.current = null;
      setBusy(false);
    }
  }
  const reload = async (signal?: AbortSignal) => {
    const value = await get<SettingsData>("/settings", signal);
    setSnapshot(value);
    setChanges({});
    queries.setQueryData(["instance-settings"], value);
    await queries.invalidateQueries({ queryKey: ["info"] });
  };
  const save = () =>
    run(async (signal) => {
      if (!snapshot) return;
      const value = await request<SettingsData>(
        "/settings",
        "PUT",
        {
          revision: snapshot.revision,
          changes: Object.fromEntries(
            Object.entries(changes).map(([key, value]) => [
              key,
              Array.isArray(value) &&
              (snapshot.fields || []).find((field) => field.key === key)
                ?.kind === "list"
                ? value.map((item: string) => item.trim()).filter(Boolean)
                : value,
            ]),
          ),
        },
        signal,
      );
      setSnapshot(value);
      setChanges({});
      queries.setQueryData(["instance-settings"], value);
      await queries.invalidateQueries({ queryKey: ["info"] });
      await queries.invalidateQueries({ queryKey: ["client"] });
      setNotice(t("settingsSaved"));
      // A new document is needed for the updated map resource CSP.
      if (Object.keys(changes).some((key) => key.startsWith("server.map_")))
        window.location.reload();
    });
  return (
    <section
      aria-label={t("instanceSettings")}
      style={
        { "--section-top-offset": `${headerHeight + 40}px` } as CSSProperties
      }
    >
      <header ref={headerRef} className="page-header settings-page-header">
        <div>
          <h1>{t("instanceSettings")}</h1>
        </div>
        <Group className="header-actions">
          <Button
            disabled={busy}
            onClick={() => run((signal) => reload(signal))}
          >
            {t("settingsReload")}
          </Button>
          <Button
            variant="filled"
            loading={busy}
            disabled={!Object.keys(changes).length}
            onClick={save}
          >
            {t("save")}
          </Button>
        </Group>
      </header>
      <div className="instance-settings-content">
        {query.isPending && !snapshot && (
          <Text role="status">{t("loading")}</Text>
        )}
        {(error || query.error) && (
          <Alert color="red" role="alert">
            {error || errorMessage(query.error, t)}
          </Alert>
        )}
        {notice && (
          <Text role="status" size="sm">
            {notice}
          </Text>
        )}
        {snapshot && (
          <>
            <SectionIndex
              ready={headerHeight > 0}
              items={[
                { id: "settings-appearance", label: t("settingAppearance") },
                { id: "settings-storage", label: t("settingStorage") },
                { id: "settings-service", label: t("settingService") },
                { id: "settings-maps", label: t("settingMaps") },
                {
                  id: "settings-access",
                  label: t(
                    snapshot.mode === "client"
                      ? "clientConnection"
                      : "settingCredentials",
                  ),
                },
              ]}
            >
              {["appearance", "storage", "service", "maps"].map((group) => (
                <section
                  key={group}
                  id={`settings-${group}`}
                  className="flat-page-section"
                  aria-labelledby={`settings-${group}-title`}
                >
                  <h2 id={`settings-${group}-title`}>
                    {t(
                      group === "appearance"
                        ? "settingAppearance"
                        : group === "storage"
                          ? "settingStorage"
                          : group === "maps"
                            ? "settingMaps"
                            : "settingService",
                    )}
                  </h2>
                  <Stack gap="lg">
                    {group === "storage" && <StorageUsage />}
                    {(snapshot.fields || [])
                      .filter((field) => category(field) === group)
                      .map((field) => {
                        const changed = Object.hasOwn(changes, field.key);
                        const value = changed
                          ? changes[field.key] === null
                            ? field.default
                            : changes[field.key]
                          : field.value;
                        const overridden = (snapshot.overrides || []).includes(
                          field.key,
                        );
                        return (
                          <div key={field.key} className="instance-setting">
                            <SettingInput
                              field={field}
                              value={value}
                              disabled={busy}
                              change={(value) =>
                                setChanges((old) => ({
                                  ...old,
                                  [field.key]: value,
                                }))
                              }
                            />
                            <Group
                              className="setting-field-footer"
                              wrap="nowrap"
                              justify={
                                overridden ? "space-between" : "flex-end"
                              }
                              gap="xs"
                              mt="xs"
                            >
                              {overridden && (
                                <Text
                                  size="sm"
                                  c="var(--mantine-primary-color-filled)"
                                >
                                  {t("settingOverride")}{" "}
                                  {typeof snapshot.effective[field.key] ===
                                  "string"
                                    ? String(snapshot.effective[field.key])
                                    : JSON.stringify(
                                        snapshot.effective[field.key],
                                      )}
                                </Text>
                              )}
                              <Button
                                size="compact-xs"
                                variant="subtle"
                                disabled={
                                  busy ||
                                  (changed
                                    ? changes[field.key] === null
                                    : !field.explicit)
                                }
                                onClick={() =>
                                  setChanges((old) => ({
                                    ...old,
                                    [field.key]: null,
                                  }))
                                }
                              >
                                {t("settingReset")}
                              </Button>
                            </Group>
                            {!overridden &&
                              (snapshot.restart_required || []).includes(
                                field.key,
                              ) && (
                                <Text size="sm" c="dimmed">
                                  {t("settingRestartPending")}
                                </Text>
                              )}
                          </div>
                        );
                      })}
                  </Stack>
                </section>
              ))}
              {snapshot.mode === "client" && (
                <section
                  id="settings-access"
                  className="flat-page-section"
                  aria-labelledby="settings-access-title"
                >
                  <h2 id="settings-access-title">{t("clientConnection")}</h2>
                  {Object.keys(changes).length > 0 && (
                    <Text size="sm" c="dimmed" mb="md">
                      {t("settingsPending")}
                    </Text>
                  )}
                  {snapshot.mode === "client" && (
                    <ConnectionSettings
                      key={snapshot.revision}
                      settings={snapshot}
                      busy={busy || Object.keys(changes).length > 0}
                      run={run}
                      saved={reload}
                    />
                  )}
                </section>
              )}
              {snapshot.mode === "library" && (
                <section
                  id="settings-access"
                  className="flat-page-section"
                  aria-labelledby="settings-access-title"
                >
                  <h2
                    id="settings-access-title"
                    className="settings-section-heading"
                  >
                    {t("settingCredentials")}
                    <HelpTooltip label={t("settingCredentials")}>
                      {t("settingCredentialsHint")}
                    </HelpTooltip>
                  </h2>
                  {Object.keys(changes).length > 0 && (
                    <Text size="sm" c="dimmed" mb="md">
                      {t("settingsPending")}
                    </Text>
                  )}
                  {snapshot.mode === "library" && (
                    <Credentials
                      busy={busy || Object.keys(changes).length > 0}
                      run={run}
                      saved={reload}
                    />
                  )}
                </section>
              )}
            </SectionIndex>
          </>
        )}
      </div>
    </section>
  );
}
function StorageUsage() {
  const { t, prefs } = useI18n();
  const usage = useQuery({
    queryKey: ["instance-storage-usage"],
    queryFn: ({ signal }) =>
      get<components["schemas"]["StorageUsage"]>("/settings/storage", signal),
    refetchOnWindowFocus: false,
  });
  return (
    <section aria-label={t("settingStorageUsage")}>
      <Group gap="xs" mb="sm">
        <Text size="sm" fw={500}>
          {t("settingStorageUsage")}
        </Text>
        <HelpTooltip label={t("settingStorageUsage")}>
          {t("settingStorageUsageHint")}
        </HelpTooltip>
        <Button
          size="compact-xs"
          variant="subtle"
          loading={usage.isFetching}
          onClick={() => void usage.refetch()}
        >
          {t("refresh")}
        </Button>
      </Group>
      {usage.isPending && (
        <Text size="sm" c="dimmed" role="status">
          {t("loading")}
        </Text>
      )}
      {usage.error && (
        <Text size="sm" c="red" role="alert">
          {errorMessage(usage.error, t)}
        </Text>
      )}
      {usage.data && (
        <div className="storage-usage-values">
          {(
            [
              ["originals_bytes", "settingOriginalsUsage"],
              ["metadata_bytes", "settingMetadataUsage"],
              ["previews_bytes", "settingPreviewsUsage"],
            ] as const
          ).map(([key, label]) => (
            <div key={key}>
              <Text size="xs" c="dimmed">
                {t(label)}
              </Text>
              <Text>{formatBytes(usage.data[key], prefs)}</Text>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
function SettingInput(props: {
  field: SettingField;
  value: unknown;
  disabled: boolean;
  change: (value: unknown) => void;
}) {
  const { t } = useI18n();
  const id = useId();
  const { field } = props;
  const label = labels[field.key] ? t(labels[field.key]) : field.key;
  const hint =
    field.key === "client.cache_max_age"
      ? t("clientMaxAgeHint")
      : field.key === "storage.link_mode"
        ? t("settingLinkModeHint")
        : field.key === "library.timezone"
          ? t("settingTimezoneHint")
          : field.key === "server.allowed_origins"
            ? t("settingOriginsHint")
            : field.kind === "bytes"
              ? t("settingBytesHint")
              : field.kind === "list"
                ? t("settingOnePerLine")
                : undefined;
  return (
    <>
      <Group className="setting-field-heading" gap="xs" mb={4}>
        <Input.Label htmlFor={id}>{label}</Input.Label>
        {hint && <HelpTooltip label={label}>{hint}</HelpTooltip>}
        {field.restart && (
          <Text size="xs" c="dimmed">
            {t("settingRestart")}
          </Text>
        )}
      </Group>
      <SettingControl {...props} id={id} />
    </>
  );
}
function SettingControl({
  id,
  field,
  value,
  disabled,
  change,
}: {
  id: string;
  field: SettingField;
  value: unknown;
  disabled: boolean;
  change: (value: unknown) => void;
}) {
  const { t } = useI18n();
  const label = labels[field.key] ? t(labels[field.key]) : field.key;
  if (field.key === "appearance.primary_color")
    return (
      <Radio.Group
        id={id}
        aria-label={label}
        value={String(value)}
        onChange={change}
      >
        <Group gap="sm" mt="xs" wrap="wrap">
          {colors.map((color) => (
            <Radio
              key={color}
              value={color}
              disabled={disabled}
              color={color}
              label={
                <Group gap={6}>
                  <ColorSwatch
                    color={DEFAULT_THEME.colors[color][6]}
                    size={16}
                  />
                  {t(color)}
                </Group>
              }
            />
          ))}
        </Group>
      </Radio.Group>
    );
  if (field.kind === "select")
    return (
      <Select
        id={id}
        aria-label={label}
        value={String(value)}
        onValueChange={change}
        disabled={disabled}
        options={(field.options || []).map((value) => ({
          value,
          label: value || t("auto"),
        }))}
      />
    );
  if (field.kind === "bytes")
    return (
      <TextInput
        id={id}
        aria-label={label}
        value={
          typeof value === "number" ? editableBytes(value) : String(value ?? "")
        }
        onChange={(e) => change(e.currentTarget.value)}
        disabled={disabled}
      />
    );
  if (field.kind === "integer")
    return (
      <NumberInput
        id={id}
        aria-label={label}
        value={typeof value === "number" ? value : ""}
        onChange={change}
        disabled={disabled}
        min={1}
        max={Number.MAX_SAFE_INTEGER}
        allowDecimal={false}
      />
    );
  if (field.kind === "list")
    return (
      <Textarea
        id={id}
        aria-label={label}
        value={Array.isArray(value) ? value.join("\n") : ""}
        onChange={(e) => change(e.currentTarget.value.split("\n"))}
        disabled={disabled}
        autosize
        minRows={2}
      />
    );

  return (
    <TextInput
      id={id}
      aria-label={label}
      value={String(value ?? "")}
      disabled={disabled}
      onChange={(e) => change(e.currentTarget.value)}
    />
  );
}
// Use exact units only: rounded display values must never change a limit when edited.
function editableBytes(value: number) {
  for (const [unit, size] of [
    ["TiB", 2 ** 40],
    ["TB", 10 ** 12],
    ["GiB", 2 ** 30],
    ["GB", 10 ** 9],
    ["MiB", 2 ** 20],
    ["MB", 10 ** 6],
    ["KiB", 2 ** 10],
    ["kB", 10 ** 3],
  ] as const) {
    if (value >= size && value % size === 0) return `${value / size} ${unit}`;
  }
  return `${value} B`;
}
type Run = (command: (signal: AbortSignal) => Promise<void>) => Promise<void>;
function ConnectionSettings({
  settings,
  busy,
  run,
  saved,
}: {
  settings: SettingsData;
  busy: boolean;
  run: Run;
  saved: (signal?: AbortSignal) => Promise<void>;
}) {
  const { t } = useI18n();
  const serverId = useId();
  const tokenId = useId();
  const [server, setServer] = useState(settings.connection?.server || "");
  const [token, setToken] = useState("");
  const [allowHTTP, setAllowHTTP] = useState(
    settings.connection?.allow_http || false,
  );
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void run(async (signal) => {
          await request(
            "/client/connection",
            "PUT",
            {
              revision: settings.revision,
              server,
              token,
              allow_http: allowHTTP,
            },
            signal,
          );
          setToken("");
          await saved(signal);
        });
      }}
    >
      <Stack gap="lg">
        <div>
          <Group className="setting-field-heading" gap="xs" mb={4}>
            <Input.Label htmlFor={serverId} required>
              {t("clientServer")}
            </Input.Label>
          </Group>
          <TextInput
            id={serverId}
            value={server}
            onChange={(e) => setServer(e.currentTarget.value)}
            disabled={busy}
            required
            type="url"
          />
        </div>
        <div>
          <Group className="setting-field-heading" gap="xs" mb={4}>
            <Input.Label htmlFor={tokenId} required>
              {t("clientToken")}
            </Input.Label>
            <HelpTooltip label={t("clientCredentialHelp")}>
              {t("clientConnectionHint")}
              {settings.credential_configured &&
                ` ${t("settingCredentialStored")}`}
            </HelpTooltip>
            {settings.credential_configured && (
              <Text size="xs" c="dimmed">
                {t("clientCredentialSaved")}
              </Text>
            )}
          </Group>
          <PasswordInput
            id={tokenId}
            value={token}
            onChange={(e) => setToken(e.currentTarget.value)}
            disabled={busy}
            required
            autoComplete="off"
          />
        </div>
        <Checkbox
          label={t("clientAllowHTTP")}
          size="sm"
          checked={allowHTTP}
          onChange={(e) => setAllowHTTP(e.currentTarget.checked)}
          disabled={busy}
        />
        <Group>
          <Button type="submit" disabled={busy}>
            {t("clientConnect")}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
function Credentials({
  busy,
  run,
  saved,
}: {
  busy: boolean;
  run: Run;
  saved: (signal?: AbortSignal) => Promise<void>;
}) {
  const { t, prefs } = useI18n();
  const queries = useQueryClient();
  const [secret, setSecret] = useState("");
  const [note, setNote] = useState("");
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<{ id: string; note: string }>();
  const query = useQuery({
    queryKey: ["sync-credentials"],
    queryFn: ({ signal }) =>
      get<components["schemas"]["SyncCredential"][]>(
        "/settings/credentials",
        signal,
      ),
  });
  return (
    <Stack gap="md">
      {!creating && (
        <Group justify="flex-end">
          <Button
            disabled={busy || !!editing}
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
            void run(async (signal) => {
              const result = await request<{ token: string }>(
                "/settings/credentials",
                "POST",
                { note },
                signal,
              );
              setSecret(result.token);
              setNote("");
              setCreating(false);
              await queries.invalidateQueries({
                queryKey: ["sync-credentials"],
              });
              await saved(signal);
            });
          }}
        >
          <TextInput
            label={t("settingCredentialNote")}
            placeholder={t("settingCredentialNoteExample")}
            value={note}
            onChange={(event) => setNote(event.currentTarget.value)}
            maxLength={200}
            disabled={busy}
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
      {secret && (
        <Alert>
          <Text size="sm">{t("settingCredentialOnce")}</Text>
          <code className="setting-secret">{secret}</code>
        </Alert>
      )}
      <Table.ScrollContainer minWidth={720}>
        <Table
          highlightOnHover
          verticalSpacing="xs"
          horizontalSpacing="sm"
          className="credential-table"
        >
          <Table.Thead>
            <Table.Tr>
              <Table.Th>{t("settingCredentialNoteHeading")}</Table.Th>
              <Table.Th>{t("settingCredentialDetails")}</Table.Th>
              <Table.Th>{t("settingCredentialCreated")}</Table.Th>
              <Table.Th>
                <span className="sr-only">{t("settingCredentialActions")}</span>
              </Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {query.data?.length === 0 && (
              <Table.Tr>
                <Table.Td colSpan={4}>
                  <Text size="sm" c="dimmed">
                    {t("settingCredentialsEmpty")}
                  </Text>
                </Table.Td>
              </Table.Tr>
            )}
            {query.data?.map((item) => (
              <Table.Tr key={item.id} className="credential-row">
                <Table.Td>
                  {editing?.id === item.id ? (
                    <TextInput
                      aria-label={t("settingCredentialNote")}
                      value={editing.note}
                      maxLength={200}
                      disabled={busy}
                      onChange={(event) =>
                        setEditing({
                          id: item.id,
                          note: event.currentTarget.value,
                        })
                      }
                      autoFocus
                    />
                  ) : (
                    <Text size="sm" className="credential-note">
                      {item.note || t("settingCredentialUnnamed")}
                    </Text>
                  )}
                </Table.Td>
                <Table.Td className="credential-id">
                  <Text size="xs" c="dimmed" ff="monospace">
                    {item.id}
                  </Text>
                </Table.Td>
                <Table.Td className="credential-created">
                  <Text size="xs" c="dimmed">
                    {formatInstant(item.created_at, prefs)}
                  </Text>
                </Table.Td>
                <Table.Td>
                  <Group gap="xs" justify="flex-end" wrap="nowrap">
                    {editing?.id === item.id ? (
                      <>
                        <Button
                          size="compact-sm"
                          variant="subtle"
                          disabled={busy}
                          onClick={() =>
                            void run(async (signal) => {
                              await request(
                                `/settings/credentials/${encodeURIComponent(item.id)}`,
                                "PUT",
                                { note: editing.note },
                                signal,
                              );
                              setEditing(undefined);
                              await queries.invalidateQueries({
                                queryKey: ["sync-credentials"],
                              });
                              await saved(signal);
                            })
                          }
                        >
                          {t("save")}
                        </Button>
                        <Button
                          size="compact-sm"
                          variant="subtle"
                          disabled={busy}
                          onClick={() => setEditing(undefined)}
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
                            setEditing({ id: item.id, note: item.note || "" })
                          }
                        >
                          {t("settingEditCredentialNote")}
                        </Button>
                        <Button
                          size="compact-sm"
                          variant="subtle"
                          disabled={busy || !!editing || creating}
                          color="red"
                          onClick={() =>
                            void run(async (signal) => {
                              await request(
                                `/settings/credentials/${encodeURIComponent(item.id)}`,
                                "DELETE",
                                undefined,
                                signal,
                              );
                              setSecret("");
                              await queries.invalidateQueries({
                                queryKey: ["sync-credentials"],
                              });
                              await saved(signal);
                            })
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
    </Stack>
  );
}
