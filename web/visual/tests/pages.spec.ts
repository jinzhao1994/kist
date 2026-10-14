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
  size: 123456,
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
    if (path === "/saved-searches") body = { searches: [] };
    else if (path === "/boxes" || path === "/boxes/search")
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
    else if (path === "/boxes/box-1/folders")
      body = {
        folders: [
          {
            id: "folder-1",
            parent_id: null,
            name: "",
            created_at: time,
            updated_at: time,
          },
        ],
      };
    else if (path === "/boxes/box-1") body = box;
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
        resumable_upload_version: 1,
        upload_chunk_bytes: 8388608,
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
    else if (
      /^\/files\/(?:file-[1-4]|visible-[0-9]+)\/derived\/[a-z0-9_]+\/content$/.test(
        path,
      )
    )
      return route.fulfill({ contentType: "image/svg+xml", body: landscape });
    else if (
      /^\/files\/(?:file-[1-4]|visible-[0-9]+)\/derived\/[a-z0-9_]+$/.test(path)
    ) {
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
  await expect(page).toHaveScreenshot(`${name}.png`, {
    fullPage: (await page.locator(".gallery").count()) === 0,
  });
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
    await expect(
      page.getByRole("region", { name: "画廊", exact: true }),
    ).toBeVisible();
    await expect(
      page
        .getByRole("region", { name: "画廊", exact: true })
        .getByText("旅途/山间清晨.jpg", { exact: true }),
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
        page
          .getByRole("region", { name: "画廊", exact: true })
          .locator(".media-placeholder [role=status]"),
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
    await page
      .getByRole("combobox", { name: "搜索或添加条件…" })
      .fill("文件类型");
    await page.getByRole("option", { name: "文件类型", exact: true }).click();
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
    await page.getByRole("button", { name: "更多操作", exact: true }).click();
    await page
      .getByRole("menuitem", { name: "移入回收站", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toContainText("全部 4 个文件");
    await capture(page, errors, info.title);
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
}

for (const scenario of [
  "light",
  "dark",
  "mobile",
  "options",
  "parent-options",
] as const) {
  test(`tag-path-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: scenario === "dark" ? "dark" : "light",
      mobile: scenario === "mobile",
    });
    await page.route("**/api/v1/tags", (route) =>
      route.fulfill({
        json: {
          tags: [
            {
              id: "education",
              name: "上学",
              parent_id: null,
              count: 0,
              children: [
                {
                  id: "school",
                  name: "高中",
                  parent_id: "education",
                  count: 0,
                  children: [
                    {
                      id: "reports",
                      name: "成绩单",
                      parent_id: "school",
                      count: 0,
                    },
                  ],
                },
              ],
            },
          ],
        },
      }),
    );
    await page.goto("/");
    await expect(page.locator(".box-card")).toHaveCount(2);
    await page.getByRole("button", { name: "管理标签", exact: true }).click();
    await page.getByRole("combobox", { name: "选择标签", exact: true }).click();
    await page
      .getByRole("option", { name: "上学 / 高中 / 成绩单", exact: true })
      .click();
    await expect(
      page.getByRole("combobox", { name: "选择标签", exact: true }),
    ).toHaveText("上学 / 高中 / 成绩单");
    await expect(
      page.getByRole("combobox", { name: "父标签", exact: true }),
    ).toHaveText("上学 / 高中");
    if (scenario === "options") {
      await page
        .getByRole("combobox", { name: "选择标签", exact: true })
        .click();
      await expect(
        page.getByRole("option", { name: "上学 / 高中 / 成绩单", exact: true }),
      ).toBeVisible();
    }
    if (scenario === "parent-options") {
      await page.setViewportSize({ width: 900, height: 520 });
      await page.getByRole("combobox", { name: "父标签", exact: true }).click();
      const search = page.getByRole("textbox", {
        name: "搜索标签",
        exact: true,
      });
      await expect(search).toBeFocused();
      await expect(
        page.getByRole("option", { name: "无父标签", exact: true }),
      ).toBeInViewport({ ratio: 1 });
      await capture(page, errors, info.title);
      await search.fill("高中");
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("Enter");
      await expect(
        page.getByRole("combobox", { name: "父标签", exact: true }),
      ).toHaveText("上学 / 高中");
      await expect(page.getByRole("dialog")).toBeVisible();
      return;
    }
    await capture(page, errors, info.title);
  });
}

for (const scenario of ["collapsed", "expanded", "mobile", "dark"] as const) {
  test(`tag-tree-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: scenario === "dark" ? "dark" : "light",
      mobile: scenario === "mobile",
    });
    await page.goto("/");
    const tree = page.getByRole("tree", { name: "标签" });
    await expect(tree.getByText("旅行", { exact: true })).toBeVisible();
    await expect(tree.getByText("山野", { exact: true })).toHaveCount(0);
    if (scenario !== "collapsed") {
      await tree.getByRole("button", { name: "展开旅行" }).click();
      await expect(tree.getByText("山野", { exact: true })).toBeVisible();
    }
    await capture(page, errors, info.title);
  });
}

