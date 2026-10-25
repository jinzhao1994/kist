import { Menu } from "@mantine/core";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { get } from "./api";
import { useI18n } from "./i18n";
import {
  navigationQuery,
  permanentURL,
  routeURL,
  type NavigationLocation,
  type Route,
} from "./navigation";
import { errorMessage } from "./mutations";

export function useObjectLink(target: Route) {
  const query = useQuery({
    queryKey: ["navigation", navigationQuery(target)],
    queryFn: ({ signal }) =>
      get<NavigationLocation>(navigationQuery(target), signal),
    retry: false,
  });
  const href = query.data
    ? routeURL({
        ...target,
        permanent: false,
        readablePath: query.data.path,
        view: query.data.view,
      })
    : undefined;
  return { ...query, href };
}
export function ObjectLinkMenuItems({
  target,
  switchAddress = false,
  openNewTab = false,
  copy: allowCopy = true,
}: {
  target: Route;
  switchAddress?: boolean;
  openNewTab?: boolean;
  copy?: boolean;
}) {
  const { t } = useI18n();
  const link = useObjectLink(target);
  const [status, setStatus] = useState<string>();
  const permanent = permanentURL(target);
  async function copy(href: string) {
    try {
      await navigator.clipboard.writeText(
        new URL(href, window.location.origin).href,
      );
      setStatus(t("copied"));
    } catch {
      setStatus(t("copyLinkFailed"));
    }
  }
  return (
    <>
      {openNewTab && (
        <Menu.Item
          component="a"
          href={link.href}
          disabled={!link.href}
          target="_blank"
          rel="noopener noreferrer"
        >
          {t("openNewTab")}
        </Menu.Item>
      )}
      {allowCopy && (
        <>
          <Menu.Item
            disabled={!link.href}
            closeMenuOnClick={false}
            onClick={() => link.href && void copy(link.href)}
          >
            {t("copyLink")}
          </Menu.Item>
          <Menu.Item
            closeMenuOnClick={false}
            onClick={() => void copy(permanent)}
          >
            {t("copyPermanentLink")}
          </Menu.Item>
        </>
      )}
      {switchAddress && (
        <Menu.Item
          onClick={() => {
            history.replaceState(
              target.file
                ? {
                    ...history.state,
                    kistScope: target.scope || {
                      box: target.box,
                      folder: target.folder,
                      tag: target.tag,
                    },
                  }
                : history.state,
              "",
              permanent + window.location.hash,
            );
            window.dispatchEvent(new PopStateEvent("popstate"));
          }}
        >
          {t("switchPermanentLink")}
        </Menu.Item>
      )}
      {status && <Menu.Label role="status">{status}</Menu.Label>}
      {link.error && (
        <Menu.Label role="alert">{errorMessage(link.error, t)}</Menu.Label>
      )}
    </>
  );
}
