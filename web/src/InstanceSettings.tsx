import { BackgroundTasks } from "./BackgroundTasks";
import { loadSettingsPage, settingsQueries } from "./settingsQueries";
import {
  SettingsFieldHeading,
  SettingsReset,
  hasSettingChange,
  type SettingsSectionDraft,
} from "./SettingsFields";
import { LocationSettings } from "./LocationSettings";
import { StorageUsageValues } from "./StorageUsageValues";
import { ColorChoiceGroup } from "./ColorChoiceGroup";
import { CacheLimitInput } from "./CacheLimitInput";
import { AIConnection } from "./AI";
import { EmptyHint } from "./EmptyHint";
import { HelpTooltip } from "./HelpTooltip";
import { FullTextTooltip } from "./FullTextTooltip";
import { SectionIndex } from "./SectionIndex";
import { Dialog, DialogPresence } from "./Dialog";
import type {
  NavigationTransition,
  RegisterNavigationBlocker,
} from "./navigation";
import {
  Alert,
  Button,
  Checkbox,
  Group,
  Input,
  NumberInput,
  PasswordInput,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
} from "@mantine/core";
import {
  useEffect,
  useRef,
  useState,
  useId,
  type CSSProperties,
  type ReactNode,
} from "react";
import { useElementSize } from "@mantine/hooks";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { APIError, result } from "./api";
import type { components } from "./api-schema";
import { useI18n, formatInstant, type MessageKey } from "./i18n";
import { errorMessage } from "./mutations";
import { Select } from "./Select";

