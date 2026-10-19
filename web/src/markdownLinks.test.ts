import { expect, it } from "vitest";
import { markdownURL, relativeTarget } from "./markdownLinks";
import type { Folder } from "./api";
const folders = [
  { id: "root", name: "", parent_id: null },
  { id: "notes", name: "notes", parent_id: "root" },
  { id: "images", name: "images", parent_id: "root" },
] as Folder[];
it("resolves relative paths within the archive box", () => {
  expect(
    relativeTarget("../images/%E4%B8%AD%E6%96%87.png", "notes", folders),
  ).toEqual({ folder: "images", name: "中文.png" });
  expect(relativeTarget("./README.md#intro", "notes", folders)).toEqual({
    folder: "notes",
    name: "README.md",
  });
  expect(relativeTarget("/images/a.png", "notes", folders)).toEqual({
    folder: "images",
    name: "a.png",
  });
  for (const path of [
    "../../outside",
    "%2e%2e/%2e%2e/outside",
    "missing/a",
    "bad%2Fname.png",
    "%00.png",
    "bad%.png",
    "https://example.org/a",
  ])
    expect(relativeTarget(path, "notes", folders)).toBeNull();
});
it("blocks executable and local schemes while classifying safe links", () => {
  for (const url of [
    "javascript:alert(1)",
    "data:text/html,x",
    "file:///tmp/a",
    " javaScript:alert(1)",
    "java\nscript:alert(1)",
    "\\evil.test/a",
  ])
    expect(markdownURL(url)).toBe("blocked");
  expect(markdownURL("https://example.org/a")).toBe("external");
  expect(markdownURL("//example.org/a")).toBe("external");
  expect(markdownURL("#标题")).toBe("anchor");
});
