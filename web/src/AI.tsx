import {
  Alert,
  Button,
  CopyButton,
  Group,
  Stack,
  Text,
  Textarea,
} from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, result, createWrite } from "./api";
import { Dialog } from "./Dialog";
import { Select } from "./Select";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

type Status = {
  supported: boolean;
  connected: boolean;
  email: string;
  pending: boolean;
  error: string;
};
type Model = { id: string; name: string };
async function post<T>(path: string): Promise<T> {
  return result<T>(
    await fetch(`/api/v1${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }),
  );
}
function useStatus() {
  return useQuery({
    queryKey: ["ai-connection"],
    queryFn: ({ signal }) => get<Status>("/ai", signal),
    retry: false,
    refetchInterval: (query) => (query.state.data?.pending ? 1000 : false),
  });
}
export function AIConnection({
  disabled = false,
  onChanged,
}: { disabled?: boolean; onChanged?: () => Promise<void> } = {}) {
  const { t } = useI18n();
  const query = useStatus();
  const queries = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [url, setURL] = useState("");
  const wasPending = useRef(false);
  useEffect(() => {
    const pending = query.data?.pending;
    if (wasPending.current && pending === false) void onChanged?.();
    wasPending.current = !!pending;
  }, [query.data?.pending, onChanged]);
  async function connect() {
    // Open synchronously within the click gesture so popup blockers allow it.
    const login = window.open("about:blank", "_blank");
    if (login) login.opener = null;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const out = await post<{ url: string }>("/ai/connect");
      setURL(out.url);
      if (login && !login.closed) login.location.replace(out.url);
    } catch (e) {
      login?.close();
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["ai-connection"] });
      await onChanged?.();
    }
  }
  async function disconnect() {
    setBusy(true);
    setError("");
    setURL("");
    try {
      const out = await post<{ revoked: boolean }>("/ai/disconnect");
      if (!out.revoked) setNotice(t("aiRevocationUnconfirmed"));
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
      await queries.invalidateQueries({ queryKey: ["ai-connection"] });
      await onChanged?.();
      await queries.invalidateQueries({ queryKey: ["ai-models"] });
    }
  }
  const status = query.data;
  return (
    <section
      id="settings-ai"
      className="flat-page-section"
      aria-labelledby="settings-ai-title"
    >
      <h2 id="settings-ai-title">{t("aiConnection")}</h2>
      <Stack>
        <Text size="sm" c="dimmed">
          {t("aiLocalHint")}
        </Text>
        {(error || query.error || status?.error) && (
          <Alert role="alert" color="red">
            {error ||
              (query.error ? errorMessage(query.error, t) : status?.error)}
          </Alert>
        )}
        {notice && <Text role="status">{notice}</Text>}
        {disabled && (
          <Text size="sm" c="dimmed">
            {t("settingsPending")}
          </Text>
        )}
        {status?.connected && (
          <Text>
            {t("aiConnected")}: {status.email}
          </Text>
        )}
        <Group>
          <Button
            disabled={disabled || busy || !status?.supported || status.pending}
            onClick={connect}
          >
            {t("aiContinue")}
          </Button>
          {(status?.connected || status?.pending) && (
            <Button disabled={disabled || busy} onClick={disconnect}>
              {t(status.pending ? "cancel" : "aiDisconnect")}
            </Button>
          )}
          {url && status?.pending && (
            <Button component="a" href={url} target="_blank" rel="noreferrer">
              {t("aiOpenSignIn")}
            </Button>
          )}
        </Group>
        {status?.pending && (
          <Text role="status" size="sm">
            {t("aiWaiting")}
          </Text>
        )}
      </Stack>
    </section>
  );
}
export function OCRDialog({
  fileID,
  onClose,
}: {
  fileID: string;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const status = useStatus();
  const models = useQuery({
    queryKey: ["ai-models"],
    queryFn: ({ signal }) => get<Model[]>("/ai/models", signal),
    enabled: !!status.data?.connected,
    retry: false,
  });
  const [selected, setSelected] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [write, setWrite] = useState<ReturnType<typeof createWrite>>();
  const model =
    selected ??
    (models.data?.some((m) => m.id === "gpt-6-luna") ? "gpt-6-luna" : "");
  async function run() {
    setBusy(true);
    setError("");
    const command =
      write ?? createWrite("POST", `/files/${fileID}/ocr`, { model });
    setWrite(command);
    try {
      await command.run();
      await queries.invalidateQueries();
      onClose();
    } catch (e) {
      setError(errorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
      title={t("ocrExtract")}
    >
      <Stack>
        <Text size="sm">{t("ocrSendHint")}</Text>
        {(error || status.error || models.error) && (
          <Alert role="alert" color="red">
            {error ||
              (status.error
                ? errorMessage(status.error, t)
                : errorMessage(models.error, t))}
          </Alert>
        )}
        {status.data && !status.data.connected && (
          <Text>
            {t("aiConnectFirst")}{" "}
            <a href="/settings#settings-ai">{t("instanceSettings")}</a>
          </Text>
        )}
        <Select
          label={t("aiModel")}
          value={model}
          onValueChange={(value) => {
            if (!busy) {
              setSelected(value);
              setWrite(undefined);
            }
          }}
          options={(models.data || []).map((m) => ({
            value: m.id,
            label: m.name || m.id,
          }))}
        />
        <Group>
          <Button
            disabled={busy || !model || !status.data?.connected}
            onClick={run}
          >
            {t(busy ? "extracting" : "ocrExtract")}
          </Button>
          <Button disabled={busy} onClick={onClose}>
            {t("cancel")}
          </Button>
        </Group>
      </Stack>
    </Dialog>
  );
}
export function OCRText({ text }: { text: string }) {
  const { t } = useI18n();
  return (
    <Stack>
      <Textarea
        label={t("ocrText")}
        value={text}
        readOnly
        autosize
        minRows={3}
        maxRows={14}
      />
      <CopyButton value={text}>
        {({ copied, copy }) => (
          <Button onClick={copy}>{t(copied ? "copied" : "copy")}</Button>
        )}
      </CopyButton>
    </Stack>
  );
}
