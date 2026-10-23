import { SectionIndex } from "./SectionIndex";
import {
  Alert,
  Button,
  Checkbox,
  ColorSwatch,
  DEFAULT_THEME,
  Group,
  NumberInput,
  PasswordInput,
  Radio,
  Stack,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { useElementSize } from "@mantine/hooks";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { APIError, get, result } from "./api";
import type { components } from "./api-schema";
import { useI18n, formatInstant, type MessageKey } from "./i18n";
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
          <Text size="sm" c="dimmed">
            {t("instanceSettingsHint")}
          </Text>
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
              items={[
                { id: "settings-appearance", label: t("settingAppearance") },
                { id: "settings-storage", label: t("settingStorage") },
                { id: "settings-service", label: t("settingService") },
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
              {["appearance", "storage", "service"].map((group) => (
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
                          : "settingService",
                    )}
                  </h2>
                  <Stack gap="lg">
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
                            <Group justify="space-between" gap="xs" mt="xs">
                              <Text size="xs" c="dimmed">
                                {t(
                                  (changed && changes[field.key] === null) ||
                                    (!changed && !field.explicit)
                                    ? "settingDefaultValue"
                                    : "settingCustomValue",
                                )}
                                {field.restart
                                  ? ` · ${t("settingRestart")}`
                                  : ""}
                              </Text>
                              <Button
                                size="compact-xs"
                                variant="subtle"
                                disabled={busy || (!field.explicit && !changed)}
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
                            {overridden && (
                              <Text size="sm" c="dimmed">
                                {t("settingOverride")}{" "}
                                {JSON.stringify(snapshot.effective[field.key])}
                              </Text>
                            )}
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
                    {group === "storage" && snapshot.mode === "library" && (
                      <Text size="sm" c="dimmed">
                        {t("settingTimezoneHint")}
                      </Text>
                    )}
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
                  <h2 id="settings-access-title">{t("settingCredentials")}</h2>
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
function SettingInput({
  field,
  value,
  disabled,
  change,
}: {
  field: SettingField;
  value: unknown;
  disabled: boolean;
  change: (value: unknown) => void;
}) {
  const { t } = useI18n();
  const label = labels[field.key] ? t(labels[field.key]) : field.key;
  if (field.key === "appearance.primary_color")
    return (
      <Radio.Group label={label} value={String(value)} onChange={change}>
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
        label={label}
        value={String(value)}
        onValueChange={change}
        disabled={disabled}
        options={(field.options || []).map((value) => ({
          value,
          label: value || t("auto"),
        }))}
      />
    );
  if (field.kind === "integer")
    return (
      <NumberInput
        label={label}
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
        label={label}
        description={t("settingOnePerLine")}
        value={Array.isArray(value) ? value.join("\n") : ""}
        onChange={(e) => change(e.currentTarget.value.split("\n"))}
        disabled={disabled}
        autosize
        minRows={2}
      />
    );

  return (
    <TextInput
      label={label}
      value={String(value ?? "")}
      disabled={disabled}
      onChange={(e) => change(e.currentTarget.value)}
    />
  );
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
      <Stack gap="md">
        <TextInput
          label={t("clientServer")}
          value={server}
          onChange={(e) => setServer(e.currentTarget.value)}
          disabled={busy}
          required
          type="url"
        />
        <PasswordInput
          label={t("clientToken")}
          description={
            settings.credential_configured
              ? t("settingCredentialStored")
              : undefined
          }
          value={token}
          onChange={(e) => setToken(e.currentTarget.value)}
          disabled={busy}
          required
          autoComplete="off"
        />
        <Checkbox
          label={t("clientAllowHTTP")}
          checked={allowHTTP}
          onChange={(e) => setAllowHTTP(e.currentTarget.checked)}
          disabled={busy}
        />
        <Text size="sm" c="dimmed">
          {t("clientConnectionHint")}
        </Text>
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
  const query = useQuery({
    queryKey: ["sync-credentials"],
    queryFn: ({ signal }) =>
      get<{ id: string; created_at: string }[]>(
        "/settings/credentials",
        signal,
      ),
  });
  return (
    <Stack gap="md">
      <Text size="sm" c="dimmed">
        {t("settingCredentialsHint")}
      </Text>
      {query.error && <Text role="alert">{errorMessage(query.error, t)}</Text>}
      {secret && (
        <Alert>
          <Text size="sm">{t("settingCredentialOnce")}</Text>
          <code className="setting-secret">{secret}</code>
        </Alert>
      )}
      {query.data?.map((item) => (
        <Group key={item.id} justify="space-between">
          <div>
            <Text size="sm">{item.id}</Text>
            <Text size="xs" c="dimmed">
              {formatInstant(item.created_at, prefs)}
            </Text>
          </div>
          <Button
            disabled={busy}
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
        </Group>
      ))}
      <Group>
        <Button
          disabled={busy}
          onClick={() =>
            void run(async (signal) => {
              const result = await request<{ token: string }>(
                "/settings/credentials",
                "POST",
                {},
                signal,
              );
              setSecret(result.token);
              await queries.invalidateQueries({
                queryKey: ["sync-credentials"],
              });
              await saved(signal);
            })
          }
        >
          {t("settingCreateCredential")}
        </Button>
      </Group>
    </Stack>
  );
}
