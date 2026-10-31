import { settingsQueries } from "./settingsQueries";
import { HelpTooltip } from "./HelpTooltip";
import {
  SettingsCheckbox,
  SettingsFieldHeading,
  SettingsFieldRow,
  SettingsReset,
  useSettingsPending,
  settingsDraftValue,
  SettingsErrorsContext,
  useSettingsError,
  type SettingsSectionDraft,
} from "./SettingsFields";
import {
  Alert,
  Button,
  Group,
  NumberInput,
  Text,
  TextInput,
} from "@mantine/core";
import { useContext, useEffect, useMemo, useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { result } from "./api";
import type { components } from "./api-schema";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Settings = components["schemas"]["LocationSettings"];
type Draft = Omit<Settings, "request_interval_seconds"> & {
  request_interval_seconds: number | string;
};
export function LocationSettings({
  disabled = false,
  onPendingChange,
  onDraftChange,
  reloadVersion = 0,
}: {
  disabled?: boolean;
  onPendingChange?: (pending: boolean) => void;
  onDraftChange?: (draft: SettingsSectionDraft | null) => void;
  reloadVersion?: number;
}) {
  const { t } = useI18n();
  const { clear: clearFieldError } = useContext(SettingsErrorsContext);
  const endpointError = useSettingsError("geocoding.endpoint");
  const intervalError = useSettingsError("geocoding.request_interval_seconds");
  const endpointID = useId();
  const intervalID = useId();
  const queries = useQueryClient();
  const query = useQuery({
    ...settingsQueries.location,
  });
  const [draft, setDraft] = useState<Draft>();
  const [touched, setTouched] = useState(false);
  const pending = useMemo(
    () => (draft ? settingsDraftValue(draft, query.data) : undefined),
    [draft, query.data],
  );
  const dirty = !!(pending?.changed.length || pending?.reset?.length);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useSettingsPending(disabled || busy || dirty, onPendingChange);
  useEffect(() => {
    if (!touched && query.data) setDraft(query.data);
  }, [query.data, touched]);
  function edit(change: Partial<Draft>) {
    Object.keys(change).forEach((key) => clearFieldError(`geocoding.${key}`));
    setDraft((old) =>
      old
        ? {
            ...old,
            ...change,
            changed: [
              ...new Set([...(old.changed || []), ...Object.keys(change)]),
            ],
            reset: (old.reset || []).filter(
              (key) => !Object.hasOwn(change, key),
            ),
          }
        : old,
    );
    setTouched(true);
  }
  function resetAction(key: keyof NonNullable<Settings["defaults"]>) {
    const value = query.data?.defaults?.[key];
    const specified =
      !!draft &&
      (draft.changed?.includes(key) ||
        (!!query.data?.explicit?.includes(key) && !draft.reset?.includes(key)));
    return (
      <SettingsReset
        specified={!!specified}
        disabled={disabled || busy || value === undefined || !specified}
        onReset={() => {
          if (value === undefined) return;
          clearFieldError(`geocoding.${key}`);
          setDraft((old) =>
            old
              ? {
                  ...old,
                  [key]: value,
                  changed: (old.changed || []).filter((field) => field !== key),
                  reset: [...new Set([...(old.reset || []), key])],
                }
              : old,
          );
          setTouched(true);
        }}
      />
    );
  }
  useEffect(() => {
    setTouched(false);
    setError("");
    setDraft(queries.getQueryData(["location-settings"]));
  }, [reloadVersion, queries]);
  useEffect(() => {
    onDraftChange?.(dirty && pending ? { value: pending } : null);
  }, [dirty, pending, onDraftChange]);
  async function save() {
    if (!draft) return;
    setBusy(true);
    setError("");
    try {
      const value = await result<Settings>(
        await fetch("/api/v1/geo/settings", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(pending),
        }),
      );
      queries.setQueryData(["location-settings"], value);
      setDraft(value);
      setTouched(false);
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section
      id="settings-location"
      className="flat-page-section"
      aria-labelledby="settings-location-title"
    >
      <h2
        id="settings-location-title"
        aria-label={t("geoTitle")}
        className="settings-section-heading"
      >
        {t("geoTitle")}
        <HelpTooltip label={t("geoTitle")}>{t("geoSettingsHint")}</HelpTooltip>
      </h2>
      <div className="settings-blocks">
        {(error || query.error) && (
          <Alert role="alert" color="red">
            {error || (query.error && errorMessage(query.error, t))}
          </Alert>
        )}
        {query.isPending && <Text role="status">{t("mapLoading")}</Text>}
        {draft && (
          <div className="settings-fields">
            <SettingsCheckbox
              label={t("geoAuto")}
              action={resetAction("automatic")}
              help={t("geoAutoHint")}
              checked={draft.automatic}
              disabled={disabled || busy}
              onChange={(e) => edit({ automatic: e.currentTarget.checked })}
            />
            <div
              className="setting-field"
              data-setting-field="geocoding.endpoint"
            >
              <SettingsFieldHeading
                htmlFor={endpointID}
                label={t("geoEndpoint")}
                action={resetAction("endpoint")}
                help={t("geoEndpointHint")}
              />
              <TextInput
                id={endpointID}
                value={draft.endpoint}
                error={endpointError}
                disabled={disabled || busy}
                onChange={(e) => edit({ endpoint: e.currentTarget.value })}
              />
            </div>
            <SettingsFieldRow>
              <div
                className="setting-field"
                data-setting-field="geocoding.request_interval_seconds"
              >
                <SettingsFieldHeading
                  htmlFor={intervalID}
                  label={t("geoRequestInterval")}
                  action={resetAction("request_interval_seconds")}
                  help={t("geoRequestIntervalHint")}
                />
                <NumberInput
                  id={intervalID}
                  error={intervalError}
                  allowDecimal={false}
                  value={draft.request_interval_seconds}
                  disabled={disabled || busy}
                  onChange={(value) =>
                    edit({ request_interval_seconds: value })
                  }
                />
              </div>
            </SettingsFieldRow>
            <SettingsCheckbox
              label={t("backgroundPauseOnError")}
              action={resetAction("pause_on_error")}
              help={t("backgroundPauseOnErrorHint")}
              checked={draft.pause_on_error}
              disabled={disabled || busy}
              onChange={(e) =>
                edit({ pause_on_error: e.currentTarget.checked })
              }
            />
            {!onDraftChange && (
              <Group>
                <Button
                  disabled={!dirty || disabled || busy}
                  loading={busy}
                  onClick={() => void save()}
                >
                  {t("geoSaveSettings")}
                </Button>
                <Button
                  disabled={busy || disabled}
                  onClick={() => {
                    setTouched(false);
                    setError("");
                    void query.refetch();
                  }}
                >
                  {t("geoReloadSettings")}
                </Button>
              </Group>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
