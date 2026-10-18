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
  ["竖版证书.png", "image", "image/png"],
  ["旅行记录.pdf", "document", "application/pdf"],
  ["归途.mov", "video", "video/quicktime"],
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
  display_tags: i === 0 ? [{ id: "tag-1", name: "旅行" }] : undefined,
}));
const landscape = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600" viewBox="0 0 800 600"><rect width="800" height="600" fill="#d7e4df"/><circle cx="620" cy="130" r="58" fill="#f5d99a"/><path d="M0 460L260 120 530 460Z" fill="#718c7c"/><path d="M250 520L560 220 800 460V600H0V460Z" fill="#a4b9a3"/><path d="M0 520Q400 450 800 520V600H0Z" fill="#577c83"/></svg>`;

const portrait = `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="800" viewBox="0 0 400 800"><rect width="400" height="800" fill="#194886"/><rect x="12" y="12" width="376" height="776" fill="none" stroke="#f5d99a" stroke-width="12"/><circle cx="200" cy="680" r="60" fill="#f5d99a"/><path d="M80 100H320M80 200H320M80 300H320" stroke="white" stroke-width="16"/></svg>`;
const documentPage = `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="800" viewBox="0 0 600 800"><rect width="600" height="800" fill="#faf6ed"/><rect x="10" y="10" width="580" height="780" fill="none" stroke="#718c7c" stroke-width="8"/><path d="M60 100H540M60 200H540M60 300H540M60 400H400" stroke="#577c83" stroke-width="16"/><rect x="60" y="600" width="150" height="100" fill="#a4b9a3"/></svg>`;

const panorama = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="300" viewBox="0 0 800 300"><rect width="800" height="300" fill="#d7e4df"/><rect x="8" y="8" width="784" height="284" fill="none" stroke="#577c83" stroke-width="12"/><circle cx="650" cy="80" r="40" fill="#f5d99a"/><path d="M20 280L240 40 480 280Z" fill="#718c7c"/></svg>`;

const modelPage = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512"><rect width="512" height="512" fill="#f3f4f6"/><path d="M256 72L432 168 256 272 80 168Z" fill="#6796b5"/><path d="M80 168L256 272V440L80 336Z" fill="#355e7b"/><path d="M256 272L432 168V336L256 440Z" fill="#4f7c9c"/></svg>`;

