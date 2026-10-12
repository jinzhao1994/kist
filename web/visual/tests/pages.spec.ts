import { test, expect, type Page } from "@playwright/test";
import { colors, type Appearance } from "../../src/theme";
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
  primaryColor?: Appearance["primaryColor"];
  mode?: Appearance["mode"];
  state?: MediaJob["state"];
  language?: "zh" | "en";
  empty?: boolean;
  mobile?: boolean;
  manyFiles?: boolean;
};
async function prepare(page: Page, options: Options = {}) {
  if (options.mobile) await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(
    ({ primaryColor, mode, language }) => {
      localStorage.setItem(
        "kist.appearance.v2",
        JSON.stringify({ primaryColor, mode }),
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
      primaryColor: options.primaryColor || "blue",
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
    else if (path === "/ops") {
      const deleted = {
        id: 12,
        action: "delete_files",
        ts: time,
        origin: "user",
        reverts_id: 10,
        undone_by: null,
        undoable: true,
      };
      body = {
        ops:
          new URL(route.request().url()).searchParams.get("trash") === "true"
            ? [deleted]
            : [
                deleted,
                {
                  ...deleted,
                  id: 10,
                  action: "import_files",
                  reverts_id: null,
                  undone_by: 12,
                },
              ],
      };
    } else if (path === "/ops/12")
      body = {
        id: 12,
        action: "delete_files",
        ts: time,
        origin: "user",
        reverts_id: 10,
        undone_by: null,
        undoable: true,
        boxes: [box.id],
        files: [files[0].id],
        payload: { files: [{ file_id: files[0].id, name: files[0].name }] },
      };
    else if (path === "/history/11/boxes/box-1")
      body = { op: 11, box, files, folders: [], box_tags: ["tag-1"], tags };
    else if (path === "/history/11/files/file-1")
      body = {
        op: 11,
        file: {
          file: files[0],
          tags: ["tag-2"],
          metadata: [
            { key: "file:size", value: "204800", type: "int", source: "file" },
          ],
          extracted: ["file"],
        },
        tags,
      };
    else if (path === "/history/12/files/file-1")
      return route.fulfill({
        status: 404,
        json: {
          code: "file_not_found",
          class: "not_found",
          message: "Not present",
        },
      });
    else if (path === "/tags") body = { tags };
    else if (path === "/info")
      body = {
        version: "visual",
        timezone: "UTC",
        projection_dirty: false,
        max_upload_bytes: 1073741824,
        max_upload_files: 100,
      };
    else if (path === "/boxes/box-1/deletion-preview")
      body = { name: box.name, file_count: 4, revision: 42 };
    else if (path === "/boxes/box-1/tags") body = { tags: ["tag-1"] };
    else if (path === "/files")
      body = {
        files: options.empty
          ? []
          : options.manyFiles
            ? Array.from({ length: 100 }, (_, index) => ({
                ...files[2],
                id: `scroll-${index}`,
                name: `document-${index}.pdf`,
              }))
            : files,
      };
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
for (const primaryColor of colors) {
  for (const mode of ["light", "dark"] as const) {
    test(`grid-${primaryColor}-${mode}`, async ({ page }, info) => {
      const errors = await prepare(page, { primaryColor, mode });
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
    if (scenario === "mobile") {
      await expect(page.locator(".file-count")).toContainText("4");
      await page.evaluate(() => document.fonts.ready);
      // Column measurement changes the document height after the first render.
      // Wait for the requested scroll position, not just the file-count text.
      await expect
        .poll(() =>
          page.locator(".file-scroll").evaluate((element) => {
            window.scrollTo(
              0,
              element.getBoundingClientRect().top + window.scrollY,
            );
            return Math.abs(element.getBoundingClientRect().top);
          }),
        )
        .toBeLessThan(1);
      await grid(page);
      expect(
        await page.locator("main").evaluate((element) => element.scrollTop),
      ).toBe(0);
      expect(
        await page
          .locator(".file-scroll")
          .evaluate((element) => element.scrollTop),
      ).toBe(0);
      expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
    } else if (scenario === "box-files") await grid(page);
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

for (const scenario of [
  "history",
  "trash",
  "operation",
  "historical-box",
  "version-comparison",
] as const) {
  test(scenario, async ({ page }, info) => {
    const errors = await prepare(page);
    await page.goto(scenario === "trash" ? "/trash" : "/history");
    await expect(page.locator(".history-entry").first()).toBeVisible();
    if (
      ["operation", "historical-box", "version-comparison"].includes(scenario)
    ) {
      await page.locator(".history-entry").first().click();
      await expect(
        page.getByRole("dialog").getByRole("heading", { name: "删除文件" }),
      ).toBeVisible();
    }
    if (scenario === "historical-box") {
      await page.getByRole("button", { name: "查看删除前的状态" }).click();
      await expect(page.getByText("操作 #11 之后的只读历史状态")).toBeVisible();
      await expect(
        page
          .getByRole("dialog")
          .getByText("旅途/山间清晨.jpg", { exact: true }),
      ).toBeVisible();
    }
    if (scenario === "version-comparison") {
      await page.getByRole("button", { name: "比较版本", exact: true }).click();
      await expect(page.getByText("此时尚不存在或已删除")).toBeVisible();
    }
    await capture(page, errors, info.title);
  });
}

for (const mode of ["light", "dark"] as const) {
  test(`filter-menu-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, { mode });
    await page.goto("/files");
    await grid(page);
    await page.getByRole("button", { name: "添加条件", exact: true }).click();
    await page.getByRole("button", { name: "文件类型", exact: true }).click();
    await page.getByRole("combobox", { name: "文件类型", exact: true }).click();
    await page.getByRole("option", { name: "图片", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "文件类型: 图片", exact: true }),
    ).toBeVisible();
    await capture(page, errors, info.title);
  });
}

test("independent-scroll", async ({ page }, info) => {
  await page.setViewportSize({ width: 1280, height: 400 });
  const errors = await prepare(page, { manyFiles: true });
  await page.goto("/files");
  await expect(page.locator(".file-count")).toContainText("100");
  const sidebar = page.locator(".sidebar");
  const main = page.locator("main");
  await sidebar.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  expect(
    await sidebar.evaluate((element) => element.scrollTop),
  ).toBeGreaterThan(0);
  expect(await main.evaluate((element) => element.scrollTop)).toBe(0);
  const sidebarTop = await sidebar.evaluate((element) => element.scrollTop);
  await main.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(
    page.getByRole("button", { name: /document-99.pdf/ }),
  ).toBeVisible();
  expect(await sidebar.evaluate((element) => element.scrollTop)).toBe(
    sidebarTop,
  );
  expect(await page.locator(".file-card").count()).toBeLessThan(30);
  expect(
    await page.locator(".file-scroll").evaluate((element) => element.scrollTop),
  ).toBe(0);
  expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(
    page.viewportSize()!.height,
  );
  await capture(page, errors, info.title);
});

test("automatic-pagination", async ({ page }, info) => {
  const errors = await prepare(page);
  const firstPage = Array.from({ length: 100 }, (_, index) => ({
    ...files[2],
    id: `auto-${index}`,
    name: `document-${index}.pdf`,
  }));
  let nextRequests = 0;
  let release: () => void = () => {};
  const responseReady = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/files?**", async (route) => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    if (!cursor)
      return route.fulfill({ json: { files: firstPage, cursor: "next" } });
    expect(cursor).toBe("next");
    nextRequests++;
    await responseReady;
    await route.fulfill({
      json: { files: [{ ...files[2], id: "last", name: "last.pdf" }] },
    });
  });
  await page.goto("/files");
  await expect(page.locator(".file-count")).toContainText("100");
  await page.locator("main").evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(page.locator(".load-more")).toHaveText("正在加载…");
  expect(nextRequests).toBe(1);
  release();
  await expect(page.locator(".load-more")).toHaveText("已全部显示 101 个文件");
  await page.locator("main").evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(page.getByRole("button", { name: /last.pdf/ })).toBeVisible();
  expect(nextRequests).toBe(1);
  await page.evaluate(() => document.fonts.ready);
  // The appended virtual row can settle after the total-count text updates.
  // Pin the completed footer inside the viewport before capturing.
  await expect
    .poll(() =>
      page.locator(".load-more").evaluate((footer) => {
        const main = footer.closest("main")!;
        main.scrollTop = main.scrollHeight;
        return Math.max(
          0,
          footer.getBoundingClientRect().bottom -
            main.getBoundingClientRect().bottom,
        );
      }),
    )
    .toBe(0);
  await expect(page.locator(".load-more")).toBeInViewport({ ratio: 1 });
  await capture(page, errors, info.title);
});

for (const mobile of [false, true]) {
  test(`box-trash-confirm-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    await page.goto("/boxes/box-1");
    await expect(page.locator(".file-count")).toContainText("4");
    await page.getByRole("button", { name: "移入回收站", exact: true }).click();
    await expect(page.getByRole("dialog")).toContainText("全部 4 个文件");
    await capture(page, errors, info.title);
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
}