for (const scenario of [
  "mixed",
  "boxes-only",
  "empty",
  "mobile",
  "dark",
] as const) {
  test(`tag-results-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: scenario === "dark" ? "dark" : "light",
      mobile: scenario === "mobile",
    });
    const matchingBoxes = scenario === "empty" ? [] : [box];
    const matchingFiles =
      scenario === "empty" || scenario === "boxes-only" ? [] : files;
    await page.route("**/api/v1/tags", (route) =>
      route.fulfill({
        json: {
          tags: [
            {
              ...tags[0],
              count: matchingBoxes.length + matchingFiles.length,
              box_count: matchingBoxes.length,
              file_count: matchingFiles.length,
            },
          ],
        },
      }),
    );
    await page.route("**/api/v1/boxes/search?*", (route) =>
      route.fulfill({ json: { boxes: matchingBoxes } }),
    );
    await page.route("**/api/v1/files?*", (route) =>
      route.fulfill({ json: { files: matchingFiles, cursor: "" } }),
    );
    await page.goto("/tags/tag-1");
    await expect(
      page.getByRole("heading", { name: "旅行", level: 1 }),
    ).toBeVisible();
    const boxes = page.getByRole("region", { name: "归档盒", exact: true });
    if (matchingBoxes.length)
      await expect(
        boxes.getByRole("heading", { name: box.name }),
      ).toBeVisible();
    else await expect(boxes.getByText("没有匹配的归档盒")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "文件", level: 2, exact: true }),
    ).toBeVisible();
    await expect(page.locator(".file-count")).toContainText(
      String(matchingFiles.length),
    );
    await capture(page, errors, info.title);
  });
}

for (const mobile of [false, true]) {
  test(`box-search-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    await page.route("**/api/v1/boxes/search?**", (route) => {
      const params = new URL(route.request().url()).searchParams;
      return route.fulfill({
        json: {
          boxes:
            params.get("text") === "年度报告"
              ? [{ ...box, name: "工作档案", description: "年度报告和合同" }]
              : [box],
        },
      });
    });
    await page.goto("/");
    await page
      .getByRole("combobox", { name: "搜索或添加条件…" })
      .fill("年度报告");
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "工作档案" })).toBeVisible();
    await expect(page.locator(".load-more")).toHaveText(
      "已全部显示 1 个归档盒",
    );
    await page.getByRole("combobox", { name: "搜索或添加条件…" }).click();
    await expect(
      page.getByRole("option", { name: "文件类型", exact: true }),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");
    await capture(page, errors, info.title);
  });
}

test("box-automatic-pagination-and-search-reset", async ({ page }, info) => {
  const errors = await prepare(page);
  const firstPage = Array.from({ length: 100 }, (_, i) => ({
    ...box,
    id: `box-${i}`,
    name: `Archive ${i}`,
  }));
  let nextRequests = 0;
  let release: () => void = () => {};
  const ready = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/boxes/search?**", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    if (params.get("text")) {
      expect(params.get("cursor") || "").toBe("");
      return route.fulfill({
        json: { boxes: [{ ...box, name: "Search result" }] },
      });
    }
    if (!params.get("cursor"))
      return route.fulfill({ json: { boxes: firstPage, cursor: "next" } });
    expect(params.get("cursor")).toBe("next");
    nextRequests++;
    await ready;
    return route.fulfill({
      json: { boxes: [{ ...box, id: "last-box", name: "Last archive" }] },
    });
  });
  await page.goto("/");
  await expect(page.locator(".box-card")).toHaveCount(100);
  await page.locator("main").evaluate((e) => {
    e.scrollTop = e.scrollHeight;
  });
  await expect(page.locator(".load-more")).toHaveText("正在加载…");
  expect(nextRequests).toBe(1);
  release();
  await expect(page.locator(".box-card")).toHaveCount(101);
  await expect(page.locator(".load-more")).toHaveText(
    "已全部显示 101 个归档盒",
  );
  await page
    .getByRole("combobox", { name: "搜索或添加条件…" })
    .fill("new search");
  await page.keyboard.press("Enter");
  await expect(page.locator(".box-card")).toHaveCount(1);
  await expect(
    page.getByRole("heading", { name: "Search result" }),
  ).toBeVisible();
  await expect(page.locator(".load-more")).toHaveText("已全部显示 1 个归档盒");
  expect(nextRequests).toBe(1);
  await capture(page, errors, info.title);
});

