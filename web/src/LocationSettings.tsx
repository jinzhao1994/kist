import {
  Alert,
  Button,
  Checkbox,
  Group,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, result } from "./api";
import type { components } from "./api-schema";
import { BackgroundTaskQueue } from "./AI";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Settings = components["schemas"]["LocationSettings"];
export function LocationSettings({ disabled = false }: { disabled?: boolean }) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const query = useQuery({
    queryKey: ["location-settings"],
    queryFn: ({ signal }) => get<Settings>("/geo/settings", signal),
    retry: false,
  });
  const [draft, setDraft] = useState<Settings>();
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!dirty && query.data) setDraft(query.data);
  }, [query.data, dirty]);
  async function save() {
    if (!draft) return;
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
      <h2 id="settings-location-title">{t("geoTitle")}</h2>
      <Stack>
        <Text size="sm" c="dimmed">
          {t("geoSettingsHint")}
        </Text>
        {(error || query.error) && (
          <Alert role="alert" color="red">
            {error || (query.error && errorMessage(query.error, t))}
          </Alert>
        )}
        {query.isPending && <Text role="status">{t("mapLoading")}</Text>}
        {draft && (
          <>
            <Checkbox
              label={t("geoAuto")}
              description={t("geoAutoHint")}
              checked={draft.automatic}
              disabled={disabled || busy}
              onChange={(e) => {
                setDraft({ ...draft, automatic: e.currentTarget.checked });
                setDirty(true);
              }}
            />
            <TextInput
              label={t("geoEndpoint")}
              description={t("geoEndpointHint")}
              value={draft.endpoint}
              disabled={disabled || busy}
              onChange={(e) => {
                setDraft({ ...draft, endpoint: e.currentTarget.value });
                setDirty(true);
              }}
            />
          </>
        )}
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
              setDirty(false);
              setError("");
              void query.refetch();
            }}
          >
            {t("geoReloadSettings")}
          </Button>
        </Group>
        <BackgroundTaskQueue
          kind="geo"
          disabled={disabled || busy || dirty}
          connected={!!query.data}
          model="photon"
        />
      </Stack>
    </section>
  );
}
