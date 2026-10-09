import { defineConfig } from "@playwright/test";
import environment from "./visual/environment.json";

if (!process.env.CI || process.platform !== "linux" || process.arch !== "x64") {
  throw new Error(
    "Formal visual captures run only in the pinned Linux amd64 CI image",
  );
}

export default defineConfig({
  testDir: "./visual/tests",
  forbidOnly: true,
  retries: 0,
  workers: 1,
  timeout: 30000,
  outputDir: "./visual-results/test-results",
  // Capture into a disposable candidate directory; comparison never overwrites Git baselines.
  snapshotPathTemplate:
    "{testDir}/../../visual-results/candidate/screenshots/{arg}{ext}",
  updateSnapshots: "all",
  reporter: [["list"], ["json", { outputFile: "visual-results/capture.json" }]],
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
    command:
      "../bin/kist init /tmp/kist-visual-library --yes && ../bin/kist --library /tmp/kist-visual-library serve",
    url: "http://127.0.0.1:8473",
    reuseExistingServer: false,
  },
});
