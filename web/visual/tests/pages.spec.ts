import { test, expect, type Page } from "@playwright/test";
import { themes, type Appearance } from "../../src/theme";
import type { ArchiveFile, Box, MediaJob, Tag } from "../../src/api";

const time = "2026-01-02T10:30:00Z";
const box: Box = {
  id: "box-1",
  name: "山海拾光",
  description: "一次旅行，一些值得留下的瞬间。",
  start_at: "2026-01-01",
  end_at: "2026-01-03",
  created_at: time,
  updated_at: time,
};
const tags: Tag[] = [
  {
    id: "tag-1",
    name: "旅行",
    parent_id: null,
    count: 3,
    children: [{ id: "tag-2", name: "山野", parent_id: "tag-1", count: 2 }],
  },
  { id: "tag-3", name: "证件与档案", parent_id: null, count: 1 },
];
const files: ArchiveFile[] = [
  ["山间清晨.jpg", "image", "image/jpeg"],
  ["海边日落.jpg", "image", "image/jpeg"],
  ["旅行记录.pdf", "document", "application/pdf"],
  ["归途.mp4", "video", "video/mp4"],
].map(([name, kind, media_type], i) => ({
  id: `file-${i + 1}`,
  box_id: box.id,
  folder_id: "folder-1",
  name,
  path: `旅途/${name}`,
  kind,
  media_type,
  blob_key: `${i + 1}`.repeat(64),
  created_at: time,
  updated_at: time,
  captured_at: time,
}));
const landscape = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600" viewBox="0 0 800 600"><rect width="800" height="600" fill="#d7e4df"/><circle cx="620" cy="130" r="58" fill="#f5d99a"/><path d="M0 460L260 120 530 460Z" fill="#718c7c"/><path d="M250 520L560 220 800 460V600H0V460Z" fill="#a4b9a3"/><path d="M0 520Q400 450 800 520V600H0Z" fill="#577c83"/></svg>`;

