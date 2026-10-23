import { expect, it } from "vitest";
import { routeURL } from "./navigation";

it("targets the storage section without changing ordinary settings navigation", () => {
  const route = { files: false, section: "settings" } as const;
  expect(routeURL(route, "settings-storage")).toBe(
    "/settings#settings-storage",
  );
  expect(routeURL(route)).toBe("/settings");
});