type Options = {
  primaryColor?: Appearance["primaryColor"];
  mode?: Appearance["mode"];
  state?: MediaJob["state"];
  language?: "zh" | "en";
  empty?: boolean;
  mobile?: boolean;
  manyFiles?: boolean;
  stl?: boolean;
  gpx?: boolean;
  mapFailure?: boolean;
  dateline?: boolean;
};
async function prepare(page: Page, options: Options = {}) {
  const mediaFiles =
    options.stl || options.gpx
      ? files.map((file, i) =>
          i === 2
            ? {
                ...file,
                name: options.gpx ? "山间路线.GPX" : "模型.STL",
                media_type: "application/octet-stream",
                kind: "other",
              }
            : file,
        )
      : files;
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
  // Native PDF chrome is browser-owned; capture our container with a stable
  // frame fixture. Real PDF rendering is verified separately in desktop Chrome.
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "pdfViewerEnabled", {
      value: true,
      configurable: true,
    });
  });
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
    if (path === "/files/file-3/content")
      return route.fulfill({
        contentType: "text/html",
        body: '<!doctype html><html><body style="margin:0;background:#282828;color:white;font:16px sans-serif;display:grid;place-items:center;height:100vh"><p>Browser PDF reader · synthetic fixture</p></body></html>',
      });
    if (options.gpx && path === "/map-provider")
      body = {
        style_url: "/api/v1/fixture-map-style",
        terrain_tiles: "",
        terrain_attribution: "",
        resource_origins: [],
      };
    else if (options.gpx && path === "/fixture-map-style") {
      if (options.mapFailure)
        return route.fulfill({ status: 503, body: "Map unavailable" });
      body = {
        version: 8,
        sources: {
          roads: {
            type: "geojson",
            attribution: '<a href="https://example.org/maps">Synthetic map</a>',
            data: {
              type: "FeatureCollection",
              features: [
                {
                  type: "Feature",
                  properties: {},
                  geometry: {
                    type: "LineString",
                    coordinates: [
                      [6.86, 45.92],
                      [6.87, 45.93],
                      [6.88, 45.94],
                    ],
                  },
                },
              ],
            },
          },
        },
        layers: [
          {
            id: "background",
            type: "background",
            paint: { "background-color": "#e5eadf" },
          },
          {
            id: "road-edge",
            type: "line",
            source: "roads",
            paint: { "line-color": "#d5c9a9", "line-width": 10 },
          },
          {
            id: "road",
            type: "line",
            source: "roads",
            paint: { "line-color": "#fff8dd", "line-width": 6 },
          },
        ],
      };
    } else if (
      options.gpx &&
      path.includes("/file-3/derived/gpx_map_v1/content")
    )
      body = {
        type: "FeatureCollection",
        features: [
          {
            type: "Feature",
            properties: {},
            geometry: {
              type: "LineString",
              coordinates: options.dateline
                ? [
                    [179.6, 10],
                    [179.9, 10.2],
                    [-179.7, 10.1],
                    [-179.5, 10.3],
                  ]
                : [
                    [6.8694, 45.9237],
                    [6.86, 45.9282],
                    [6.8552, 45.9324],
                    [6.8657, 45.9358],
                    [6.8751, 45.9295],
                    [6.872, 45.925],
                  ],
            },
          },
        ],
      };
    else if (path === "/saved-searches") body = { searches: [] };
    else if (path === "/boxes" || path === "/boxes/search")
      body = {
        boxes: options.empty
          ? []
          : [
              {
                ...box,
                tags: [
                  { id: "tag-1", name: "旅行" },
                  { id: "tag-2", name: "山野" },
                ],
              },
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
    } else if (path === "/ops/12/summary" || path === "/ops/10/summary")
      body = {
        name: mediaFiles[0].name,
        count: 1,
        current_names: false,
      };
    else if (path === "/ops/12")
      body = {
        id: 12,
        action: "delete_files",
        ts: time,
        origin: "user",
        reverts_id: 10,
        undone_by: null,
        undoable: true,
        boxes: [box.id],
        files: [mediaFiles[0].id],
        payload: {
          files: [{ file_id: mediaFiles[0].id, name: mediaFiles[0].name }],
        },
      };
    else if (path === "/history/11/boxes/box-1")
      body = { op: 11, box, files, folders: [], box_tags: ["tag-1"], tags };
    else if (path === "/history/11/files/file-1")
      body = {
        op: 11,
        file: {
          file: mediaFiles[0],
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
                ...mediaFiles[2],
                media_type: "text/plain",
                id: `scroll-${index}`,
                name: `document-${index}.txt`,
              }))
            : mediaFiles,
      };
    else if (/^\/files\/file-[1-4]$/.test(path))
      body = {
        file: mediaFiles[Number(path.at(-1)) - 1],
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
      return route.fulfill({
        contentType: "image/svg+xml",
        body: path.includes("/file-2/")
          ? portrait
          : path.includes("/file-3/")
            ? options.stl
              ? modelPage
              : documentPage
            : path.includes("/file-4/")
              ? panorama
              : landscape,
      });
    else if (
      /^\/files\/(?:file-[1-4]|visible-[0-9]+)\/derived\/[a-z0-9_]+$/.test(path)
    ) {
      const recipe = path.split("/").at(-1);
      if (options.gpx && path.includes("/file-3/") && recipe !== "gpx_map_v1")
        errors.push(`Unexpected GPX recipe: ${recipe}`);
      if (
        options.stl &&
        path.includes("/file-3/") &&
        recipe !== "stl_thumb_512_v1" &&
        recipe !== "stl_preview_2048_v1"
      )
        errors.push(`Unexpected STL recipe: ${recipe}`);
      if (
        path.includes("/file-4/") &&
        recipe !== "video_thumb_512_v2" &&
        recipe !== "preview_1080p_v1"
      )
        errors.push(`Unexpected video recipe: ${recipe}`);
      const state = options.state || "ready";
      body = {
        state,
        generation: "fixture-generation",
        blob_key: mediaFiles[0].blob_key,
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
  if (await page.locator(".pdf-reader iframe").count()) {
    await expect(page.locator(".pdf-reader iframe")).toHaveAttribute(
      "src",
      "/api/v1/files/file-3/content",
    );
    await expect(
      page
        .frameLocator(".pdf-reader iframe")
        .getByText("Browser PDF reader · synthetic fixture"),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "在新标签页打开 PDF" }),
    ).toHaveAttribute("target", "_blank");
  }
  await page.evaluate(() => document.fonts.ready);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page).toHaveScreenshot(`${name}.png`, {
    fullPage: (await page.locator(".gallery").count()) === 0,
  });
  expect(errors).toEqual([]);
}
async function grid(page: Page, expectedImages = 4) {
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
              (image as HTMLImageElement).naturalWidth > 0 &&
              getComputedStyle(image).objectFit === "contain",
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
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
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
    page.getByRole("button", { name: /document-99.txt/ }),
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
    media_type: "text/plain",
    id: `auto-${index}`,
    name: `document-${index}.txt`,
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
      json: {
        files: [
          {
            ...files[2],
            media_type: "text/plain",
            id: "last",
            name: "last.txt",
          },
        ],
      },
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
  await expect(page.getByRole("button", { name: /last.txt/ })).toBeVisible();
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
          boxes: params.getAll("condition").some((raw) => {
            const c = JSON.parse(raw);
            return c.field === "keyword" && c.values.includes("年度报告");
          })
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
    if (params.has("condition")) {
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
      await page.getByRole("button", { name: "显示方式", exact: true }).click();
      await page.getByText(label, { exact: true }).click();
      const root = page.locator(
        kind === "boxes" ? ".box-grid" : ".file-scroll",
      );
      await expect(root).toHaveClass(new RegExp(`view-${mode}`));
      if (kind === "boxes") {
        await expect(root.locator(".entry-tags")).toContainText("旅行");
        await expect(root.locator(".entry-tags .tag-chip")).toHaveCount(2);
      }
      if (kind === "files") await grid(page, mode === "list" ? 0 : 4);
      if (kind === "files" && mode === "list")
        await expect(root.locator(".entry-tags")).toContainText("旅行");
      await page.reload();
      await expect(root).toHaveClass(new RegExp(`view-${mode}`));
      if (kind === "files") await grid(page, mode === "list" ? 0 : 4);
      await capture(page, errors, info.title);
    });
  }
  test(`display-${kind}-list-mobile`, async ({ page }, info) => {
    const errors = await prepare(page, { mobile: true });
    await page.goto(kind === "boxes" ? "/" : "/files");
    await page.getByRole("button", { name: "显示方式", exact: true }).click();
    await page.getByText("列表", { exact: true }).click();
    await expect(
      page.locator(kind === "boxes" ? ".box-grid" : ".file-scroll"),
    ).toHaveClass(/view-list/);
    if (kind === "boxes")
      await expect(page.locator(".entry-tags")).toContainText("旅行");
    if (kind === "files")
      await expect(page.locator(".file-scroll .entry-tags")).toContainText(
        "旅行",
      );
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(390);
    // The dismissed display menu can leave the pointer over a file row.
    // Capture the normal list state, independent of hover update timing.
    if (kind === "files") await page.mouse.move(0, 0);
    await capture(page, errors, info.title);
  });
}