type SettingsData = components["schemas"]["SettingsResponse"];
type SettingField = NonNullable<SettingsData["fields"]>[number];
const labels: Record<string, MessageKey> = {
  "appearance.primary_color": "primaryColor",
  "library.language": "settingLibraryLanguage",
  "library.timezone": "settingLibraryTimezone",
  "storage.link_mode": "settingLinkMode",
  "client.cache_max_age": "clientMaxAge",
  "client.cache_max_bytes": "settingCacheBytes",
  "video.cache_max_age": "settingVideoCacheAge",
  "video.cache_max_bytes": "settingVideoCacheBytes",
  "previews.cache_max_age": "settingPreviewCacheAge",
  "previews.cache_max_bytes": "settingPreviewCacheBytes",
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

export function InstanceSettings({
  registerNavigationBlocker,
}: { registerNavigationBlocker?: RegisterNavigationBlocker } = {}) {
  const { ref: headerRef, height: headerHeight } = useElementSize();
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useQuery({
    queryKey: ["instance-settings"],
    queryFn: ({ signal }) => loadSettingsPage(queries, signal),
  });
  const [snapshot, setSnapshot] = useState<SettingsData>();
  const [draftChanges, setChanges] = useState<Record<string, unknown>>({});
  const changes = Object.fromEntries(
    Object.entries(draftChanges).filter(([key, value]) => {
      const field = snapshot?.fields?.find((field) => field.key === key);
      return !field || hasSettingChange(field, value);
    }),
  );
  const [aiPending, setAIPending] = useState(false);
  const [locationPending, setLocationPending] = useState(false);
  const [aiDraft, setAIDraft] = useState<SettingsSectionDraft | null>(null);
  const [locationDraft, setLocationDraft] =
    useState<SettingsSectionDraft | null>(null);
  const [reloadVersion, setReloadVersion] = useState(0);
  const unsaved =
    Object.keys(changes).length > 0 || !!aiDraft || !!locationDraft;
  const valid = (aiDraft?.valid ?? true) && (locationDraft?.valid ?? true);
  const [busy, setBusy] = useState(false);
  const saveHint = busy
    ? ""
    : !valid
      ? t("settingsSaveInvalid")
      : !unsaved
        ? t("settingsSaveNoChanges")
        : "";
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [leaving, setLeaving] = useState(false);
  const pendingNavigation = useRef<NavigationTransition | null>(null);
  const allowLeave = useRef(false);
  const unsavedRef = useRef(unsaved);
  unsavedRef.current = unsaved;
  const requestLeave = useRef<(transition: NavigationTransition) => boolean>(
    () => false,
  );
  requestLeave.current = (transition) => {
    if (!unsavedRef.current || allowLeave.current) return false;
    if (!pendingNavigation.current) pendingNavigation.current = transition;
    setLeaving(true);
    return true;
  };
  useEffect(
    () =>
      registerNavigationBlocker?.((transition) =>
        requestLeave.current(transition),
      ),
    [registerNavigationBlocker],
  );
  useEffect(() => {
    if (!unsaved) return;
    const unload = (event: BeforeUnloadEvent) => {
      if (!unsavedRef.current || allowLeave.current) return;
      event.preventDefault();
      event.returnValue = "";
    };
    const link = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const target =
        event.target instanceof Element
          ? event.target.closest("a[href]")
          : null;
      if (
        !(target instanceof HTMLAnchorElement) ||
        target.hasAttribute("download") ||
        (target.target && target.target !== "_self")
      )
        return;
      const href = new URL(target.href);
      if (!["http:", "https:"].includes(href.protocol)) return;
      if (
        href.origin === location.origin &&
        href.pathname === location.pathname &&
        href.search === location.search
      )
        return;
      if (
        requestLeave.current({
          href: href.href,
          proceed: () => location.assign(href.href),
        })
      ) {
        event.preventDefault();
        event.stopPropagation();
      }
    };
    window.addEventListener("beforeunload", unload);
    document.addEventListener("click", link, true);
    return () => {
      window.removeEventListener("beforeunload", unload);
      document.removeEventListener("click", link, true);
    };
  }, [unsaved]);
  function continueEditing() {
    if (busy) return;
    pendingNavigation.current = null;
    setLeaving(false);
  }
  function leave(reloadDocument = false) {
    const next = pendingNavigation.current;
    if (!next) return;
    allowLeave.current = true;
    pendingNavigation.current = null;
    setLeaving(false);
    if (reloadDocument) location.assign(next.href);
    else next.proceed();
  }
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
    const value = await loadSettingsPage(queries, signal, true);
    setSnapshot(value);
    setChanges({});
    setAIDraft(null);
    setLocationDraft(null);
    setReloadVersion((old) => old + 1);
    queries.setQueryData(["instance-settings"], value);
    await queries.invalidateQueries({ queryKey: ["info"] });
  };
  const save = async (skipDocumentReload = false) => {
    let saved = false;
    await run(async (signal) => {
      if (!snapshot) return;
      const value = await request<SettingsData>(
        "/settings",
        "PUT",
        {
          revision: snapshot.revision,
          ai: aiDraft?.value,
          location: locationDraft?.value,
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
      queries.setQueryData(["instance-settings"], value);
      await reload(signal);
      await queries.invalidateQueries({ queryKey: ["client"] });
      if (aiDraft) await queries.invalidateQueries({ queryKey: ["ai-models"] });
      setNotice(t("settingsSaved"));
      saved = true;
      // A new document is needed for the updated map resource CSP.
      if (
        !skipDocumentReload &&
        Object.keys(changes).some((key) => key.startsWith("server.map_"))
      ) {
        allowLeave.current = true;
        window.location.reload();
      }
    });
    return saved;
  };
  function renderGroup(group: string) {
    if (!snapshot) return null;
    return (
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
          {group === "storage" && (
            <StorageUsage canClearVideo={snapshot.mode === "library"} />
          )}
          {(snapshot.fields || [])
            .filter((field) => category(field) === group)
            .map((field) => {
              const changed = Object.hasOwn(draftChanges, field.key);
              const value = changed
                ? draftChanges[field.key] === null
                  ? field.default
                  : draftChanges[field.key]
                : field.value;
              const overridden = (snapshot.overrides || []).includes(field.key);
              return (
                <div key={field.key} className="setting-field">
                  <SettingInput
                    field={field}
                    value={value}
                    disabled={busy}
                    action={
                      <SettingsReset
                        specified={
                          changed
                            ? draftChanges[field.key] !== null
                            : field.explicit
                        }
                        disabled={
                          busy ||
                          (changed
                            ? draftChanges[field.key] === null
                            : !field.explicit)
                        }
                        onReset={() =>
                          setChanges((old) => ({ ...old, [field.key]: null }))
                        }
                      />
                    }
                    change={(value) =>
                      setChanges((old) => ({
                        ...old,
                        [field.key]: value,
                      }))
                    }
                  />
                  {overridden && (
                    <div className="setting-field-footer">
                      <Text size="sm" c="var(--mantine-primary-color-filled)">
                        {t("settingOverride")}{" "}
                        {typeof snapshot.effective[field.key] === "string"
                          ? String(snapshot.effective[field.key])
                          : JSON.stringify(snapshot.effective[field.key])}
                      </Text>
                    </div>
                  )}
                  {!overridden &&
                    (snapshot.restart_required || []).includes(field.key) && (
                      <Text size="sm" c="dimmed">
                        {t("settingRestartPending")}
                      </Text>
                    )}
                </div>
              );
            })}
        </Stack>
      </section>
    );
  }
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
          <FullTextTooltip text={saveHint} disabled={!saveHint}>
            <span
              className="tooltip-target mantine-focus-auto"
              tabIndex={saveHint ? 0 : undefined}
            >
              <Button
                variant="filled"
                loading={busy}
                disabled={!unsaved || !valid}
                onClick={() => void save()}
              >
                {t("save")}
              </Button>
            </span>
          </FullTextTooltip>
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
                ...(snapshot.mode === "client"
                  ? [{ id: "settings-maps", label: t("settingMaps") }]
                  : []),
                ...(snapshot.mode === "client"
                  ? [
                      {
                        id: "settings-connection",
                        label: t("clientConnection"),
                      },
                    ]
                  : []),
                { id: "settings-access", label: t("settingCredentials") },
                ...(snapshot.mode === "library"
                  ? [
                      { id: "settings-ai", label: t("aiConnection") },
                      { id: "settings-location", label: t("geoTitle") },
                      { id: "settings-maps", label: t("settingMaps") },
                      {
                        id: "settings-background-tasks",
                        label: t("backgroundTasks"),
                      },
                    ]
                  : []),
              ]}
            >
              {["appearance", "storage", "service"].map(renderGroup)}
              {snapshot.mode === "client" && renderGroup("maps")}
              {snapshot.mode === "client" && (
                <section
                  id="settings-connection"
                  className="flat-page-section"
                  aria-labelledby="settings-connection-title"
                >
                  <h2 id="settings-connection-title">
                    {t("clientConnection")}
                  </h2>
                  {Object.keys(changes).length > 0 && (
                    <Text size="sm" c="dimmed" mb="md">
                      {t("settingsPending")}
                    </Text>
                  )}
                  {snapshot.mode === "client" && (
                    <ConnectionSettings
                      key={snapshot.revision}
                      settings={snapshot}
                      busy={busy || unsaved}
                      run={run}
                      saved={reload}
                    />
                  )}
                </section>
              )}
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
                <Credentials busy={busy || unsaved} run={run} saved={reload} />
              </section>
              {snapshot.mode === "library" && (
                <AIConnection
                  onPendingChange={setAIPending}
                  disabled={busy}
                  configurationPending={unsaved}
                  onDraftChange={setAIDraft}
                  reloadVersion={reloadVersion}
                  onChanged={async () => {
                    if (Object.keys(changes).length === 0)
                      await run(async (signal) => reload(signal));
                  }}
                />
              )}
              {snapshot.mode === "library" && (
                <LocationSettings
                  onPendingChange={setLocationPending}
                  onDraftChange={setLocationDraft}
                  reloadVersion={reloadVersion}
                  disabled={busy}
                />
              )}
              {snapshot.mode === "library" && renderGroup("maps")}
              {snapshot.mode === "library" && (
                <BackgroundTasks
                  disabled={busy || unsaved}
                  aiPending={aiPending}
                  locationPending={locationPending}
                />
              )}
            </SectionIndex>
          </>
        )}
      </div>
      <DialogPresence>
        {leaving && (
          <Dialog
            open
            title={t("settingsUnsavedTitle")}
            description={t("settingsUnsavedDescription")}
            onOpenChange={(open) => {
              if (!open) continueEditing();
            }}
            size="md"
          >
            {error && (
              <Alert color="red" role="alert">
                {error}
              </Alert>
            )}
            <div className="actions">
              <Button data-autofocus disabled={busy} onClick={continueEditing}>
                {t("settingsContinueEditing")}
              </Button>
              <Button disabled={busy} onClick={() => leave()}>
                {t("settingsDiscardLeave")}
              </Button>
              <Button
                variant="filled"
                loading={busy}
                disabled={busy || !valid}
                onClick={async () => {
                  const reloadDocument = Object.keys(changes).some((key) =>
                    key.startsWith("server.map_"),
                  );
                  if (await save(true)) leave(reloadDocument);
                }}
              >
                {t("settingsSaveLeave")}
              </Button>
            </div>
          </Dialog>
        )}
      </DialogPresence>
    </section>
  );
}
function StorageUsage({ canClearVideo }: { canClearVideo: boolean }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [clearing, setClearing] = useState(false);
  const [clearError, setClearError] = useState<unknown>();
  const usage = useQuery({
    ...settingsQueries.storage,
    refetchOnWindowFocus: false,
  });
  return (
    <section aria-label={t("settingStorageUsage")} aria-busy={usage.isFetching}>
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
        {canClearVideo && (
          <Button
            size="compact-xs"
            variant="subtle"
            loading={clearing}
            onClick={async () => {
              setClearing(true);
              setClearError(undefined);
              try {
                await request("/settings/video-cache/clear", "POST", {});
                await client.invalidateQueries({
                  queryKey: settingsQueries.storage.queryKey,
                });
              } catch (error) {
                setClearError(error);
              } finally {
                setClearing(false);
              }
            }}
          >
            {t("settingClearVideoCache")}
          </Button>
        )}
      </Group>
      {usage.error && (
        <Text size="sm" c="red" role="alert">
          {errorMessage(usage.error, t)}
        </Text>
      )}
      {!!clearError && (
        <Text size="sm" c="red" role="alert">
          {errorMessage(clearError, t)}
        </Text>
      )}
      <StorageUsageValues
        items={[
          ["settingOriginalsUsage", usage.data?.originals_bytes],
          ["settingMetadataUsage", usage.data?.metadata_bytes],
          ["settingPreviewsUsage", usage.data?.previews_bytes],
          ["settingVideoUsage", usage.data?.video_bytes],
        ]}
      />
    </section>
  );
}
function SettingInput(props: {
  field: SettingField;
  value: unknown;
  disabled: boolean;
  change: (value: unknown) => void;
  action?: ReactNode;
}) {
  const { t } = useI18n();
  const id = useId();
  const { field } = props;
  const label = labels[field.key] ? t(labels[field.key]) : field.key;
  const hint = field.unlimited
    ? t(
        field.kind === "bytes"
          ? "settingCacheBytesHint"
          : "settingCacheAgeHint",
      )
    : field.key === "library.language"
      ? t("settingLanguageHint")
      : field.key === "client.cache_max_age" ||
          field.key === "video.cache_max_age"
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
      <SettingsFieldHeading
        label={label}
        htmlFor={id}
        help={hint}
        action={props.action}
      >
        {field.restart && (
          <Text size="xs" c="dimmed">
            {t("settingRestart")}
          </Text>
        )}
      </SettingsFieldHeading>
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
  if (field.unlimited)
    return (
      <CacheLimitInput
        id={id}
        label={label}
        disabled={disabled}
        value={
          typeof value === "number" ? editableBytes(value) : String(value ?? "")
        }
        onChange={change}
      />
    );
  if (field.key === "appearance.primary_color")
    return (
      <ColorChoiceGroup
        id={id}
        label={label}
        value={String(value)}
        onChange={change}
        disabled={disabled}
      />
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
    ...settingsQueries.credentials,
  });
  return (
    <Stack gap="md">
      {!creating && (
        <Group
          justify={query.data?.length === 0 ? "space-between" : "flex-end"}
        >
          {query.data?.length === 0 && (
            <EmptyHint>{t("settingCredentialsEmpty")}</EmptyHint>
          )}
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
      {!!query.data?.length && (
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
      )}
    </Stack>
  );
}
