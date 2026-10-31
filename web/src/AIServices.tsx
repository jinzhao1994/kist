import {
  Anchor,
  Autocomplete,
  Button,
  Group,
  Loader,
  NumberInput,
  PasswordInput,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { components } from "./api-schema";
import { get, result } from "./api";
import { EmptyHint } from "./EmptyHint";
import { HelpTooltip } from "./HelpTooltip";
import { Select } from "./Select";
import {
  SettingsCheckbox,
  SettingsErrorsContext,
  SettingsFieldHeading,
  SettingsHint,
  useSettingsError,
  useSettingsPending,
  type SettingsSectionDraft,
} from "./SettingsFields";
import { settingsQueries } from "./settingsQueries";
import { TaskError, TaskStatus } from "./TaskStatus";
import { errorMessage } from "./mutations";
import { useI18n, type MessageKey } from "./i18n";

type Schema = components["schemas"];
type View = Schema["AiServicesView"];
type Proxy = Schema["AIProxySettings"];
type Account = Schema["AIAccountSettings"];
type Feature = Schema["Feature"];
export type AIConnectionProps = {
  disabled?: boolean;
  configurationPending?: boolean;
  onChanged?: () => Promise<void>;
  onPendingChange?: (pending: boolean) => void;
  onDraftChange?: (draft: SettingsSectionDraft | null) => void;
  reloadVersion?: number;
};

export function aiServicesDraft(value: View) {
  return {
    revision: value.revision,
    proxies: (value.proxies || []).map(
      ({ secret_configured: _, ...proxy }) => ({
        ...proxy,
        password: proxy.password || undefined,
        clear_secret: proxy.clear_secret || undefined,
      }),
    ),
    accounts: (value.accounts || []).map(
      ({ connected: _, email: _email, ...account }) => ({
        ...account,
        api_key: account.api_key || undefined,
        clear_secret: account.clear_secret || undefined,
      }),
    ),
    features: value.features,
  };
}
const emptyFeature = (): Feature => ({
  account: 0,
  model: "",
  reasoning_effort: "",
  image_detail: "",
  automatic: false,
});
const changed = (a: unknown, b: unknown) =>
  JSON.stringify(a) !== JSON.stringify(b);
function proxyAddress(address: string) {
  try {
    const url = new URL(address);
    url.username = "";
    url.password = "";
    return url.toString();
  } catch {
    return "—";
  }
}
const objectNumber = (number: number) => (number > 0 ? String(number) : "—");

function ServiceField({
  path,
  label,
  help,
  children,
}: {
  path: string;
  label: string;
  help?: string;
  children: (id: string, error?: string) => ReactNode;
}) {
  const id = useId();
  const error = useSettingsError(path);
  return (
    <div className="setting-field" data-setting-field={path}>
      <SettingsFieldHeading label={label} help={help} htmlFor={id} />
      {children(id, error)}
    </div>
  );
}

export function AIConnection({
  disabled = false,
  configurationPending = false,
  onChanged,
  onPendingChange,
  onDraftChange,
  reloadVersion = 0,
}: AIConnectionProps = {}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const connection = useQuery(settingsQueries.connection);
  const query = useQuery({
    ...settingsQueries.services,
    enabled: !!connection.data?.supported,
    refetchInterval: (query) =>
      query.state.data?.pending_account ? 1000 : false,
  });
  const [draft, setDraft] = useState<View>();
  const [touched, setTouched] = useState(false);
  const [proxyEditor, setProxyEditor] = useState<number>();
  const [accountEditor, setAccountEditor] = useState<number>();
  const [busy, setBusy] = useState<number>();
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loginURL, setLoginURL] = useState("");
  const pendingAccount = useRef(0);
  const temporary = useRef(-1);
  const { errors, clear } = useContext(SettingsErrorsContext);
  const pending = useMemo(
    () => (draft ? aiServicesDraft(draft) : undefined),
    [draft],
  );
  const dirty =
    !!pending &&
    !!query.data &&
    changed(
      { ...pending, revision: "" },
      { ...aiServicesDraft(query.data), revision: "" },
    );
  const connectionsDirty =
    !!draft &&
    !!query.data &&
    (changed(pending?.accounts, aiServicesDraft(query.data).accounts) ||
      changed(pending?.proxies, aiServicesDraft(query.data).proxies));
  useSettingsPending(disabled || busy !== undefined || dirty, onPendingChange);
  useEffect(() => {
    if (query.data && !touched) setDraft(query.data);
  }, [query.data, touched]);
  useEffect(() => {
    setTouched(false);
    setDraft(queries.getQueryData(["ai-services"]));
    setError("");
    setNotice("");
  }, [reloadVersion, queries]);
  useEffect(() => {
    onDraftChange?.(dirty && pending ? { value: pending } : null);
  }, [dirty, pending, onDraftChange]);
  useEffect(() => {
    const before = pendingAccount.current;
    pendingAccount.current = query.data?.pending_account || 0;
    if (
      before &&
      !query.data?.pending_account &&
      !dirty &&
      !configurationPending
    ) {
      void onChanged?.();
      void queries.invalidateQueries({
        queryKey: ["ai-account-models", before],
      });
    }
  }, [
    query.data?.pending_account,
    onChanged,
    queries,
    dirty,
    configurationPending,
  ]);
  useEffect(() => {
    const key = Object.keys(errors).find((key) =>
      key.startsWith("ai_services."),
    );
    const match = key?.match(/^ai_services\.(proxies|accounts)\.(\d+)\./);
    if (match && draft) {
      const number =
        draft[match[1] as "proxies" | "accounts"]?.[Number(match[2])]?.number;
      if (match[1] === "proxies") setProxyEditor(number);
      else setAccountEditor(number);
    }
  }, [errors, draft]);
  function edit(next: View, path?: string) {
    if (path) clear(path);
    setTouched(true);
    setDraft(next);
    setNotice("");
  }
  function proxyChange(number: number, values: Partial<Proxy>) {
    if (!draft) return;
    const index = draft.proxies!.findIndex((p) => p.number === number);
    Object.keys(values).forEach((key) =>
      clear(`ai_services.proxies.${index}.${key}`),
    );
    edit({
      ...draft,
      proxies: draft.proxies!.map((p) =>
        p.number === number ? { ...p, ...values } : p,
      ),
    });
  }
  function accountChange(number: number, values: Partial<Account>) {
    if (!draft) return;
    const index = draft.accounts!.findIndex((a) => a.number === number);
    Object.keys(values).forEach((key) =>
      clear(`ai_services.accounts.${index}.${key}`),
    );
    edit({
      ...draft,
      accounts: draft.accounts!.map((a) =>
        a.number === number ? { ...a, ...values } : a,
      ),
    });
  }
  async function accountAction(
    number: number,
    action: "connect" | "disconnect",
  ) {
    const login =
      action === "connect" ? window.open("about:blank", "_blank") : null;
    if (login) login.opener = null;
    setBusy(number);
    setError("");
    setNotice("");
    try {
      const value = await result<{ url?: string; revoked?: boolean }>(
        await fetch(`/api/v1/ai/accounts/${number}/${action}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
        }),
      );
      if (value.url) {
        setLoginURL(value.url);
        if (login && !login.closed) login.location.replace(value.url);
      }
      if (value.revoked === false) setNotice(t("aiRevocationUnconfirmed"));
    } catch (error) {
      login?.close();
      setError(errorMessage(error, t));
    } finally {
      await queries.invalidateQueries({ queryKey: ["ai-services"] });
      await onChanged?.();
      setBusy(undefined);
    }
  }
  const locked = disabled || busy !== undefined;
  const proxy = draft?.proxies?.find((p) => p.number === proxyEditor);
  const proxyIndex =
    draft?.proxies?.findIndex((p) => p.number === proxyEditor) ?? -1;
  const account = draft?.accounts?.find((a) => a.number === accountEditor);
  const accountIndex =
    draft?.accounts?.findIndex((a) => a.number === accountEditor) ?? -1;
  const proxyOptions = [
    { value: "0", label: t("aiDirect") },
    ...(draft?.proxies || []).map((p) => ({
      value: String(p.number),
      label: p.id || t("aiUnnamed"),
    })),
  ];
  if (
    account?.proxy &&
    !proxyOptions.some((p) => p.value === String(account.proxy))
  )
    proxyOptions.push({
      value: String(account.proxy),
      label: t("aiRemovedReference", { number: account.proxy }),
    });
  const rowAction = (label: string, action: () => void, blocked = locked) => (
    <Anchor
      component="button"
      type="button"
      className="text-action-link"
      size="xs"
      disabled={blocked}
      onClick={action}
    >
      {label}
    </Anchor>
  );
  const connectionBlocked =
    disabled || configurationPending || dirty || busy !== undefined;
  return (
    <section
      id="settings-ai"
      className="flat-page-section"
      aria-labelledby="settings-ai-title"
    >
      <h2
        id="settings-ai-title"
        className="settings-section-heading"
        aria-label={t("aiConnection")}
      >
        {t("aiConnection")}
        <HelpTooltip label={t("aiConnection")}>
          {t("aiServicesHint")}
        </HelpTooltip>
      </h2>
      {(connection.isPending ||
        (query.isPending && connection.data?.supported)) && (
        <TaskStatus>{t("loading")}</TaskStatus>
      )}
      {(error || connection.error || query.error || query.data?.error) && (
        <TaskError>
          {error ||
            (connection.error && errorMessage(connection.error, t)) ||
            (query.error ? errorMessage(query.error, t) : query.data?.error)}
        </TaskError>
      )}
      {(connection.error || query.error) && (
        <Anchor
          component="button"
          type="button"
          size="xs"
          className="text-action-link"
          onClick={() =>
            void (connection.error ? connection.refetch() : query.refetch())
          }
        >
          {t("retry")}
        </Anchor>
      )}
      {notice && <TaskStatus>{notice}</TaskStatus>}
      {draft && (
        <div className="settings-blocks">
          <section
            id="settings-ai-proxies"
            className="settings-fields settings-subsection"
            aria-labelledby="ai-proxies-title"
          >
            <div className="settings-section-heading">
              <h3 id="ai-proxies-title" className="settings-subsection-heading">
                {t("aiProxies")}
              </h3>
              <HelpTooltip label={t("aiProxies")}>
                {t("aiProxiesHint")}
              </HelpTooltip>
            </div>
            <Group justify="space-between">
              <EmptyHint>
                {draft.proxies?.length
                  ? t("aiObjectCount", { count: draft.proxies.length })
                  : t("aiNoProxies")}
              </EmptyHint>
              <Button
                disabled={locked}
                onClick={() => {
                  const number = temporary.current--;
                  edit({
                    ...draft,
                    proxies: [
                      ...(draft.proxies || []),
                      {
                        number,
                        id: "",
                        type: "http",
                        url: "",
                        host: "",
                        port: 0,
                        server_name: "",
                        secret_configured: false,
                      },
                    ],
                  });
                  setProxyEditor(number);
                }}
              >
                {t("aiAddProxy")}
              </Button>
            </Group>
            {!!draft.proxies?.length && (
              <Table.ScrollContainer minWidth={480}>
                <Table
                  highlightOnHover
                  className="settings-table settings-table--compact"
                  aria-label={t("aiProxies")}
                >
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>{t("aiNumber")}</Table.Th>
                      <Table.Th>ID</Table.Th>
                      <Table.Th>{t("type")}</Table.Th>
                      <Table.Th>{t("aiEndpoint")}</Table.Th>
                      <Table.Th>
                        <span className="sr-only">
                          {t("settingCredentialActions")}
                        </span>
                      </Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {draft.proxies.map((p) => (
                      <Table.Tr key={p.number}>
                        <Table.Td>{objectNumber(p.number)}</Table.Td>
                        <Table.Th scope="row">
                          {p.id || t("aiUnnamed")}
                        </Table.Th>
                        <Table.Td>
                          {p.type === "http" ? "HTTP" : "Trojan"}
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" truncate>
                            {p.type === "http"
                              ? proxyAddress(p.url)
                              : `${p.host}:${p.port}`}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          <Group
                            gap="md"
                            justify="flex-end"
                            wrap="nowrap"
                            className="table-row-actions"
                          >
                            {rowAction(
                              t(
                                p.number === proxyEditor
                                  ? "aiCollapse"
                                  : "aiEdit",
                              ),
                              () =>
                                setProxyEditor(
                                  p.number === proxyEditor
                                    ? undefined
                                    : p.number,
                                ),
                            )}
                            {rowAction(t("aiDelete"), () => {
                              edit({
                                ...draft,
                                proxies: draft.proxies!.filter(
                                  (value) => value.number !== p.number,
                                ),
                              });
                              if (proxyEditor === p.number)
                                setProxyEditor(undefined);
                            })}
                          </Group>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>
            )}
            {proxy && (
              <div
                className="settings-fields"
                role="region"
                aria-label={t("aiProxyEditor")}
              >
                <ServiceField
                  path={`ai_services.proxies.${proxyIndex}.id`}
                  label={t("aiObjectID")}
                >
                  {(id, error) => (
                    <TextInput
                      id={id}
                      value={proxy.id}
                      error={error}
                      disabled={locked}
                      onChange={(e) =>
                        proxyChange(proxy.number, { id: e.currentTarget.value })
                      }
                    />
                  )}
                </ServiceField>
                <ServiceField
                  path={`ai_services.proxies.${proxyIndex}.type`}
                  label={t("type")}
                >
                  {(id, error) => (
                    <Select
                      id={id}
                      value={proxy.type}
                      error={error}
                      disabled={locked}
                      options={[
                        { value: "http", label: "HTTP" },
                        { value: "trojan", label: "Trojan" },
                      ]}
                      onValueChange={(type) => {
                        const saved = query.data?.proxies?.find(
                          (value) =>
                            value.number === proxy.number &&
                            value.type === type,
                        );
                        proxyChange(proxy.number, {
                          type: type as Proxy["type"],
                          url: "",
                          host: "",
                          port: type === "http" ? 0 : 443,
                          password: "",
                          server_name: "",
                          secret_configured: false,
                          ...saved,
                          id: proxy.id,
                          clear_secret: undefined,
                        });
                      }}
                    />
                  )}
                </ServiceField>
                {proxy.type === "http" ? (
                  <>
                    <ServiceField
                      path={`ai_services.proxies.${proxyIndex}.url`}
                      label={t("aiProxy")}
                      help={t("aiProxySecretHint")}
                    >
                      {(id, error) => (
                        <PasswordInput
                          id={id}
                          value={proxy.url}
                          error={error}
                          disabled={locked}
                          autoComplete="off"
                          onChange={(e) =>
                            proxyChange(proxy.number, {
                              url: e.currentTarget.value,
                            })
                          }
                        />
                      )}
                    </ServiceField>
                    {proxy.secret_configured && (
                      <SettingsCheckbox
                        label={t("aiClearProxyAuth")}
                        help={t("aiClearProxyAuthHint")}
                        checked={!!proxy.clear_secret}
                        disabled={locked}
                        onChange={(event) =>
                          proxyChange(proxy.number, {
                            clear_secret: event.currentTarget.checked,
                          })
                        }
                      />
                    )}
                  </>
                ) : (
                  <>
                    <ServiceField
                      path={`ai_services.proxies.${proxyIndex}.host`}
                      label={t("aiProxyHost")}
                    >
                      {(id, error) => (
                        <TextInput
                          id={id}
                          value={proxy.host}
                          error={error}
                          disabled={locked}
                          onChange={(e) =>
                            proxyChange(proxy.number, {
                              host: e.currentTarget.value,
                            })
                          }
                        />
                      )}
                    </ServiceField>
                    <ServiceField
                      path={`ai_services.proxies.${proxyIndex}.port`}
                      label={t("aiProxyPort")}
                    >
                      {(id, error) => (
                        <NumberInput
                          id={id}
                          value={proxy.port}
                          error={error}
                          disabled={locked}
                          onChange={(value) =>
                            proxyChange(proxy.number, { port: Number(value) })
                          }
                        />
                      )}
                    </ServiceField>
                    <ServiceField
                      path={`ai_services.proxies.${proxyIndex}.server_name`}
                      label={t("aiTLSServerName")}
                      help={t("aiTLSServerNameHint")}
                    >
                      {(id, error) => (
                        <TextInput
                          id={id}
                          value={proxy.server_name}
                          error={error}
                          disabled={locked}
                          onChange={(e) =>
                            proxyChange(proxy.number, {
                              server_name: e.currentTarget.value,
                            })
                          }
                        />
                      )}
                    </ServiceField>
                    <ServiceField
                      path={`ai_services.proxies.${proxyIndex}.password`}
                      label={t("aiProxyPassword")}
                      help={
                        proxy.secret_configured
                          ? t("aiSecretKeepHint")
                          : undefined
                      }
                    >
                      {(id, error) => (
                        <PasswordInput
                          id={id}
                          value={proxy.password || ""}
                          error={error}
                          disabled={locked}
                          autoComplete="new-password"
                          placeholder={
                            proxy.secret_configured
                              ? t("aiSecretConfigured")
                              : undefined
                          }
                          onChange={(e) =>
                            proxyChange(proxy.number, {
                              password: e.currentTarget.value,
                            })
                          }
                        />
                      )}
                    </ServiceField>
                  </>
                )}
              </div>
            )}
          </section>
          <section
            id="settings-ai-accounts"
            className="settings-fields settings-subsection"
            aria-labelledby="ai-accounts-title"
          >
            <div className="settings-section-heading">
              <h3
                id="ai-accounts-title"
                className="settings-subsection-heading"
              >
                {t("aiAccounts")}
              </h3>
              <HelpTooltip label={t("aiAccounts")}>
                {t("aiAccountsHint")}
              </HelpTooltip>
            </div>
            <Group justify="space-between">
              <EmptyHint>
                {draft.accounts?.length
                  ? t("aiObjectCount", { count: draft.accounts.length })
                  : t("aiNoAccounts")}
              </EmptyHint>
              <Button
                disabled={locked}
                onClick={() => {
                  const number = temporary.current--;
                  edit({
                    ...draft,
                    accounts: [
                      ...(draft.accounts || []),
                      {
                        number,
                        id: "",
                        type: "chatgpt",
                        proxy: 0,
                        connected: false,
                        email: "",
                      },
                    ],
                  });
                  setAccountEditor(number);
                }}
              >
                {t("aiAddAccount")}
              </Button>
            </Group>
            {!!draft.accounts?.length && (
              <Table.ScrollContainer minWidth={560}>
                <Table
                  highlightOnHover
                  className="settings-table settings-table--compact"
                  aria-label={t("aiAccounts")}
                >
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>{t("aiNumber")}</Table.Th>
                      <Table.Th>ID</Table.Th>
                      <Table.Th>{t("type")}</Table.Th>
                      <Table.Th>{t("aiAccountProxy")}</Table.Th>
                      <Table.Th>{t("backgroundTaskState")}</Table.Th>
                      <Table.Th>
                        <span className="sr-only">
                          {t("settingCredentialActions")}
                        </span>
                      </Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {draft.accounts.map((a) => (
                      <Table.Tr key={a.number}>
                        <Table.Td>{objectNumber(a.number)}</Table.Td>
                        <Table.Th scope="row">
                          {a.id || t("aiUnnamed")}
                        </Table.Th>
                        <Table.Td>
                          {a.type === "chatgpt" ? "ChatGPT" : "OpenAI API"}
                        </Table.Td>
                        <Table.Td>
                          {a.proxy
                            ? draft.proxies?.find((p) => p.number === a.proxy)
                                ?.id ||
                              t("aiRemovedReference", { number: a.proxy })
                            : t("aiDirect")}
                        </Table.Td>
                        <Table.Td>
                          {query.data?.pending_account === a.number
                            ? t("aiWaiting")
                            : a.connected
                              ? t("aiConnected")
                              : t("aiDisconnected")}
                        </Table.Td>
                        <Table.Td>
                          <Group
                            gap="md"
                            justify="flex-end"
                            wrap="nowrap"
                            className="table-row-actions"
                            data-busy={busy === a.number || undefined}
                          >
                            {a.type === "chatgpt" &&
                              rowAction(
                                t("aiContinue"),
                                () => void accountAction(a.number, "connect"),
                                connectionBlocked ||
                                  a.number <= 0 ||
                                  !!query.data?.pending_account,
                              )}
                            {(a.connected ||
                              query.data?.pending_account === a.number) &&
                              rowAction(
                                t(
                                  query.data?.pending_account === a.number
                                    ? "cancel"
                                    : "aiDisconnect",
                                ),
                                () =>
                                  void accountAction(a.number, "disconnect"),
                                connectionBlocked || a.number <= 0,
                              )}
                            {rowAction(
                              t(
                                a.number === accountEditor
                                  ? "aiCollapse"
                                  : "aiEdit",
                              ),
                              () =>
                                setAccountEditor(
                                  a.number === accountEditor
                                    ? undefined
                                    : a.number,
                                ),
                            )}
                            {rowAction(t("aiDelete"), () => {
                              edit({
                                ...draft,
                                accounts: draft.accounts!.filter(
                                  (value) => value.number !== a.number,
                                ),
                              });
                              if (accountEditor === a.number)
                                setAccountEditor(undefined);
                            })}
                          </Group>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              </Table.ScrollContainer>
            )}
            {loginURL && !!query.data?.pending_account && (
              <Anchor href={loginURL} target="_blank" rel="noreferrer">
                {t("aiOpenSignIn")}
              </Anchor>
            )}
            {account && (
              <div
                className="settings-fields"
                role="region"
                aria-label={t("aiAccountEditor")}
              >
                <ServiceField
                  path={`ai_services.accounts.${accountIndex}.id`}
                  label={t("aiObjectID")}
                >
                  {(id, error) => (
                    <TextInput
                      id={id}
                      value={account.id}
                      error={error}
                      disabled={locked}
                      onChange={(e) =>
                        accountChange(account.number, {
                          id: e.currentTarget.value,
                        })
                      }
                    />
                  )}
                </ServiceField>
                <ServiceField
                  path={`ai_services.accounts.${accountIndex}.type`}
                  label={t("type")}
                >
                  {(id, error) => (
                    <Select
                      id={id}
                      value={account.type}
                      error={error}
                      disabled={locked}
                      options={[
                        { value: "chatgpt", label: "ChatGPT" },
                        { value: "openai", label: "OpenAI API" },
                      ]}
                      onValueChange={(type) => {
                        const saved = query.data?.accounts?.find(
                          (value) =>
                            value.number === account.number &&
                            value.type === type,
                        );
                        accountChange(account.number, {
                          type: type as Account["type"],
                          api_key: "",
                          connected: saved?.connected || false,
                          email: saved?.email || "",
                          clear_secret: undefined,
                        });
                      }}
                    />
                  )}
                </ServiceField>
                <ServiceField
                  path={`ai_services.accounts.${accountIndex}.proxy`}
                  label={t("aiAccountProxy")}
                >
                  {(id, error) => (
                    <Select
                      id={id}
                      value={String(account.proxy)}
                      error={error}
                      disabled={locked}
                      options={proxyOptions}
                      onValueChange={(proxy) =>
                        accountChange(account.number, { proxy: Number(proxy) })
                      }
                    />
                  )}
                </ServiceField>
                {account.type === "openai" && (
                  <ServiceField
                    path={`ai_services.accounts.${accountIndex}.api_key`}
                    label="API Key"
                    help={
                      account.connected
                        ? t("aiSecretKeepHint")
                        : t("aiAPIKeyHint")
                    }
                  >
                    {(id, error) => (
                      <PasswordInput
                        id={id}
                        value={account.api_key || ""}
                        error={error}
                        disabled={locked}
                        autoComplete="new-password"
                        placeholder={
                          account.connected
                            ? t("aiSecretConfigured")
                            : undefined
                        }
                        onChange={(e) =>
                          accountChange(account.number, {
                            api_key: e.currentTarget.value,
                          })
                        }
                      />
                    )}
                  </ServiceField>
                )}
                {account.email && <SettingsHint>{account.email}</SettingsHint>}
                {account.number <= 0 && account.type === "chatgpt" && (
                  <SettingsHint>{t("aiSaveBeforeSignIn")}</SettingsHint>
                )}
              </div>
            )}
          </section>
          {(draft.definitions || []).map((definition) => (
            <FeatureSettings
              key={definition.name}
              definition={definition}
              value={draft.features[definition.name] || emptyFeature()}
              accounts={draft.accounts || []}
              disabled={locked}
              modelsBlocked={connectionsDirty}
              onChange={(values) => {
                const feature = {
                  ...(draft.features[definition.name] || emptyFeature()),
                  ...values,
                };
                Object.keys(values).forEach((key) =>
                  clear(`ai_services.features.${definition.name}.${key}`),
                );
                edit({
                  ...draft,
                  features: { ...draft.features, [definition.name]: feature },
                });
              }}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function FeatureSettings({
  definition,
  value,
  accounts,
  disabled,
  modelsBlocked,
  onChange,
}: {
  definition: Schema["FeatureDefinition"];
  value: Feature;
  accounts: Account[];
  disabled: boolean;
  modelsBlocked: boolean;
  onChange: (value: Partial<Feature>) => void;
}) {
  const { t } = useI18n();
  const path = `ai_services.features.${definition.name}`;
  const options = [
    { value: "0", label: t("aiFeatureDisabled"), disabled: false },
    ...accounts
      .filter((a) => (definition.account_types || []).includes(a.type))
      .map((a) => ({
        value: String(a.number),
        label: a.id || t("aiUnnamed"),
        disabled: false,
      })),
  ];
  if (!options.some((option) => option.value === String(value.account)))
    options.push({
      value: String(value.account),
      label: t("aiRemovedReference", { number: value.account }),
      disabled: true,
    });
  const account = accounts.find((a) => a.number === value.account);
  return (
    <div
      id={`settings-ai-${definition.name}`}
      className="settings-fields settings-subsection"
      role="region"
      aria-label={t(`${definition.name}Title` as MessageKey)}
    >
      <div className="settings-section-heading">
        <h3 className="settings-subsection-heading">
          {t(`${definition.name}Title` as MessageKey)}
        </h3>
        <HelpTooltip label={t(`${definition.name}Title` as MessageKey)}>
          {t(`${definition.name}BackfillHint` as MessageKey)}
        </HelpTooltip>
      </div>
      <ServiceField path={`${path}.account`} label={t("aiAccount")}>
        {(id, error) => (
          <Select
            id={id}
            value={String(value.account)}
            error={error}
            disabled={disabled}
            options={options}
            onValueChange={(account) => onChange({ account: Number(account) })}
          />
        )}
      </ServiceField>
      <ServiceField path={`${path}.model`} label={t("aiModel")}>
        {(id, error) => (
          <ModelInput
            id={id}
            error={error}
            value={value.model}
            account={account}
            blocked={modelsBlocked}
            disabled={disabled}
            onChange={(model) => onChange({ model })}
          />
        )}
      </ServiceField>
      {(definition.parameters || []).map((parameter) => (
        <ServiceField
          key={parameter}
          path={`${path}.${parameter}`}
          label={
            parameter === "reasoning_effort"
              ? t("aiReasoningEffort")
              : parameter === "image_detail"
                ? t("aiImageDetail")
                : parameter
          }
        >
          {(id, error) => (
            <Select
              id={id}
              error={error}
              value={String(value[parameter as keyof Feature] || "")}
              disabled={disabled}
              options={(definition.options[parameter] || []).map((option) => ({
                value: option,
                label: option || t("aiModelDefault"),
              }))}
              onValueChange={(next) => onChange({ [parameter]: next })}
            />
          )}
        </ServiceField>
      ))}
      <SettingsCheckbox
        label={t(`${definition.name}Auto` as MessageKey)}
        help={t(`${definition.name}AutoHint` as MessageKey)}
        checked={value.automatic}
        disabled={disabled}
        onChange={(event) =>
          onChange({ automatic: event.currentTarget.checked })
        }
      />
    </div>
  );
}

function ModelInput({
  id,
  error,
  value,
  account,
  blocked,
  disabled,
  onChange,
}: {
  id: string;
  error?: string;
  value: string;
  account?: Account;
  blocked: boolean;
  disabled: boolean;
  onChange: (model: string) => void;
}) {
  const { t } = useI18n();
  const [requested, setRequested] = useState(false);
  const models = useQuery({
    queryKey: ["ai-account-models", account?.number],
    queryFn: ({ signal }) =>
      get<Schema["Model"][]>(`/ai/accounts/${account!.number}/models`, signal),
    enabled:
      requested &&
      !!account?.connected &&
      account.number > 0 &&
      !blocked &&
      !disabled,
    staleTime: 60_000,
    retry: false,
  });
  return (
    <>
      <Autocomplete
        id={id}
        value={value}
        error={error}
        disabled={disabled}
        autoComplete="off"
        placeholder={t("aiModelPlaceholder")}
        data={(models.data || []).map((m) => ({
          value: m.id,
          label: m.name === m.id || !m.name ? m.id : `${m.id} · ${m.name}`,
        }))}
        onChange={onChange}
        onFocus={() => setRequested(true)}
        rightSection={
          requested && models.isFetching ? <Loader size="xs" /> : undefined
        }
      />
      {requested && models.isFetching && (
        <TaskStatus>{t("aiModelsLoading")}</TaskStatus>
      )}
      {requested &&
        models.isSuccess &&
        !models.isFetching &&
        models.data.length === 0 && (
          <TaskStatus>{t("aiModelsEmpty")}</TaskStatus>
        )}
      {requested && models.isError && (
        <Group>
          <TaskError>{errorMessage(models.error, t)}</TaskError>
          <Anchor
            component="button"
            type="button"
            className="text-action-link"
            size="xs"
            disabled={blocked || disabled}
            onClick={() => void models.refetch()}
          >
            {t("retry")}
          </Anchor>
        </Group>
      )}
    </>
  );
}
