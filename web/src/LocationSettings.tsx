import { settingsQueries } from "./settingsQueries";
import { HelpTooltip } from "./HelpTooltip";
import { SettingsCheckbox, SettingsFieldHeading } from "./SettingsFields";
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
import { BackgroundTaskQueue } from "./AI";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Settings = components["schemas"]["LocationSettings"];
type Draft = Omit<Settings, "request_interval_seconds"> & {
  request_interval_seconds: number | string;
};
export function LocationSettings({ disabled = false }: { disabled?: boolean }) {
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
  useEffect(() => {
    if (!dirty && query.data) setDraft(query.data);
  }, [query.data, dirty]);
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
          body: JSON.stringify(draft),
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
              help={t("geoAutoHint")}
              checked={draft.automatic}
              disabled={disabled || busy}
              onChange={(e) => {
                setDraft({ ...draft, automatic: e.currentTarget.checked });
                setDirty(true);
              }}
            />
            <div>
              <SettingsFieldHeading
                htmlFor={endpointID}
                label={t("geoEndpoint")}
                help={t("geoEndpointHint")}
              />
              <TextInput
                id={endpointID}
                value={draft.endpoint}
                disabled={disabled || busy}
                onChange={(e) => {
                  setDraft({ ...draft, endpoint: e.currentTarget.value });
                  setDirty(true);
                }}
              />
            </div>
            <div>
              <SettingsFieldHeading
                htmlFor={intervalID}
                label={t("geoRequestInterval")}
                help={t("geoRequestIntervalHint")}
              />
              <NumberInput
                id={intervalID}
                min={0}
                max={86400}
                allowDecimal={false}
                value={draft.request_interval_seconds}
                disabled={disabled || busy}
                onChange={(value) => {
                  setDraft({ ...draft, request_interval_seconds: value });
                  setDirty(true);
                }}
              />
            </div>
            <SettingsCheckbox
              label={t("geoPauseOnError")}
              help={t("geoPauseOnErrorHint")}
              checked={draft.pause_on_error}
              disabled={disabled || busy}
              onChange={(e) => {
                setDraft({ ...draft, pause_on_error: e.currentTarget.checked });
                setDirty(true);
              }}
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
        <BackgroundTaskQueue
          kind="geo"
          disabled={disabled || busy || dirty}
          connected={!!query.data}
          model="photon"
        />
      </div>
    </section>
  );
}