for (const kind of ["boxes", "files"] as const) {
  for (const [mode, label] of [
    ["large", "大图标"],
    ["medium", "中图标"],
    ["small", "小图标"],
    ["list", "列表"],
  ]) {
    test(`display-${kind}-${mode}`, async ({ page }, info) => {
      const errors = await prepare(page);
      await page.goto(kind === "boxes" ? "/" : "/files");
      await page.getByText(label, { exact: true }).click();
      const root = page.locator(
        kind === "boxes" ? ".box-grid" : ".file-scroll",
      );
      await expect(root).toHaveClass(new RegExp(`view-${mode}`));
      if (kind === "files") await grid(page);
      await page.reload();
      await expect(root).toHaveClass(new RegExp(`view-${mode}`));
      if (kind === "files") await grid(page);
      await capture(page, errors, info.title);
    });
  }
  test(`display-${kind}-list-mobile`, async ({ page }, info) => {
    const errors = await prepare(page, { mobile: true });
    await page.goto(kind === "boxes" ? "/" : "/files");
    await page.getByText("列表", { exact: true }).click();
    await expect(
      page.locator(kind === "boxes" ? ".box-grid" : ".file-scroll"),
    ).toHaveClass(/view-list/);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(390);
    await capture(page, errors, info.title);
  });
}

