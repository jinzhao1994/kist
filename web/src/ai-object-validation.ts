import type { components } from "./api-schema";

type Schema = components["schemas"];
type Errors = Record<string, string>;

function validHost(host: string) {
  if (!host || host.length > 253) return false;
  if (host.startsWith("[") && host.endsWith("]")) {
    try {
      return new URL(`http://${host}`).hostname.startsWith("[");
    } catch {
      return false;
    }
  }
  return host
    .replace(/\.$/, "")
    .split(".")
    .every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label));
}

export function validProxyAddress(value: string) {
  if (!value || value !== value.trim() || value.length > 2048) return false;
  try {
    const url = new URL(value);
    if (
      !["http:", "https:", "socks5:", "socks5h:"].includes(url.protocol) ||
      !value.toLowerCase().startsWith(`${url.protocol}//`) ||
      (url.pathname !== "" && url.pathname !== "/") ||
      url.search ||
      url.hash
    )
      return false;
    const authority = value.slice(value.indexOf("//") + 2).split(/[/?#]/)[0];
    const hostPort = authority.slice(authority.lastIndexOf("@") + 1);
    const host = hostPort.startsWith("[")
      ? hostPort.slice(0, hostPort.indexOf("]") + 1)
      : hostPort.split(":")[0];
    return (
      !authority.endsWith(":") &&
      validHost(host) &&
      (!url.port || (Number(url.port) >= 1 && Number(url.port) <= 65535))
    );
  } catch {
    return false;
  }
}

// These checks gate local add/done actions. The server remains authoritative
// when the whole settings draft is saved; no validation request is issued.
export function aiObjectErrors(
  value: Schema["AIProxySettings"] | Schema["AIAccountSettings"],
  peers: { number: number; id: string }[],
): Errors {
  const errors: Errors = {};
  if (!value.id.trim()) errors.id = "required";
  else if (
    value.id !== value.id.trim() ||
    new TextEncoder().encode(value.id).length > 128 ||
    /\p{Cc}/u.test(value.id) ||
    peers.some((peer) => peer.number !== value.number && peer.id === value.id)
  )
    errors.id = "unique_id";
  if ("url" in value) {
    if (value.type === "http") {
      if (!validProxyAddress(value.url)) errors.url = "proxy";
    } else {
      // Trojan stores IPv6 hosts without URL brackets.
      if (!validHost(value.host.includes(":") ? `[${value.host}]` : value.host))
        errors.host = "host";
      if (!Number.isInteger(value.port) || value.port < 1 || value.port > 65535)
        errors.port = "port";
      if (
        !(value.password || "").length &&
        (!value.secret_configured || value.clear_secret)
      )
        errors.password = "required";
      if (
        value.server_name &&
        !validHost(
          value.server_name.includes(":")
            ? `[${value.server_name}]`
            : value.server_name,
        )
      )
        errors.server_name = "host";
    }
  } else if (value.type === "openai" && /\s/.test(value.api_key || ""))
    errors.api_key = "api_key";
  return errors;
}
