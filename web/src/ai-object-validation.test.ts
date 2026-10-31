import { expect, it } from "vitest";
import { validProxyAddress } from "./ai-object-validation";

it("accepts supported proxy URLs and rejects malformed hosts, protocols and ports", () => {
  for (const value of [
    "http://127.0.0.1:7890",
    "https://proxy.example",
    "http://localhost",
    "socks5://proxy.example:1080",
    "socks5h://name:secret@proxy.example:1080",
    "http://[2001:DB8::1]:7890",
  ])
    expect(validProxyAddress(value), value).toBe(true);
  for (const value of [
    "",
    "456",
    "   ",
    "http:proxy.example",
    "ftp://proxy.example",
    "http://proxy.example:0",
    "http://proxy.example:65536",
    "http://proxy.example:",
    "http://invalid..example",
    "http://-proxy.example",
    "http://proxy.example/path",
    "http://proxy.example?query=1",
    "http://proxy.example#fragment",
  ])
    expect(validProxyAddress(value), value).toBe(false);
});