test("gallery-navigation-and-return", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.goto("/files");
  await grid(page);
  await page.getByRole("combobox", { name: "搜索或添加条件…" }).fill("旅行");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: /山间清晨.jpg/ }).click();
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(gallery).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get("filter")).toContain("旅行");
  await page.keyboard.press("ArrowRight");
  await expect(
    gallery.getByRole("heading", { name: "海边日落.jpg", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("ArrowLeft");
  await expect(
    gallery.getByRole("heading", { name: "山间清晨.jpg", exact: true }),
  ).toBeVisible();
  await expect(gallery.locator("aside")).toBeVisible();
  await gallery.getByRole("button", { name: "文件详情", exact: true }).click();
  await expect(gallery.locator("aside")).toHaveCount(0);
  await gallery.getByRole("button", { name: "下一个文件" }).click();
  await gallery.getByRole("button", { name: "下一个文件" }).click();
  await expect(
    gallery.getByRole("heading", { name: "旅行记录.pdf", exact: true }),
  ).toBeVisible();
  await expect(gallery.getByRole("link", { name: "下载原文件" })).toBeVisible();
  await capture(page, errors, info.title);
  await page.goBack();
  await expect(gallery).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "文件名: 旅行", exact: true }),
  ).toBeVisible();
  await page.goForward();
  await expect(
    gallery.getByRole("heading", { name: "旅行记录.pdf", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(gallery).toHaveCount(0);
});

test("gallery-pagination", async ({ page }, info) => {
  const errors = await prepare(page);
  let release: () => void = () => {};
  const ready = new Promise<void>((resolve) => {
    release = resolve;
  });
  let next = 0;
  await page.route("**/api/v1/files?**", async (route) => {
    if (!new URL(route.request().url()).searchParams.get("cursor"))
      return route.fulfill({
        json: { files: files.slice(0, 2), cursor: "next" },
      });
    next++;
    await ready;
    return route.fulfill({ json: { files: [files[2]] } });
  });
  await page.goto("/files?file=file-2");
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(gallery.locator(".gallery-loading")).toHaveText("正在加载…");
  await gallery.getByRole("button", { name: "下一个文件" }).click();
  release();
  await expect(
    gallery.getByRole("heading", { name: "旅行记录.pdf", exact: true }),
  ).toBeVisible();
  await expect(
    gallery.getByRole("button", { name: "下一个文件" }),
  ).toBeDisabled();
  await expect(gallery.locator("nav [role=status]")).toHaveText("3 / 3");
  expect(next).toBe(1);
  await capture(page, errors, info.title);
});

test("gallery-mobile-swipe", async ({ page }, info) => {
  const errors = await prepare(page, { mobile: true, mode: "dark" });
  await page.goto("/files?file=file-1");
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(
    gallery.getByRole("heading", { name: "山间清晨.jpg", exact: true }),
  ).toBeVisible();
  await expect(gallery.locator("aside")).toHaveCount(0);
  await gallery.locator(".gallery-media").evaluate((element) => {
    const touch = (x: number) =>
      new Touch({ identifier: 1, target: element, clientX: x, clientY: 250 });
    element.dispatchEvent(
      new TouchEvent("touchstart", { bubbles: true, touches: [touch(300)] }),
    );
    element.dispatchEvent(
      new TouchEvent("touchend", {
        bubbles: true,
        touches: [],
        changedTouches: [touch(80)],
      }),
    );
  });
  await expect(
    gallery.getByRole("heading", { name: "海边日落.jpg", exact: true }),
  ).toBeVisible();
  await expect
    .poll(() =>
      gallery
        .locator(".detail-image")
        .evaluateAll(
          (images) =>
            images.length === 1 &&
            (images[0] as HTMLImageElement).complete &&
            (images[0] as HTMLImageElement).naturalWidth > 0,
        ),
    )
    .toBe(true);
  expect(await gallery.evaluate((e) => e.getBoundingClientRect().height)).toBe(
    844,
  );
  await page.mouse.wheel(0, 1000);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
  await capture(page, errors, info.title);
});

test("gallery-restores-scroll", async ({ page }, info) => {
  const errors = await prepare(page, { manyFiles: true });
  await page.route("**/api/v1/files/scroll-*", (route) => {
    const id = new URL(route.request().url()).pathname.split("/").at(-1)!;
    return route.fulfill({
      json: {
        file: { ...files[2], id, name: `${id}.pdf` },
        tags: [],
        metadata: [],
      },
    });
  });
  await page.goto("/files");
  const main = page.locator("main");
  await expect(page.locator(".file-count")).toContainText("100");
  await main.evaluate((e) => {
    e.scrollTop = 800;
  });
  const scroll = await main.evaluate((e) => e.scrollTop);
  await page
    .locator(".file-card")
    .filter({ hasText: "document-12.pdf" })
    .click();
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(gallery).toBeVisible();
  await gallery.getByRole("button", { name: "下一个文件" }).click();
  await gallery.getByRole("button", { name: "返回列表" }).click();
  await expect(gallery).toHaveCount(0);
  expect(await main.evaluate((e) => e.scrollTop)).toBe(scroll);
  await capture(page, errors, info.title);
});

for (const reduced of [false, true]) {
  test(`gallery-motion-${reduced ? "reduced" : "enabled"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page);
    await page.emulateMedia({
      reducedMotion: reduced ? "reduce" : "no-preference",
    });
    await page.addInitScript(() => {
      const animate = Element.prototype.animate;
      (window as unknown as { flights: number }).flights = 0;
      Element.prototype.animate = function (...args) {
        if (this.classList.contains("gallery-flight"))
          (window as unknown as { flights: number }).flights++;
        return animate.apply(this, args);
      };
    });
    await page.goto("/files");
    await grid(page);
    const thumbnail = await page.locator(".file-image").first().elementHandle();
    expect(thumbnail).not.toBeNull();
    await page.getByRole("button", { name: /山间清晨.jpg/ }).click();
    const gallery = page.getByRole("region", { name: "画廊", exact: true });
    await expect(gallery).toBeVisible();
    expect(await thumbnail!.evaluate((node) => node.isConnected)).toBe(true);
    await expect(gallery.locator("aside")).toBeVisible();
    await expect(page.locator(".gallery-flight")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => (window as unknown as { flights: number }).flights,
      ),
    ).toBe(reduced ? 0 : 1);
    await gallery.getByRole("button", { name: "返回列表" }).click();
    await expect(gallery).toHaveCount(0);
    expect(
      await thumbnail!.evaluate(
        (node) => node.isConnected && (node as HTMLImageElement).complete,
      ),
    ).toBe(true);
    await expect(page.locator(".gallery-flight")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => (window as unknown as { flights: number }).flights,
      ),
    ).toBe(reduced ? 0 : 2);
    await page.getByRole("button", { name: /山间清晨.jpg/ }).click();
    await expect(gallery).toBeVisible();
    await capture(page, errors, info.title);
  });
}

test("visible-thumbnails-beyond-eight", async ({ page }, info) => {
  const errors = await prepare(page);
  const images = Array.from({ length: 100 }, (_, index) => ({
    ...files[0],
    id: `visible-${index}`,
    name: `image-${index}.jpg`,
  }));
  await page.route("**/api/v1/files?**", (route) =>
    route.fulfill({ json: { files: images } }),
  );
  await page.goto("/files");
  await page.getByText("小图标", { exact: true }).click();
  const main = page.locator("main");
  for (const top of [0, 650, 0]) {
    await main.evaluate((element, value) => {
      element.scrollTop = value;
    }, top);
    await expect
      .poll(() =>
        page.locator(".file-card").evaluateAll((cards) => {
          const main = document.querySelector("main")!.getBoundingClientRect();
          const visible = cards.filter((card) => {
            const rect = card.getBoundingClientRect();
            return rect.bottom > main.top && rect.top < main.bottom;
          });
          return (
            visible.length > 8 &&
            visible.every((card) => {
              const image = card.querySelector("img");
              return image?.complete && image.naturalWidth > 0;
            })
          );
        }),
      )
      .toBe(true);
    expect(await page.locator(".file-card").count()).toBeLessThan(
      images.length,
    );
  }
  await capture(page, errors, info.title);
});

for (const mobile of [false, true]) {
  test(`folder-navigation-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    const directories = [
      {
        id: "root",
        parent_id: null,
        name: "",
        created_at: time,
        updated_at: time,
      },
      {
        id: "photos",
        parent_id: "root",
        name: "照片",
        created_at: time,
        updated_at: time,
      },
      {
        id: "day1",
        parent_id: "photos",
        name: "第一天",
        created_at: time,
        updated_at: time,
      },
    ];
    await page.route("**/api/v1/boxes/box-1/folders", (route) =>
      route.fulfill({ json: { folders: directories } }),
    );
    await page.route("**/api/v1/files?**", (route) =>
      route.fulfill({ json: { files: [] } }),
    );
    await page.goto("/boxes/box-1");
    await page.getByRole("button", { name: "照片", exact: true }).click();
    await expect(page).toHaveURL(/folder=photos/);
    await expect(
      page.getByRole("button", { name: "第一天", exact: true }),
    ).toBeVisible();
    await expect(page.getByLabel("目录路径")).toContainText("照片");
    await capture(page, errors, info.title);
    await page.reload();
    await expect(
      page.getByRole("button", { name: "第一天", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "根目录", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "照片", exact: true }),
    ).toBeVisible();
  });
}

