import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "@playwright/test";
import environment from "./visual/environment.json" with { type: "json" };

if (
  !process.argv.includes("--list") &&
  (!process.env.CI || process.platform !== "linux" || process.arch !== "x64")
) {
  throw new Error(
    "Formal visual captures run only in the pinned Linux amd64 CI image",
  );
}

const side = process.env.VISUAL_SIDE || "candidate";
if (side !== "candidate" && side !== "baseline")
  throw new Error("Invalid capture side");
const web = path.dirname(fileURLToPath(import.meta.url));
const source =
  side === "baseline"
    ? path.resolve(web, "../.visual-baseline")
    : path.resolve(web, "..");
const results = path.join(web, "visual-results");

export default defineConfig({
  testDir: path.join(source, "web/visual/tests"),
  forbidOnly: true,
  retries: 0,
  workers: 1,
  timeout: 30000,
  outputDir: path.join(results, "test-results", side),
  // Both revisions use this driver and environment, with their own test sources.
  snapshotPathTemplate: path.join(results, side, "screenshots/{arg}{ext}"),
  updateSnapshots: "all",
  reporter: [
    ["list"],
    ["json", { outputFile: path.join(results, side, "capture.json") }],
  ],
  use: {
    browserName: "chromium",
    baseURL: "http://127.0.0.1:8473",
    viewport: environment.viewport,
    locale: environment.locale,
    timezoneId: environment.timezoneId,
    deviceScaleFactor: environment.deviceScaleFactor,
    reducedMotion: "reduce",
    colorScheme: "light",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  expect: {
    toHaveScreenshot: { animations: "disabled", caret: "hide", scale: "css" },
  },
  webServer: {
    cwd: source,
    command: `./bin/kist init /tmp/kist-visual-${side}-library --yes && ./bin/kist --library /tmp/kist-visual-${side}-library serve`,
    url: "http://127.0.0.1:8473",
    reuseExistingServer: false,
  },
});
