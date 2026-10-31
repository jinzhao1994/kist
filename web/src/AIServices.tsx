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
import { SettingsCollection } from "./SettingsCollection";
import { AIProxyRow } from "./AIProxyRow";
import { ConnectionTest, ConnectionStatus } from "./ConnectionTest";
import { SecretInput } from "./SecretInput";
import { HelpTooltip } from "./HelpTooltip";
import { Select } from "./Select";
import {
  SettingsCheckbox,
  SettingsErrorsContext,
  SettingsFieldHeading,
  SettingsFieldRow,
  SettingsHint,
  SettingsReset,
  useSettingsError,
  useSettingsPending,
  type SettingsSectionDraft,
  type SettingsErrors,
} from "./SettingsFields";
import { aiObjectErrors } from "./ai-object-validation";
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
      ({
        connected: _,
        email: _email,
        authentication_error: _auth,
        ...account
      }) => ({
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
    return address.replace(/(^|\/\/)[^/@\s]*@/g, "$1") || "—";
  }
}
const objectNumber = (number: number, pendingLabel: string) =>
  number > 0 ? String(number) : pendingLabel;

function ServiceField({
  path,
  label,
  help,
  wide = false,
  required = false,
  children,
}: {
  path: string;
  label: string;
  help?: string;
  wide?: boolean;
  required?: boolean;
  children: (id: string, error?: string) => ReactNode;
}) {
  const id = useId();
  const error = useSettingsError(path);
  return (
    <div
      className={wide ? "setting-field settings-editor-wide" : "setting-field"}
      data-setting-field={path}
    >
      <SettingsFieldHeading
        label={label}
        help={help}
        htmlFor={id}
        required={required}
      />
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
  const [creatingProxy, setCreatingProxy] = useState<number>();
  const [creatingAccount, setCreatingAccount] = useState<number>();
  const originalProxy = useRef<Proxy | undefined>(undefined);
  const originalAccount = useRef<Account | undefined>(undefined);
  const [busy, setBusy] = useState<number>();
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loginURL, setLoginURL] = useState("");
  const pendingAccount = useRef(0);
  const ownedSignIns = useRef(new Set<string>());
  const authorizationGeneration = useRef(0);
  const [signIn, setSignIn] = useState<{ id: string; number: number }>();
  const signInQuery = useQuery({
    queryKey: ["ai-sign-in", signIn?.id],
    queryFn: ({ signal }) =>
      get<Schema["SignInDraftView"]>(`/ai/sign-ins/${signIn!.id}`, signal),
    enabled: !!signIn,
    refetchInterval: (query) => (query.state.data?.pending ? 1000 : false),
    retry: false,
  });
  function discardSignIn(id: string) {
    ownedSignIns.current.delete(id);
    void fetch(`/api/v1/ai/sign-ins/${id}`, {
      method: "DELETE",
      keepalive: true,
    }).catch(() => {});
  }
  useEffect(
    () => () => {
      authorizationGeneration.current++;
      for (const id of [...ownedSignIns.current]) discardSignIn(id);
    },
    [],
  );
  const temporary = useRef(-1);
  const context = useContext(SettingsErrorsContext);
  const [localErrors, setLocalErrors] = useState<SettingsErrors>({});
  const errors = useMemo(
    () => ({ ...context.errors, ...localErrors }),
    [context.errors, localErrors],
  );
  function clear(path: string) {
    context.clear(path);
    setLocalErrors((old) => {
      if (!old[path]) return old;
      const next = { ...old };
      delete next[path];
      return next;
    });
  }
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
    authorizationGeneration.current++;
    for (const id of [...ownedSignIns.current]) discardSignIn(id);
    setBusy(undefined);
    setSignIn(undefined);
    setLoginURL("");
    setTouched(false);
    setDraft(queries.getQueryData(["ai-services"]));
    setError("");
    setNotice("");
    setProxyEditor(undefined);
    setAccountEditor(undefined);
    setCreatingProxy(undefined);
    setCreatingAccount(undefined);
    setLocalErrors({});
  }, [reloadVersion, queries]);
  const creationRequiresLogin = !!draft?.accounts?.some(
    (a) => a.number === creatingAccount && a.type === "chatgpt" && !a.connected,
  );
  useEffect(() => {
    onDraftChange?.(
      dirty && pending
        ? {
            value: pending,
            ...(creationRequiresLogin ? { blocked: true } : {}),
          }
        : null,
    );
  }, [dirty, pending, onDraftChange, creationRequiresLogin]);
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
      if (match[1] === "proxies" && number !== creatingProxy) {
        if (proxyEditor !== number)
          originalProxy.current = draft.proxies?.find(
            (p) => p.number === number,
          );
        setCreatingProxy(undefined);
        setProxyEditor(number);
      } else if (match[1] === "accounts" && number !== creatingAccount) {
        if (accountEditor !== number)
          originalAccount.current = draft.accounts?.find(
            (a) => a.number === number,
          );
        setCreatingAccount(undefined);
        setAccountEditor(number);
      }
    }
  }, [
    errors,
    draft,
    creatingProxy,
    creatingAccount,
    proxyEditor,
    accountEditor,
  ]);
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
  useEffect(() => {
    const retained = new Set(
      draft?.accounts?.map((a) => a.login_draft).filter(Boolean),
    );
    for (const id of [...ownedSignIns.current]) {
      if (!retained.has(id)) {
        discardSignIn(id);
        if (signIn?.id === id) {
          setSignIn(undefined);
          setLoginURL("");
        }
      }
    }
  }, [draft, signIn]);
  useEffect(() => {
    if (!signIn || !signInQuery.data || signInQuery.data.pending) return;
    const value = signInQuery.data;
    if (value.connected) {
      const index =
        draft?.accounts?.findIndex((a) => a.number === signIn.number) ?? -1;
      if (index >= 0) clear(`ai_services.accounts.${index}.login_draft`);
      if (
        creatingAccount === signIn.number &&
        draft?.accounts?.[index]?.login_draft === signIn.id
      )
        setCreatingAccount(undefined);
    }
    setDraft(
      (current) =>
        current && {
          ...current,
          accounts: (current.accounts || []).map((a) =>
            a.number === signIn.number && a.login_draft === signIn.id
              ? { ...a, connected: value.connected, email: value.email }
              : a,
          ),
        },
    );
  }, [signIn, signInQuery.data]);
  async function accountAction(
    number: number,
    action: "connect" | "disconnect",
  ) {
    const target = draft?.accounts?.find((a) => a.number === number);
    if (!draft || !target) return;
    if (
      action === "connect" &&
      !finishObject("accounts", target, draft.accounts!.indexOf(target))
    )
      return;
    const generation = authorizationGeneration.current;
    const login =
      action === "connect" ? window.open("about:blank", "_blank") : null;
    if (login) login.opener = null;
    setBusy(number);
    setError("");
    setNotice("");
    try {
      if (action === "disconnect" && target.login_draft) {
        discardSignIn(target.login_draft);
        if (signIn?.id === target.login_draft) setSignIn(undefined);
        const saved = query.data?.accounts?.find(
          (a) => a.number === number && a.type === target.type,
        );
        accountChange(number, {
          login_draft: undefined,
          connected: saved?.connected || false,
          email: saved?.email || "",
        });
        return;
      }
      const value = await result<{
        id?: string;
        url?: string;
        revoked?: boolean;
      }>(
        await fetch(
          action === "connect"
            ? "/api/v1/ai/sign-ins"
            : `/api/v1/ai/accounts/${number}/${action}`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body:
              action === "connect"
                ? JSON.stringify({ number, settings: aiServicesDraft(draft) })
                : "{}",
          },
        ),
      );
      if (generation !== authorizationGeneration.current) {
        if (value.id) discardSignIn(value.id);
        login?.close();
        return;
      }
      if (value.id) {
        if (target.login_draft) discardSignIn(target.login_draft);
        ownedSignIns.current.add(value.id);
        setSignIn({ id: value.id, number });
        accountChange(number, {
          login_draft: value.id,
          clear_secret: undefined,
          connected: false,
        });
      }
      if (value.url) {
        setLoginURL(value.url);
        if (login && !login.closed) login.location.replace(value.url);
      }
      if (value.revoked === false) setNotice(t("aiRevocationUnconfirmed"));
    } catch (error) {
      login?.close();
      if (generation === authorizationGeneration.current)
        setError(errorMessage(error, t));
    } finally {
      if (generation === authorizationGeneration.current) {
        await queries.invalidateQueries({ queryKey: ["ai-services"] });
        if (action === "disconnect" && !target.login_draft) await onChanged?.();
        setBusy(undefined);
      }
    }
  }
  const locked = disabled || busy !== undefined;
  const proxy = draft?.proxies?.find(
    (p) => p.number === (creatingProxy ?? proxyEditor),
  );
  const proxyIndex =
    draft?.proxies?.findIndex(
      (p) => p.number === (creatingProxy ?? proxyEditor),
    ) ?? -1;
  const account = draft?.accounts?.find(
    (a) => a.number === (creatingAccount ?? accountEditor),
  );
  const accountIndex =
    draft?.accounts?.findIndex(
      (a) => a.number === (creatingAccount ?? accountEditor),
    ) ?? -1;
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
  const signInLabel = (account: Account) =>
    t(
      account.connected ||
        query.data?.accounts?.some(
          (saved) =>
            saved.number === account.number &&
            saved.type === "chatgpt" &&
            saved.connected,
        )
        ? "aiSignInAgain"
        : "aiSignIn",
    );
  const connectionBlocked =
    disabled || configurationPending || dirty || busy !== undefined;
  const signInPending =
    !!signIn &&
    (!signInQuery.data || signInQuery.data.pending) &&
    !signInQuery.error;
  const signInFeedback = (value: Account) =>
    value.login_draft && (
      <div
        data-setting-field={`ai_services.accounts.${draft!.accounts!.indexOf(value)}.login_draft`}
      >
        {authorizationFeedback(value)}
      </div>
    );
  function authorizationFeedback(value: Account) {
    const failure =
      errors[
        `ai_services.accounts.${draft!.accounts!.indexOf(value)}.login_draft`
      ];
    if (failure)
      return (
        <TaskError>
          {t(
            failure.code === "authorization_pending"
              ? "aiSignInPendingSave"
              : "aiSignInExpired",
          )}
        </TaskError>
      );
    if (
      (signIn?.id === value.login_draft &&
        (signInQuery.error || signInQuery.data?.error)) ||
      (signIn?.id !== value.login_draft && !value.connected)
    )
      return <TaskError>{t("aiSignInExpired")}</TaskError>;
    return (
      <TaskStatus>
        {t(
          signIn?.id === value.login_draft && signInPending
            ? "aiSigningIn"
            : "aiSignInUnsaved",
        )}
      </TaskStatus>
    );
  }
  function finishObject(
    kind: "proxies" | "accounts",
    value: Proxy | Account,
    index: number,
  ) {
    const prefix = `ai_services.${kind}.${index}.`;
    const fields = aiObjectErrors(value, draft?.[kind] || []);
    const entries = Object.entries(fields).map(
      ([key, code]) =>
        [
          prefix + key,
          { field: prefix + key, code, message: "Invalid field" },
        ] as const,
    );
    setLocalErrors((old) => ({
      ...Object.fromEntries(
        Object.entries(old).filter(([key]) => !key.startsWith(prefix)),
      ),
      ...Object.fromEntries(entries),
    }));
    if (entries.length) {
      const field = Array.from(
        document.querySelectorAll<HTMLElement>("[data-setting-field]"),
      ).find((element) => element.dataset.settingField === entries[0][0]);
      field?.scrollIntoView({ block: "center" });
      field?.querySelector<HTMLElement>('input, [role="combobox"]')?.focus();
      return false;
    }
    Object.keys(context.errors)
      .filter((key) => key.startsWith(prefix))
      .forEach(context.clear);
    return true;
  }
  const proxyForm = proxy && (
    <form
      className="settings-object-editor"
      onSubmit={(event) => {
        event.preventDefault();
        if (!finishObject("proxies", proxy, proxyIndex)) return;
        setCreatingProxy(undefined);
        setProxyEditor(undefined);
      }}
      role="region"
      aria-label={t(
        creatingProxy !== undefined ? "aiAddProxy" : "aiProxyEditor",
      )}
    >
      <ServiceField
        required
        path={`ai_services.proxies.${proxyIndex}.id`}
        label={t("aiObjectID")}
        help={t("aiObjectIDHint")}
      >
        {(id, error) => (
          <TextInput
            id={id}
            autoFocus={creatingProxy === undefined}
            aria-required="true"
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
                (value) => value.number === proxy.number && value.type === type,
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
            wide
            path={`ai_services.proxies.${proxyIndex}.url`}
            label={t("aiProxyAddress")}
            help={t("aiProxySecretHint")}
          >
            {(id, error) => (
              <TextInput
                id={id}
                type="text"
                value={proxy.url}
                error={error}
                disabled={locked}
                autoComplete="off"
                spellCheck={false}
                autoCapitalize="none"
                onChange={(e) =>
                  proxyChange(proxy.number, {
                    url: e.currentTarget.value,
                  })
                }
              />
            )}
          </ServiceField>
          {proxy.secret_configured && (
            <div className="settings-editor-wide">
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
            </div>
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
            help={proxy.secret_configured ? t("aiSecretKeepHint") : undefined}
          >
            {(id, error) => (
              <PasswordInput
                id={id}
                value={proxy.password || ""}
                error={error}
                disabled={locked}
                autoComplete="new-password"
                placeholder={
                  proxy.secret_configured ? t("aiSecretConfigured") : undefined
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
      <Group className="settings-editor-wide" gap="xs">
        <Button type="submit" variant="filled" disabled={locked}>
          {t(creatingProxy !== undefined ? "aiAddProxy" : "aiFinishEditing")}
        </Button>
        <Button
          disabled={locked}
          onClick={() => {
            Object.keys(errors)
              .filter((key) => key.startsWith("ai_services.proxies."))
              .forEach(clear);
            if (creatingProxy !== undefined) {
              edit({
                ...draft!,
                proxies: draft!.proxies!.filter(
                  (p) => p.number !== creatingProxy,
                ),
              });
              setCreatingProxy(undefined);
            } else if (originalProxy.current) {
              const original = originalProxy.current;
              edit({
                ...draft!,
                proxies: draft!.proxies!.map((p) =>
                  p.number === original.number ? original : p,
                ),
              });
            }
            setProxyEditor(undefined);
          }}
        >
          {t("cancel")}
        </Button>
      </Group>
    </form>
  );
  const accountForm = account && (
    <form
      className="settings-object-editor"
      autoComplete="off"
      onSubmit={(event) => {
        event.preventDefault();
        if (!finishObject("accounts", account, accountIndex)) return;
        if (
          creatingAccount !== undefined &&
          account.type === "chatgpt" &&
          !account.connected
        ) {
          void accountAction(account.number, "connect");
          return;
        }
        setCreatingAccount(undefined);
        setAccountEditor(undefined);
      }}
      role="region"
      aria-label={t(
        creatingAccount !== undefined ? "aiAddAccount" : "aiAccountEditor",
      )}
    >
      <ServiceField
        required
        path={`ai_services.accounts.${accountIndex}.id`}
        label={t("aiObjectID")}
        help={t("aiObjectIDHint")}
      >
        {(id, error) => (
          <TextInput
            id={id}
            autoFocus={creatingAccount === undefined}
            aria-required="true"
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
                  value.number === account.number && value.type === type,
              );
              accountChange(account.number, {
                type: type as Account["type"],
                api_key: "",
                connected: saved?.connected || false,
                email: saved?.email || "",
                clear_secret: undefined,
                login_draft: undefined,
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
          help={account.connected ? t("aiSecretKeepHint") : t("aiAPIKeyHint")}
        >
          {(id, error) => (
            <SecretInput
              id={id}
              value={account.api_key || ""}
              error={error}
              disabled={locked}
              placeholder={
                account.connected ? t("aiSecretConfigured") : undefined
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
      {account.email && (
        <div className="settings-editor-wide">
          <SettingsHint>{account.email}</SettingsHint>
        </div>
      )}
      {account.type === "chatgpt" && (
        <div className="settings-editor-wide">
          <Group gap="md">
            {creatingAccount === undefined &&
              rowAction(
                signInLabel(account),
                () => void accountAction(account.number, "connect"),
                locked || signInPending,
              )}
            {account.login_draft &&
              rowAction(
                t("cancel"),
                () => void accountAction(account.number, "disconnect"),
                locked,
              )}
          </Group>
          {signInFeedback(account)}
        </div>
      )}
      <Group className="settings-editor-wide" gap="xs">
        <Button
          type="submit"
          variant="filled"
          disabled={locked || signInPending}
        >
          {t(
            creatingAccount !== undefined
              ? account.type === "chatgpt"
                ? "aiSignInAndAdd"
                : "aiAddAccount"
              : "aiFinishEditing",
          )}
        </Button>
        <Button
          disabled={locked}
          onClick={() => {
            Object.keys(errors)
              .filter((key) => key.startsWith("ai_services.accounts."))
              .forEach(clear);
            if (creatingAccount !== undefined) {
              edit({
                ...draft!,
                accounts: draft!.accounts!.filter(
                  (a) => a.number !== creatingAccount,
                ),
              });
              setCreatingAccount(undefined);
            } else if (originalAccount.current) {
              const original = originalAccount.current;
              edit({
                ...draft!,
                accounts: draft!.accounts!.map((a) =>
                  a.number === original.number ? original : a,
                ),
              });
            }
            setAccountEditor(undefined);
          }}
        >
          {t("cancel")}
        </Button>
      </Group>
    </form>
  );
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
          <SettingsCollection
            id="settings-ai-proxies"
            title={t("aiProxies")}
            help={t("aiProxiesHint")}
            addLabel={t("aiAddProxy")}
            emptyLabel={t("aiNoProxies")}
            items={draft.proxies || []}
            creating={creatingProxy}
            editing={proxyEditor}
            disabled={locked}
            minWidth={480}
            headers={[
              t("aiNumber"),
              t("aiObjectID"),
              t("type"),
              t("aiEndpoint"),
            ]}
            onCreate={() => {
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
              setCreatingProxy(number);
              setProxyEditor(undefined);
            }}
            editor={
              <SettingsErrorsContext value={{ errors, clear }}>
                {proxyForm}
              </SettingsErrorsContext>
            }
            renderRow={(p) => (
              <AIProxyRow
                number={p.number}
                name={p.id || t("aiUnnamed")}
                type={p.type === "http" ? "HTTP" : "Trojan"}
                address={
                  p.type === "http"
                    ? proxyAddress(p.url)
                    : `${p.host}:${p.port}`
                }
                revision={draft.revision}
                saved={
                  p.number > 0 &&
                  !changed(
                    p,
                    query.data?.proxies?.find(
                      (value) => value.number === p.number,
                    ),
                  )
                }
                disabled={
                  locked ||
                  configurationPending ||
                  creatingProxy !== undefined ||
                  proxyEditor !== undefined
                }
              >
                {rowAction(
                  t("aiEdit"),
                  () => {
                    originalProxy.current = p;
                    setProxyEditor(p.number);
                  },
                  locked ||
                    creatingProxy !== undefined ||
                    proxyEditor !== undefined,
                )}
                {rowAction(
                  t("aiDelete"),
                  () => {
                    edit({
                      ...draft,
                      proxies: draft.proxies!.filter(
                        (value) => value.number !== p.number,
                      ),
                    });
                    if (proxyEditor === p.number) setProxyEditor(undefined);
                  },
                  locked ||
                    creatingProxy !== undefined ||
                    proxyEditor !== undefined,
                )}
              </AIProxyRow>
            )}
          />
          <SettingsCollection
            id="settings-ai-accounts"
            title={t("aiAccounts")}
            help={t("aiAccountsHint")}
            addLabel={t("aiAddAccount")}
            emptyLabel={t("aiNoAccounts")}
            items={draft.accounts || []}
            creating={creatingAccount}
            editing={accountEditor}
            disabled={locked}
            minWidth={560}
            headers={[
              t("aiNumber"),
              t("aiObjectID"),
              t("type"),
              t("aiAccountProxy"),
              t("backgroundTaskState"),
            ]}
            onCreate={() => {
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
              setCreatingAccount(number);
              setAccountEditor(undefined);
            }}
            editor={
              <SettingsErrorsContext value={{ errors, clear }}>
                {accountForm}
              </SettingsErrorsContext>
            }
            renderRow={(a) => (
              <ConnectionTest<Schema["AIAccountTestResult"]>
                path={`/ai/accounts/${a.number}/test`}
                revision={draft.revision}
                saved={a.number > 0 && !connectionsDirty}
                disabled={
                  connectionBlocked ||
                  !a.connected ||
                  !!query.data?.pending_account ||
                  creatingAccount !== undefined ||
                  accountEditor !== undefined
                }
                hint={t(
                  dirty || configurationPending
                    ? "aiAccountTestSaveFirst"
                    : a.connected
                      ? "aiAccountTestHint"
                      : "aiAccountTestConfigure",
                )}
                saveHint={t("aiAccountTestSaveFirst")}
                reasons={{
                  not_configured: "aiAccountTestConfigure",
                  authentication: "aiAccountTestAuth",
                  permission: "aiAccountTestPermission",
                  rate_limited: "aiAccountTestRate",
                  timeout: "aiAccountTestTimeout",
                  connection: "aiAccountTestConnection",
                  service: "aiAccountTestService",
                }}
                success={(value) =>
                  t("aiAccountTestResult", {
                    count: value.models?.length || 0,
                    ms: value.elapsed_ms,
                  })
                }
              >
                {({ action, feedback, testing }) => (
                  <>
                    <Table.Td className="settings-number-column">
                      {objectNumber(a.number, t("aiPendingNumber"))}
                    </Table.Td>
                    <Table.Th scope="row">{a.id || t("aiUnnamed")}</Table.Th>
                    <Table.Td>
                      {a.type === "chatgpt" ? "ChatGPT" : "OpenAI API"}
                    </Table.Td>
                    <Table.Td>
                      {a.proxy
                        ? draft.proxies?.find((p) => p.number === a.proxy)
                            ?.id || t("aiRemovedReference", { number: a.proxy })
                        : t("aiDirect")}
                    </Table.Td>
                    <Table.Td>
                      <ConnectionStatus
                        configured={a.connected}
                        pending={
                          query.data?.pending_account === a.number ||
                          (signIn?.number === a.number && signInPending)
                        }
                      />
                      {a.authentication_error && !a.login_draft && (
                        <TaskError>
                          {t(
                            a.authentication_error === "reauthenticate"
                              ? "aiLoginRequired"
                              : "aiRefreshFailed",
                          )}
                        </TaskError>
                      )}
                      {feedback}
                      {signInFeedback(a)}
                    </Table.Td>
                    <Table.Td>
                      <Group
                        gap="md"
                        justify="flex-end"
                        wrap="nowrap"
                        className="table-row-actions"
                        data-busy={busy === a.number || testing || undefined}
                      >
                        {a.type === "chatgpt" &&
                          rowAction(
                            signInLabel(a),
                            () => void accountAction(a.number, "connect"),
                            locked ||
                              signInPending ||
                              !!query.data?.pending_account,
                          )}
                        {a.type === "chatgpt" &&
                          (a.connected ||
                            a.login_draft ||
                            query.data?.pending_account === a.number) &&
                          rowAction(
                            t(
                              query.data?.pending_account === a.number ||
                                a.login_draft
                                ? "cancel"
                                : "aiDisconnect",
                            ),
                            () => void accountAction(a.number, "disconnect"),
                            a.login_draft
                              ? locked
                              : connectionBlocked || a.number <= 0,
                          )}
                        {action}
                        {rowAction(
                          t("aiEdit"),
                          () => {
                            originalAccount.current = a;
                            setAccountEditor(a.number);
                          },
                          locked ||
                            creatingAccount !== undefined ||
                            accountEditor !== undefined,
                        )}
                        {rowAction(
                          t("aiDelete"),
                          () => {
                            edit({
                              ...draft,
                              accounts: draft.accounts!.filter(
                                (value) => value.number !== a.number,
                              ),
                            });
                            if (accountEditor === a.number)
                              setAccountEditor(undefined);
                          },
                          locked ||
                            creatingAccount !== undefined ||
                            accountEditor !== undefined,
                        )}
                      </Group>
                    </Table.Td>
                  </>
                )}
              </ConnectionTest>
            )}
          />
          {loginURL && (!!query.data?.pending_account || signInPending) && (
            <Anchor href={loginURL} target="_blank" rel="noreferrer">
              {t("aiOpenSignIn")}
            </Anchor>
          )}
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
  const parameters = definition.parameters || [];
  const automaticField = (
    <SettingsCheckbox
      label={t(`${definition.name}Auto` as MessageKey)}
      help={t(`${definition.name}AutoHint` as MessageKey)}
      checked={value.automatic}
      disabled={disabled}
      onChange={(event) => onChange({ automatic: event.currentTarget.checked })}
    />
  );
  const parameterField = (parameter: string) => (
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
  );

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
      {definition.name !== "orientation" && automaticField}
      <SettingsFieldRow
        columns={parameters.includes("reasoning_effort") ? 3 : 2}
      >
        <ServiceField path={`${path}.account`} label={t("aiAccount")}>
          {(id, error) => (
            <Select
              id={id}
              value={String(value.account)}
              error={error}
              disabled={disabled}
              options={options}
              onValueChange={(account) =>
                onChange({ account: Number(account) })
              }
            />
          )}
        </ServiceField>
        <ServiceField path={`${path}.model`} label={t("aiModel")}>
          {(id, error) => (
            <ModelInput
              id={id}
              error={error}
              value={value.model || definition.default_model || ""}
              account={account}
              feature={definition.name}
              allowedModels={definition.models || []}
              blocked={modelsBlocked}
              disabled={disabled}
              onChange={(model) => onChange({ model })}
            />
          )}
        </ServiceField>
        {parameters.includes("reasoning_effort") &&
          parameterField("reasoning_effort")}
      </SettingsFieldRow>
      {!!parameters.filter((parameter) => parameter !== "reasoning_effort")
        .length && (
        <SettingsFieldRow>
          {parameters
            .filter((parameter) => parameter !== "reasoning_effort")
            .map(parameterField)}
        </SettingsFieldRow>
      )}
      {definition.name === "orientation" && automaticField}
      <SettingsCheckbox
        label={t("backgroundPauseOnError")}
        help={t("backgroundPauseOnErrorHint")}
        checked={value.pause_on_error ?? true}
        disabled={disabled}
        action={
          <SettingsReset
            specified={value.pause_on_error != null}
            disabled={disabled || value.pause_on_error == null}
            onReset={() => onChange({ pause_on_error: undefined })}
          />
        }
        onChange={(event) =>
          onChange({ pause_on_error: event.currentTarget.checked })
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
  feature,
  allowedModels,
  blocked,
  disabled,
  onChange,
}: {
  id: string;
  error?: string;
  value: string;
  account?: Account;
  feature: string;
  allowedModels: string[];
  blocked: boolean;
  disabled: boolean;
  onChange: (model: string) => void;
}) {
  const { t } = useI18n();
  const [requested, setRequested] = useState(false);
  const models = useQuery({
    queryKey: [
      "ai-account-models",
      account?.number,
      ...(allowedModels.length ? [feature] : []),
    ],
    queryFn: ({ signal }) =>
      get<Schema["Model"][]>(
        `/ai/accounts/${account!.number}/models${allowedModels.length ? `?feature=${encodeURIComponent(feature)}` : ""}`,
        signal,
      ),
    enabled:
      requested &&
      !!account?.connected &&
      account.number > 0 &&
      !blocked &&
      !disabled,
    staleTime: 60_000,
    retry: false,
  });
  const choices = allowedModels.length
    ? (models.data || allowedModels.map((id) => ({ id, name: id }))).filter(
        (model) => allowedModels.includes(model.id),
      )
    : models.data || [];
  const selectOptions = choices.map((model) => ({
    value: model.id,
    label:
      model.name === model.id || !model.name
        ? model.id
        : `${model.id} · ${model.name}`,
  }));
  if (
    allowedModels.length &&
    !selectOptions.some((option) => option.value === value) &&
    value
  )
    selectOptions.unshift({ value, label: value });
  return (
    <>
      {allowedModels.length ? (
        <div onFocusCapture={() => setRequested(true)}>
          <Select
            id={id}
            error={error}
            value={value}
            disabled={disabled}
            options={selectOptions.map((option) => ({
              ...option,
              disabled:
                !!models.data &&
                !choices.some((model) => model.id === option.value),
            }))}
            onValueChange={onChange}
          />
        </div>
      ) : (
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
      )}
      {requested && models.isFetching && (
        <TaskStatus>{t("aiModelsLoading")}</TaskStatus>
      )}
      {requested &&
        models.isSuccess &&
        !models.isFetching &&
        choices.length === 0 && <TaskStatus>{t("aiModelsEmpty")}</TaskStatus>}
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