for (const scenario of ["sidebar", "dialog", "mobile"] as const) {
  test(`saved-search-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, { mobile: scenario === "mobile" });
    await page.route("**/api/v1/saved-searches", (route) =>
      route.fulfill({
        json: {
          searches: [
            {
              id: "saved-1",
              name: "旅行照片",
              definition: {
                version: 1,
                entity: "files",
                filters: {
                  sort: "captured",
                  asc: false,
                  kind: ["image"],
                  tag_all: ["tag-1"],
                },
              },
            },
          ],
        },
      }),
    );
    await page.goto("/files");
    await expect(
      page.getByRole("button", { name: "旅行照片", exact: true }),
    ).toBeVisible();
    if (scenario === "dialog") {
      await page
        .getByRole("button", { name: "收藏当前搜索", exact: true })
        .click();
      await page
        .getByRole("textbox", { name: "名称", exact: true })
        .fill("待整理的照片");
    }
    await capture(page, errors, info.title);
  });
}

for (const mobile of [false, true]) {
  test(`resumable-upload-paused-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    const id = "a".repeat(64);
    const files = [
      {
        path: "旅行视频.mp4",
        last_modified: 1000,
        size: 20 * 1024 * 1024,
        sha256: "b".repeat(64),
      },
    ];
    await page.addInitScript(
      ({ id, files }) => {
        localStorage.setItem(
          "kist.resumable-upload.v1:box-1:folder-1",
          JSON.stringify({
            version: 1,
            key: "visual-upload-key",
            box: "box-1",
            folder: "folder-1",
            id,
            files,
          }),
        );
      },
      { id, files },
    );
    await page.route(`**/api/v1/upload-sessions/${id}`, (route) =>
      route.fulfill({
        json: {
          session: {
            version: 1,
            id,
            manifest: {
              box: "box-1",
              folder: "folder-1",
              files,
              target: "c".repeat(64),
            },
            offsets: [8 * 1024 * 1024],
            expires: "2030-01-01T00:00:00Z",
          },
        },
      }),
    );
    await page.goto("/boxes/box-1");
    await expect(page.getByText("已暂停，服务端已确认 40%。")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "重新选择原文件" }),
    ).toBeVisible();
    await grid(page);
    await capture(page, errors, info.title);
  });
}