test("box-list-long-description-preserves-tags", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.route("**/api/v1/boxes/search?**", (route) =>
    route.fulfill({
      json: {
        boxes: [
          {
            ...box,
            description: "旅途中的照片、票据和行程记录，".repeat(20),
            tags: [
              { id: "tag-1", name: "旅行" },
              { id: "tag-2", name: "山野" },
            ],
          },
        ],
      },
    }),
  );
  await page.goto("/");
  await page.getByRole("button", { name: "显示方式", exact: true }).click();
  await page.getByText("列表", { exact: true }).click();
  const title = page.locator(".box-summary h2");
  await expect(title).toHaveText(box.name);
  expect(
    await title.evaluate((node) => node.scrollWidth <= node.clientWidth),
  ).toBe(true);
  const description = page.locator(".box-summary-description");
  await expect(description).toBeVisible();
  expect(
    await description.evaluate((node) => node.scrollWidth > node.clientWidth),
  ).toBe(true);
  const tags = page.locator(".entry-tags");
  await expect(tags).toContainText("旅行山野");
  expect(
    await tags.evaluate((node) => node.scrollWidth <= node.clientWidth),
  ).toBe(true);
  const bounds = await tags.boundingBox();
  const summary = await page.locator(".box-summary").boundingBox();
  expect(
    Math.abs(bounds!.x + bounds!.width - summary!.x - summary!.width),
  ).toBeLessThan(1);
  await capture(page, errors, info.title);
});

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
    gallery.getByRole("heading", { name: "竖版证书.png", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("ArrowLeft");
  await expect(
    gallery.getByRole("heading", { name: "山间清晨.jpg", exact: true }),
  ).toBeVisible();
  await expect(gallery.locator("aside")).toHaveCount(0);
  await gallery.getByRole("button", { name: "文件详情", exact: true }).click();
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
    page.getByRole("button", { name: "旅行", exact: true }),
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
    gallery.getByRole("heading", { name: "竖版证书.png", exact: true }),
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
        file: { ...files[2], media_type: "text/plain", id, name: `${id}.txt` },
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
    .filter({ hasText: "document-12.txt" })
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
    await expect(gallery.locator("aside")).toHaveCount(0);
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
  await page.getByRole("button", { name: "显示方式", exact: true }).click();
  await page.getByRole("menuitem", { name: "小图标", exact: true }).click();
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
    const attachedTags = page.locator(".page-header .tag-chips");
    await expect(attachedTags).toContainText("旅行");
    const tagBounds = await attachedTags.boundingBox();
    const breadcrumbBounds = await page.getByLabel("目录路径").boundingBox();
    expect(tagBounds).not.toBeNull();
    expect(breadcrumbBounds).not.toBeNull();
    expect(breadcrumbBounds!.y).toBeGreaterThan(
      tagBounds!.y + tagBounds!.height,
    );
    await capture(page, errors, info.title);
    await page.reload();
    await expect(
      page.getByRole("button", { name: "第一天", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "山海拾光", exact: true }).click();
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
      page.locator("button").filter({ hasText: "重新选择原文件" }),
    ).toBeVisible();
    if (mobile) {
      // Off-screen virtual cards do not exist yet. Scroll the stable container
      // into view first, using the same settled position as the mobile grid case.
      await page.evaluate(() => document.fonts.ready);
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
    }
    await grid(page);
    if (mobile) {
      await page
        .getByRole("region", { name: "添加文件", exact: true })
        .scrollIntoViewIfNeeded();
    }
    await capture(page, errors, info.title);
  });
}

