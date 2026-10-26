import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const side = process.argv[2];
if (!["baseline", "candidate"].includes(side))
  throw new Error("Invalid capture side");
if (!process.env.CI || process.platform !== "linux" || process.arch !== "x64")
  throw new Error("Capture requires the pinned Linux amd64 CI image");
if (side === "baseline") {
  // This approved revision raced its initial catalog invalidation against page
  // prefetch. Stabilize only that fixture; keep the old app and screenshot.
  const baseline = path.resolve(web, "../.visual-baseline");
  const revision = execFileSync("git", ["rev-parse", "HEAD"], {
    cwd: baseline,
    encoding: "utf8",
  }).trim();
  if (revision === "293bb4da02688a2a092eaa3af5ce5af71f5b47ea") {
    const file = path.join(baseline, "web/visual/tests/pages.spec.ts");
    const source = fs.readFileSync(file, "utf8");
    const start = source.indexOf('test("gallery-pagination",');
    const end = source.indexOf('test("gallery-mobile-swipe",', start);
    const original = source.slice(start, end);
    if (
      createHash("sha256").update(original).digest("hex") !==
      "9c904102d84a21adead1644711928fb6980b72b217ea099026fd173f7f85275f"
    )
      throw new Error(
        "Unexpected approved pagination fixture; compatibility fix refused",
      );
    const fixed = original
      .replace(
        "  const errors = await prepare(page);",
        `  const errors = await prepare(page);
  let releaseRevision: () => void = () => {};
  const revisionReady = new Promise<void>((resolve) => { releaseRevision = resolve; });
  await page.route("**/api/v1/catalog-revision", async (route) => {
    await revisionReady;
    return route.fulfill({ json: { revision: "fixture-catalog" } });
  });`,
      )
      .replace(
        '  await gallery.getByRole("button", { name: "下一个文件" }).click();',
        `  await expect.poll(() => next).toBe(1);
  releaseRevision();
  await expect.poll(() => next).toBe(2);
  await gallery.getByRole("button", { name: "下一个文件" }).click();`,
      )
      .replace("  expect(next).toBe(1);", "  expect(next).toBe(2);");
    fs.writeFileSync(file, source.slice(0, start) + fixed + source.slice(end));
  }
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
