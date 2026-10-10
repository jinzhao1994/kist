import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const side = process.argv[2];
if (!["baseline", "candidate"].includes(side))
  throw new Error("Invalid capture side");
if (!process.env.CI || process.platform !== "linux" || process.arch !== "x64")
  throw new Error("Capture requires the pinned Linux amd64 CI image");
if (side === "baseline") {
  // Use the current driver for old tests, while retaining their source imports.
  // Only the disposable checkout's installed package is replaced.
  const target = path.resolve(
    web,
    "../.visual-baseline/web/node_modules/@playwright/test",
  );
  fs.rmSync(target, { recursive: true, force: true });
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.symlinkSync(
    path.join(web, "node_modules/@playwright/test"),
    target,
    "dir",
  );
}
fs.mkdirSync(path.join(web, "visual-results", side), { recursive: true });
const result = spawnSync(
  process.execPath,
  [
    path.join(web, "node_modules/@playwright/test/cli.js"),
    "test",
    "--config",
    path.join(web, "playwright.config.ts"),
  ],
  {
    cwd: web,
    env: { ...process.env, VISUAL_SIDE: side },
    stdio: "inherit",
  },
);
if (result.error) throw result.error;
process.exitCode = result.status ?? 1;