test("box-file-actions-compact", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.goto("/boxes/box-1");
  await grid(page);
  await expect(page.locator(".upload-panel")).toBeHidden();
  await expect(page.getByText(box.description!, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "添加内容", exact: true }).click();
  await expect(
    page.getByRole("menuitem", { name: "新建目录", exact: true }),
  ).toBeVisible();
  await capture(page, errors, info.title);
  await page.getByRole("menuitem", { name: "添加标签", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "添加标签", exact: true }),
  ).toBeVisible();
});

for (const [mode, mobile] of [
  ["list", false],
  ["medium", false],
  ["list", true],
] as const) {
  test(`unified-directory-${mode}-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    await page.addInitScript(
      (value) => localStorage.setItem("kist.display.files.v1", value),
      mode,
    );
    await page.route("**/api/v1/boxes/box-1/folders", (route) =>
      route.fulfill({
        json: {
          folders: [
            {
              id: "folder-1",
              box_id: "box-1",
              parent_id: null,
              name: "",
              created_at: time,
              updated_at: time,
            },
            {
              id: "child",
              box_id: "box-1",
              parent_id: "folder-1",
              name: "旅行文档",
              created_at: time,
              updated_at: time,
            },
          ],
        },
      }),
    );
    await page.goto("/boxes/box-1");
    await grid(page, mode === "list" ? 0 : 4);
    const entries = page.locator(".file-card");
    await expect(entries.first()).toHaveAccessibleName("旅行文档");
    await expect(entries).toHaveCount(5);
    await expect(page.locator(".folder-grid")).toHaveCount(0);
    if (mode === "list" && !mobile) {
      await expect(
        page.getByRole("button", { name: "修改时间: 降序", exact: true }),
      ).toBeVisible();
      await expect(
        page.locator(".file-list-kind").filter({ hasText: "PDF" }),
      ).toHaveCount(1);
    }
    await capture(page, errors, info.title);
  });
}

test("tag-context-menu-pointer", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.goto("/files");
  await grid(page);
  const row = page.locator(".tag-tree-row").filter({ hasText: "旅行" });
  const bounds = (await row.boundingBox())!;
  const point = { x: bounds.x + 65, y: bounds.y + 15 };
  await page.mouse.click(point.x, point.y, { button: "right" });
  const menu = page.getByRole("menu");
  await expect(
    page.getByRole("menuitem", { name: "重命名", exact: true }),
  ).toBeVisible();
  await expect(row).toHaveAttribute("data-active", "true");
  expect(
    await row.evaluate((element) => getComputedStyle(element).backgroundColor),
  ).not.toBe(
    await page
      .locator("aside")
      .evaluate((element) => getComputedStyle(element).backgroundColor),
  );
  await expect
    .poll(async () => {
      const rect = (await menu.boundingBox())!;
      return Math.abs(rect.x - point.x) < 3 && Math.abs(rect.y - point.y) < 3;
    })
    .toBe(true);
  await expect(
    page.getByRole("menuitem", { name: "重命名", exact: true }),
  ).toHaveCSS("font-size", "12px");
  await capture(page, errors, info.title);
  await page.keyboard.press("Escape");
  await expect(menu).not.toBeVisible();
  await expect(row).not.toHaveAttribute("data-active");
  await page.getByRole("button", { name: "旅行的操作", exact: true }).click();
  await expect(menu).toBeVisible();
  await expect(row).toHaveAttribute("data-active", "true");
});

test("stl-preview", async ({ page }, info) => {
  const errors = await prepare(page, { stl: true });
  await page.goto("/files?file=file-3");
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(gallery).toBeVisible();
  await expect(gallery.getByRole("img", { name: "模型.STL" })).toHaveAttribute(
    "src",
    /stl_preview_2048_v1/,
  );
  await capture(page, errors, info.title);
});

for (const variant of ["desktop", "mobile", "dateline"] as const) {
  test(
    variant === "desktop" ? "gpx-preview" : `gpx-${variant}-preview`,
    async ({ page }, info) => {
      const errors = await prepare(page, {
        gpx: true,
        mobile: variant === "mobile",
        dateline: variant === "dateline",
      });
      await page.goto("/files?file=file-3");
      const gallery = page.getByRole("region", { name: "画廊", exact: true });
      await expect(
        gallery.getByRole("button", { name: "放大地图" }),
      ).toBeVisible();
      await expect(gallery.getByText("正在加载地图…")).toHaveCount(0);
      await expect(gallery.locator("canvas")).toBeVisible();
      await capture(page, errors, info.title);
      if (variant === "mobile") {
        await gallery.locator("canvas").evaluate((element) => {
          const touch = (x: number) =>
            new Touch({
              identifier: 1,
              target: element,
              clientX: x,
              clientY: 250,
            });
          element.dispatchEvent(
            new TouchEvent("touchstart", {
              bubbles: true,
              cancelable: true,
              touches: [touch(300)],
            }),
          );
          element.dispatchEvent(
            new TouchEvent("touchend", {
              bubbles: true,
              cancelable: true,
              touches: [],
              changedTouches: [touch(80)],
            }),
          );
        });
      }
      await gallery.getByRole("button", { name: "放大地图" }).click();
      await expect(
        gallery.getByRole("heading", { name: "山间路线.GPX" }),
      ).toBeVisible();
      await gallery.getByRole("button", { name: "返回列表" }).click();
      await page
        .locator(".file-card")
        .filter({ hasText: "山间路线.GPX" })
        .scrollIntoViewIfNeeded();
      await expect(
        page.getByRole("img", { name: "山间路线.GPX" }),
      ).toBeVisible();
      await expect(page.locator(".gpx-preview canvas")).toHaveCount(0);
    },
  );
}

test("gpx-map-unavailable", async ({ page }, info) => {
  const errors = await prepare(page, { gpx: true, mapFailure: true });
  await page.goto("/files?file=file-3");
  const gallery = page.getByRole("region", { name: "画廊", exact: true });
  await expect(gallery.getByText("地图背景暂不可用")).toBeVisible();
  await expect(gallery.locator(".gpx-fallback polyline")).toHaveCount(1);
  await expect(gallery.getByRole("button", { name: "重试" })).toBeVisible();
  await capture(page, errors, info.title);
});

test("pdf-reader-unavailable", async ({ page }, info) => {
  const errors = await prepare(page, { mobile: true });
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "pdfViewerEnabled", {
      value: false,
      configurable: true,
    });
  });
  await page.goto("/files?file=file-3");
  await expect(
    page.getByText(
      "此浏览器无法在这里显示 PDF，请在新标签页打开或下载原文件。",
    ),
  ).toBeVisible();
  await expect(page.locator(".pdf-reader iframe")).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "在新标签页打开 PDF" }),
  ).toBeVisible();
  await capture(page, errors, info.title);
});

for (const mode of ["light", "dark"] as const) {
  test(`gallery-details-edge-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, { mode });
    await page.goto("/files?file=file-3");
    const toggle = page.getByRole("button", { name: "文件详情", exact: true });
    const rail = page.locator(".gallery-details-rail");
    await expect(page.locator(".pdf-reader iframe")).toBeVisible();
    await page.mouse.move(200, 100);
    await expect(toggle).toHaveCSS("opacity", "0");
    const edge = (await rail.boundingBox())!;
    const reader = (await page.locator(".pdf-reader iframe").boundingBox())!;
    expect(reader.x + reader.width).toBeLessThanOrEqual(edge.x);
    // The top of the rail is far from the centered button.
    await page.mouse.move(edge.x + edge.width / 2, edge.y + 24);
    await expect(toggle).toHaveCSS("opacity", "1");
    await capture(page, errors, info.title);
    await toggle.click();
    await expect(page.locator(".gallery-details")).toBeVisible();
    await toggle.click();
    await expect(page.locator(".gallery-details")).toHaveCount(0);
    await page.mouse.move(200, 100);
    await expect(toggle).toHaveCSS("opacity", "0");
    await page.getByRole("link", { name: "下载原文件", exact: true }).focus();
    await page.keyboard.press("Tab");
    await expect(toggle).toBeFocused();
    await expect(toggle).toHaveCSS("opacity", "1");
  });
}
