import { Credentials } from "./Credentials";
import { BackgroundTasks } from "./BackgroundTasks";
import { loadSettingsPage, settingsQueries } from "./settingsQueries";
import {
  SettingsFieldHeading,
  SettingsFieldRow,
  SettingsReset,
  hasSettingChange,
  SettingsErrorsContext,
  useSettingsError,
  type SettingsErrors,
  type SettingsSectionDraft,
} from "./SettingsFields";
import { LocationSettings } from "./LocationSettings";
import { StorageUsageValues } from "./StorageUsageValues";
import { ColorChoiceGroup } from "./ColorChoiceGroup";
import { CacheLimitInput } from "./CacheLimitInput";
import { FloatingFeedback } from "./FloatingFeedback";
import { AIConnection } from "./AI";
import { HelpTooltip } from "./HelpTooltip";
import { FullTextTooltip } from "./FullTextTooltip";
import { SectionIndex } from "./SectionIndex";
import { Dialog, DialogPresence } from "./Dialog";
import type {
  NavigationTransition,
  RegisterNavigationBlocker,
} from "./navigation";
import {
  Button,
  Checkbox,
  Group,
  Input,
  NumberInput,
  PasswordInput,
  Stack,
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
import { useI18n, type MessageKey } from "./i18n";
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

// Pair only settings with a shared purpose. Unknown text fields keep room for
// URLs or descriptive content instead of being squeezed by registry order.
function settingRows(fields: SettingField[]) {
  const rows = new Map<string, { field: SettingField; wide: boolean }[]>();
  for (const field of fields) {
    const cache = field.key.match(/^(.+)\.cache_max_(?:age|bytes)$/);
    const locale =
      field.key === "library.timezone" || field.key === "library.language";
    const upload = field.key.startsWith("server.max_upload_");
    const row = cache
      ? `${cache[1]}.cache_limits`
      : locale
        ? "library.locale"
        : upload
          ? "server.upload_limits"
          : field.key;
    const wide =
      (!cache &&
        !locale &&
        !upload &&
        field.key !== "server.listen" &&
        !["select", "integer", "bytes"].includes(field.kind)) ||
      field.key === "appearance.primary_color";
    const values = rows.get(row) || [];
    values.push({ field, wide });
    rows.set(row, values);
  }
  return [...rows.values()];
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
  // Subscribe to the already loaded registry without issuing another request.
  const services = useQuery({ ...settingsQueries.services, enabled: false });
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
  const [credentialsDraft, setCredentialsDraft] =
    useState<SettingsSectionDraft | null>(null);
  const [createdCredentials, setCreatedCredentials] = useState<
    components["schemas"]["NewSyncCredential"][]
  >([]);
  const [locationDraft, setLocationDraft] =
    useState<SettingsSectionDraft | null>(null);
  const [reloadVersion, setReloadVersion] = useState(0);
  const unsaved =
    Object.keys(changes).length > 0 ||
    !!aiDraft ||
    !!locationDraft ||
    !!credentialsDraft;
  const [busy, setBusy] = useState(false);
  const saveHint = busy ? "" : !unsaved ? t("settingsSaveNoChanges") : "";
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<SettingsErrors>({});
  useEffect(() => {
    const first = Object.keys(fieldErrors)[0];
    if (!first) return;
    let observer: MutationObserver | undefined;
    const locate = () => {
      const field = Array.from(
        document.querySelectorAll<HTMLElement>("[data-setting-field]"),
      ).find((element) => element.dataset.settingField === first);
      if (!field) return false;
      field.scrollIntoView({ block: "center" });
      observer?.disconnect();
      return true;
    };
    // Object editors can open in response to this validation result. Wait for
    // that render before positioning the error, without polling the page.
    if (!locate()) {
      observer = new MutationObserver(locate);
      observer.observe(document.body, { childList: true, subtree: true });
    }
    return () => observer?.disconnect();
  }, [fieldErrors]);
  const [dismissedQueryError, setDismissedQueryError] = useState<unknown>();
  const clearFieldError = (key: string) =>
    setFieldErrors((old) => {
      if (!old[key]) return old;
      const next = { ...old };
      delete next[key];
      return next;
    });
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
  async function run(
    command: (signal: AbortSignal) => Promise<void>,
    invalidMessage?: string,
  ) {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError("");
    setFieldErrors({});
    setNotice("");
    try {
      await command(controller.signal);
    } catch (err) {
      if (!controller.signal.aborted) {
        if (err instanceof APIError && err.body.fields)
          setFieldErrors(
            Object.fromEntries(
              err.body.fields.map((field) => [field.field, field]),
            ),
          );
        setError(
          err instanceof APIError && err.status === 409
            ? t("settingsConflict")
            : err instanceof APIError &&
                (err.status === 400 || err.status === 422) &&
                invalidMessage
              ? invalidMessage
              : errorMessage(err, t),
        );
      }
    } finally {
      active.current = null;
      setBusy(false);
    }
  }
  const reload = async (signal?: AbortSignal, preserveSecrets = false) => {
    const value = await loadSettingsPage(queries, signal, true);
    setSnapshot(value);
    setFieldErrors({});
    setChanges({});
    setAIDraft(null);
    setLocationDraft(null);
    setCredentialsDraft(null);
    if (!preserveSecrets) setCreatedCredentials([]);
    setReloadVersion((old) => old + 1);
    queries.setQueryData(["instance-settings"], value);
    await queries.invalidateQueries({ queryKey: ["info"] });
  };
  const save = async (skipDocumentReload = false) => {
    let saved = false;
    await run(async (signal) => {
      if (!snapshot) return;
      const response = await request<
        components["schemas"]["SettingsSavedResponse"]
      >(
        "/settings",
        "PUT",
        {
          revision: snapshot.revision,
          credentials: credentialsDraft?.value,
          ai_services: aiDraft?.value,
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
      const { created_credentials, ...value } = response;
      setCreatedCredentials((old) => [
        ...old.filter(
          (item) =>
            !credentialsDraft ||
            (
              credentialsDraft.value as components["schemas"]["CredentialSettings"]
            ).items?.some((value) => value.id === item.id),
        ),
        ...(created_credentials || []),
      ]);
      queries.setQueryData(["instance-settings"], value);
      await reload(signal, true);
      await queries.invalidateQueries({ queryKey: ["client"] });
      if (aiDraft)
        await queries.invalidateQueries({ queryKey: ["ai-account-models"] });
      if (created_credentials?.length) {
        // Newly issued secrets must remain visible before any navigation or
        // full reload, including the "save and leave" path and map CSP changes.
        pendingNavigation.current = null;
        setLeaving(false);
        setNotice(t("settingsSavedCopyCredentials"));
        document
          .getElementById("settings-access")
          ?.scrollIntoView({ block: "start" });
        return;
      }
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
    }, t("settingsNotSaved"));
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
          {settingRows(
            (snapshot.fields || []).filter(
              (field) => category(field) === group,
            ),
          ).map((row) => (
            <SettingsFieldRow key={row[0].field.key}>
              {row.map(({ field, wide }) => {
                const changed = Object.hasOwn(draftChanges, field.key);
                const value = changed
                  ? draftChanges[field.key] === null
                    ? field.default
                    : draftChanges[field.key]
                  : field.value;
                const overridden = (snapshot.overrides || []).includes(
                  field.key,
                );
                return (
                  <div
                    key={field.key}
                    className={
                      wide
                        ? "setting-field settings-field-wide"
                        : "setting-field"
                    }
                    data-setting-field={field.key}
                  >
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
                          onReset={() => {
                            clearFieldError(field.key);
                            setChanges((old) => ({
                              ...old,
                              [field.key]: null,
                            }));
                          }}
                        />
                      }
                      change={(value) => {
                        clearFieldError(field.key);
                        setChanges((old) => ({
                          ...old,
                          [field.key]: value,
                        }));
                      }}
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
            </SettingsFieldRow>
          ))}
        </Stack>
      </section>
    );
  }
  return (
    <SettingsErrorsContext.Provider
      value={{ errors: fieldErrors, clear: clearFieldError }}
    >
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
                  disabled={!unsaved}
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
          {(error || (query.error && query.error !== dismissedQueryError)) && (
            <FloatingFeedback
              message={error || errorMessage(query.error, t)}
              onClose={() => {
                setError("");
                setDismissedQueryError(query.error);
              }}
            />
          )}
          {notice && !unsaved && (
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
                        ...(services.data
                          ? [
                              {
                                id: "settings-ai-proxies",
                                label: t("aiProxies"),
                                subsection: true,
                              },
                              {
                                id: "settings-ai-accounts",
                                label: t("aiAccounts"),
                                subsection: true,
                              },
                              ...(services.data.definitions || []).map(
                                ({ name }) => ({
                                  id: `settings-ai-${name}`,
                                  label: t(`${name}Title` as MessageKey),
                                  subsection: true,
                                }),
                              ),
                            ]
                          : []),
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
                  <Credentials
                    busy={busy}
                    onDraftChange={setCredentialsDraft}
                    reloadVersion={reloadVersion}
                    created={createdCredentials}
                  />
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
              <div className="actions">
                <Button
                  data-autofocus
                  disabled={busy}
                  onClick={continueEditing}
                >
                  {t("settingsContinueEditing")}
                </Button>
                <Button disabled={busy} onClick={() => leave()}>
                  {t("settingsDiscardLeave")}
                </Button>
                <Button
                  variant="filled"
                  loading={busy}
                  disabled={busy}
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
    </SettingsErrorsContext.Provider>
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
  const error = useSettingsError(field.key);
  if (field.unlimited)
    return (
      <CacheLimitInput
        id={id}
        label={label}
        disabled={disabled}
        error={error}
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
        error={error}
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
        error={error}
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
        error={error}
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
        error={error}
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
        error={error}
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
      error={error}
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
        <SettingsFieldRow>
          <div className="setting-field">
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
        </SettingsFieldRow>
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