type Options = {
  theme?: Appearance["theme"];
  mode?: Appearance["mode"];
  state?: MediaJob["state"];
  language?: "zh" | "en";
  empty?: boolean;
  mobile?: boolean;
};
async function prepare(page: Page, options: Options = {}) {
  if (options.mobile) await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(
    ({ theme, mode, language }) => {
      localStorage.setItem(
        "kist.appearance.v1",
        JSON.stringify({ theme, mode }),
      );
      localStorage.setItem(
        "kist.display.v1",
        JSON.stringify({
          language,
          formatLocale: language === "zh" ? "zh-CN" : "en-US",
          timezone: "UTC",
          direction: "ltr",
        }),
      );
    },
    {
      theme: options.theme || "archive",
      mode: options.mode || "light",
      language: options.language || "zh",
    },
  );
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname.slice(
      "/api/v1".length,
    );
    if (route.request().method() !== "GET") {
      errors.push(`Unexpected write: ${path}`);
      return route.abort();
    }
    let body: unknown;
    if (path === "/boxes")
      body = {
        boxes: options.empty
          ? []
          : [
              box,
              {
                ...box,
                id: "box-2",
                name: "生活备忘",
                description: "证件、合同与年度报告",
                start_at: "",
                end_at: "",
              },
            ],
      };
    else if (path === "/tags") body = { tags };
    else if (path === "/info")
      body = {
        version: "visual",
        timezone: "UTC",
        projection_dirty: false,
        max_upload_bytes: 1073741824,
        max_upload_files: 100,
      };
    else if (path === "/boxes/box-1/tags") body = { tags: ["tag-1"] };
    else if (path === "/files") body = { files: options.empty ? [] : files };
    else if (/^\/files\/file-[1-4]$/.test(path))
      body = {
        file: files[Number(path.at(-1)) - 1],
        tags: ["tag-2"],
        extracted: ["file", "media"],
        metadata: [
          { key: "file:size", type: "int", value: "204800", source: "file" },
          { key: "media:width", type: "int", value: "800", source: "media" },
        ],
      };
    else if (/^\/files\/file-[1-4]\/derived\/[a-z0-9_]+\/content$/.test(path))
      return route.fulfill({ contentType: "image/svg+xml", body: landscape });
    else if (/^\/files\/file-[1-4]\/derived\/[a-z0-9_]+$/.test(path)) {
      const state = options.state || "ready";
      body = {
        state,
        generation: "fixture-generation",
        blob_key: files[0].blob_key,
        recipe: path.split("/").at(-1),
        updated_at: time,
        reason: state === "blocked" ? "dependency_missing" : undefined,
        result: {
          bytes: 1000,
          width: 800,
          height: 600,
          generation_ms: 10,
          engine: "fixture",
        },
      };
    } else {
      errors.push(`Unexpected API: ${path}`);
      return route.abort();
    }
    await route.fulfill({ json: body });
  });
  return errors;
}
async function capture(page: Page, errors: string[], name: string) {
  await page.evaluate(() => document.fonts.ready);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true });
  expect(errors).toEqual([]);
}
async function grid(page: Page, expectedImages = 3) {
  await expect(page.locator(".file-count")).toContainText("4");
  await expect(page.locator(".file-image")).toHaveCount(expectedImages);
  await expect
    .poll(() =>
      page
        .locator(".file-image")
        .evaluateAll((images) =>
          images.every(
            (image) =>
              (image as HTMLImageElement).complete &&
              (image as HTMLImageElement).naturalWidth > 0,
          ),
        ),
    )
    .toBe(true);
}
for (const theme of Object.keys(themes) as Appearance["theme"][]) {
  for (const mode of ["light", "dark"] as const) {
    test(`grid-${theme}-${mode}`, async ({ page }, info) => {
      const errors = await prepare(page, { theme, mode });
      await page.goto("/files");
      await grid(page);
      await capture(page, errors, info.title);
    });
  }
}
for (const scenario of [
  "boxes",
  "box-files",
  "settings",
  "tags",
  "empty",
  "english",
  "mobile",
] as const) {
  test(scenario, async ({ page }, info) => {
    const errors = await prepare(page, {
      empty: scenario === "empty",
      language: scenario === "english" ? "en" : "zh",
      mobile: scenario === "mobile",
    });
    await page.goto(
      scenario === "box-files"
        ? "/boxes/box-1"
        : scenario === "mobile"
          ? "/files"
          : "/",
    );
    if (scenario === "box-files" || scenario === "mobile")
      await grid(page, scenario === "mobile" ? 2 : 3);
    else if (scenario === "empty")
      await expect(page.locator(".empty-state")).toBeVisible();
    else await expect(page.locator(".box-card")).toHaveCount(2);
    if (scenario === "settings" || scenario === "tags") {
      await page
        .getByRole("button", {
          name: scenario === "settings" ? "显示设置" : "管理标签",
          exact: true,
        })
        .click();
      await expect(page.getByRole("dialog")).toBeVisible();
    }
    await capture(page, errors, info.title);
  });
}
for (const state of [
  "ready",
  "queued",
  "running",
  "blocked",
  "failed",
  "cancelled",
] as const) {
  test(`detail-${state}`, async ({ page }, info) => {
    const errors = await prepare(page, { state });
    await page.goto("/files?file=file-1");
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(
      page.getByRole("dialog").getByText("旅途/山间清晨.jpg", { exact: true }),
    ).toBeVisible();
    if (state === "ready")
      await expect
        .poll(() =>
          page
            .locator(".detail-image")
            .evaluateAll(
              (images) =>
                images.length === 1 &&
                (images[0] as HTMLImageElement).naturalWidth > 0,
            ),
        )
        .toBe(true);
    else
      await expect(
        page.getByRole("dialog").locator(".media-placeholder [role=status]"),
      ).toBeVisible();
    await capture(page, errors, info.title);
  });
}
