import { settingsQueries } from "./settingsQueries";
import { HelpTooltip } from "./HelpTooltip";
import {
  SettingsCheckbox,
  SettingsFieldHeading,
  SettingsReset,
  useSettingsPending,
} from "./SettingsFields";
import {
  Alert,
  Button,
  Group,
  NumberInput,
  Text,
  TextInput,
} from "@mantine/core";
import { useEffect, useId, useState } from "react";
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
}: {
  disabled?: boolean;
  onPendingChange?: (pending: boolean) => void;
}) {
  const { t } = useI18n();
  const endpointID = useId();
  const intervalID = useId();
  const queries = useQueryClient();
  const query = useQuery({
    ...settingsQueries.location,
  });
  const [draft, setDraft] = useState<Draft>();
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useSettingsPending(disabled || busy || dirty, onPendingChange);
  useEffect(() => {
    if (!dirty && query.data) setDraft(query.data);
  }, [query.data, dirty]);
  function edit(change: Partial<Draft>) {
    setDraft((old) =>
      old
        ? {
            ...old,
            ...change,
            reset: (old.reset || []).filter(
              (key) => !Object.hasOwn(change, key),
            ),
          }
        : old,
    );
    setDirty(true);
  }
  function resetAction(key: keyof NonNullable<Settings["defaults"]>) {
    const value = query.data?.defaults?.[key];
    return (
      <SettingsReset
        disabled={
          disabled || busy || value === undefined || draft?.[key] === value
        }
        onReset={() => {
          if (value === undefined) return;
          setDraft((old) =>
            old
              ? {
                  ...old,
                  [key]: value,
                  reset: [...new Set([...(old.reset || []), key])],
                }
              : old,
          );
          setDirty(true);
        }}
      />
    );
  }
  const validInterval =
    typeof draft?.request_interval_seconds === "number" &&
    Number.isInteger(draft.request_interval_seconds) &&
    draft.request_interval_seconds >= 0 &&
    draft.request_interval_seconds <= 86400;
  async function save() {
    if (!draft || !validInterval) return;
    setBusy(true);
    setError("");
    try {
      const value = await result<Settings>(
        await fetch("/api/v1/geo/settings", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            ...draft,
            defaults: undefined,
            reset: draft.reset?.length ? draft.reset : undefined,
          }),
        }),
      );
      queries.setQueryData(["location-settings"], value);
      setDraft(value);
      setDirty(false);
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
            <div className="setting-field">
              <SettingsFieldHeading
                htmlFor={endpointID}
                label={t("geoEndpoint")}
                action={resetAction("endpoint")}
                help={t("geoEndpointHint")}
              />
              <TextInput
                id={endpointID}
                value={draft.endpoint}
                disabled={disabled || busy}
                onChange={(e) => edit({ endpoint: e.currentTarget.value })}
              />
            </div>
            <div className="setting-field">
              <SettingsFieldHeading
                htmlFor={intervalID}
                label={t("geoRequestInterval")}
                action={resetAction("request_interval_seconds")}
                help={t("geoRequestIntervalHint")}
              />
              <NumberInput
                id={intervalID}
                min={0}
                max={86400}
                allowDecimal={false}
                value={draft.request_interval_seconds}
                disabled={disabled || busy}
                onChange={(value) => edit({ request_interval_seconds: value })}
              />
            </div>
            <SettingsCheckbox
              label={t("geoPauseOnError")}
              action={resetAction("pause_on_error")}
              help={t("geoPauseOnErrorHint")}
              checked={draft.pause_on_error}
              disabled={disabled || busy}
              onChange={(e) =>
                edit({ pause_on_error: e.currentTarget.checked })
              }
            />
            <Group>
              <Button
                disabled={!dirty || !validInterval || disabled || busy}
                loading={busy}
                onClick={() => void save()}
              >
                {t("geoSaveSettings")}
              </Button>
              <Button
                disabled={busy || disabled}
                onClick={() => {
                  setDirty(false);
                  setError("");
                  void query.refetch();
                }}
              >
                {t("geoReloadSettings")}
              </Button>
            </Group>
          </div>
        )}
      </div>
    </section>
  );
}
