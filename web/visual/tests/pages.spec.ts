import { test, expect, type Page } from "@playwright/test";
import { colors, type Appearance } from "../../src/theme";
import type { ArchiveFile, Box, MediaJob, Tag } from "../../src/api";

// A real, deterministic two-page PDF exercises rendering, text and navigation.
function pdfFixture(slides = false) {
  const box = slides ? "800 450" : "600 800";
  const stream = (text: string) =>
    `BT /F1 24 Tf 40 ${slides ? 350 : 700} Td (${text}) Tj ET`;
  const first = stream("Kist PDF first page");
  const second = stream("Kist PDF second page");
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
    `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${box}] /Resources << /Font << /F1 5 0 R >> >> /Contents 6 0 R >>`,
    `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${box}] /Resources << /Font << /F1 5 0 R >> >> /Contents 7 0 R >>`,
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    `<< /Length ${first.length} >>\nstream\n${first}\nendstream`,
    `<< /Length ${second.length} >>\nstream\n${second}\nendstream`,
  ];
  let pdf = "%PDF-1.7\n";
  const offsets = [0];
  objects.forEach((object, index) => {
    offsets.push(pdf.length);
    pdf += `${index + 1} 0 obj\n${object}\nendobj\n`;
  });
  const xref = pdf.length;
  pdf += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`;
  for (const offset of offsets.slice(1))
    pdf += `${String(offset).padStart(10, "0")} 00000 n \n`;
  return Buffer.from(
    pdf +
      `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF`,
  );
}

const time = "2026-01-02T10:30:00Z";
const box: Box = {
  total_size: 493824,
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
  word?: "doc" | "docx" | "ppt" | "pptx";
  stl?: boolean;
  gpx?: boolean;
  mapFailure?: boolean;
  dateline?: boolean;
  text?: string;
  sheet?: boolean;
  textBody?: string;
  textReason?: string;
  textEncoding?: string;
  textTruncated?: boolean;
};
async function prepare(page: Page, options: Options = {}) {
  const officeFamily = options.word?.startsWith("ppt") ? "slides" : "office";
  const mediaFiles = options.text
    ? files.map((file, i) =>
        i === 2
          ? {
              ...file,
              name: options.text!,
              kind: "document",
              media_type: "text/plain",
            }
          : file,
      )
    : options.stl || options.gpx || options.word
      ? files.map((file, i) =>
          i === 2
            ? {
                ...file,
                name: options.word
                  ? `归档报告.${options.word}`
                  : options.gpx
                    ? "山间路线.GPX"
                    : "模型.STL",
                media_type: "application/octet-stream",
                kind: "other",
              }
            : file,
        )
      : options.state
        ? files.map((file, i) =>
            i === 0
              ? {
                  ...file,
                  name: "山间清晨.heic",
                  path: "旅途/山间清晨.heic",
                  media_type: "image/heic",
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
    if (options.sheet && path === "/files/file-3/sheet-preview") {
      const params = new URL(route.request().url()).searchParams;
      const sheet = Number(params.get("sheet") || 0);
      return route.fulfill({
        json: {
          supported: true,
          sheets: ["旅行费用", "行程"],
          sheet,
          row: 0,
          column: 0,
          cells:
            sheet === 0
              ? [
                  [{ text: "山海之旅 · 费用明细" }],
                  [{ text: "项目" }, { text: "金额" }, { text: "说明" }],
                  [
                    { text: "交通" },
                    { text: "1,280.00" },
                    { text: "往返车票" },
                  ],
                  [{ text: "住宿" }, { text: "960.00" }, { text: "两晚住宿" }],
                ]
              : [[{ text: "山间步道" }, { text: "2026-10-19" }]],
          widths: [24, 18, 24],
          merges:
            sheet === 0 ? [{ row: 0, column: 0, rows: 1, columns: 3 }] : [],
          more_rows: false,
          more_columns: false,
          truncated: false,
        },
      });
    }

    if (
      (options.text && path === "/files/file-3/text-preview") ||
      /^\/files\/(?:scroll-\d+|auto-\d+|last)\/text-preview$/.test(path)
    ) {
      const text =
        options.textBody ?? "Archive note\nA small plain text preview.\n";
      const detail =
        new URL(route.request().url()).searchParams.get("detail") === "true";
      return route.fulfill({
        json: {
          supported: !options.textReason,
          reason: options.textReason || "",
          encoding: options.textEncoding || "UTF-8",
          text: options.textReason
            ? ""
            : detail
              ? text
              : text.split("\n").slice(0, 20).join("\n"),
          truncated: !!options.textTruncated,
        },
      });
    }

    if (
      options.word &&
      path === `/files/file-3/derived/${officeFamily}_pdf_v1/content`
    )
      return route.fulfill({
        contentType: "application/pdf",
        body: pdfFixture(officeFamily === "slides"),
      });
    if (/^\/files\/(?:file-[12]|visible-[0-9]+)\/content$/.test(path))
      return route.fulfill({
        contentType: "image/svg+xml",
        body: path.includes("/file-2/") ? portrait : landscape,
      });
    if (path === "/files/file-3/content")
      return route.fulfill({
        contentType: "application/pdf",
        body: pdfFixture(),
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
            total_size: 493824,
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
      if (
        options.word &&
        path.includes("/file-3/") &&
        recipe !== `${officeFamily}_pdf_v1` &&
        recipe !== `${officeFamily}_thumb_512_v1`
      )
        errors.push(`Unexpected Office recipe: ${recipe}`);
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
  if (await page.getByRole("link", { name: "在新标签页打开 PDF" }).count()) {
    await expect(page.locator(".pdf-reader")).toBeVisible();
    await expect(
      page.locator(".pdf-reader").getByText("正在加载…", { exact: true }),
    ).toHaveCount(0);
  }
  if (await page.locator(".pdf-viewport-wrap:not([hidden])").count()) {
    await expect(
      page.locator('.pdfViewer .page[data-page-number="1"] canvas').first(),
    ).toBeVisible();
    await expect(
      page.locator('.pdfViewer .page[data-page-number="1"] .textLayer'),
    ).toContainText("Kist PDF first page");
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
    } else if (scenario === "box-files") {
      await grid(page);
      const path = page.getByLabel("目录路径");
      await expect(
        path.getByRole("link", { name: "根目录", exact: true }),
      ).toHaveAttribute("href", "/");
      await expect(
        path.getByRole("button", { name: "归档盒", exact: true }),
      ).toHaveAttribute("aria-current", "page");
    } else if (scenario === "empty")
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
      await expect(
        scenario === "tags"
          ? page
              .getByRole("dialog")
              .getByRole("textbox", { name: "名称", exact: true })
          : page.getByRole("dialog").locator('input[type="radio"]:checked'),
      ).toBeFocused();
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
        .getByText("旅途/山间清晨.heic", { exact: true }),
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
  expect(new URL(page.url()).searchParams.get("q")).toBe("旅行");
  await expect(gallery.locator(".detail-image")).toHaveAttribute(
    "src",
    "/api/v1/files/file-1/content",
  );
  const openImage = gallery.getByRole("link", { name: "在新标签页打开图片" });
  await expect(openImage).toHaveAttribute(
    "href",
    "/api/v1/files/file-1/content",
  );
  await expect(openImage).toHaveAttribute("target", "_blank");
  await expect(
    gallery.getByRole("link", { name: "下载原文件" }),
  ).toHaveAttribute("href", "/api/v1/files/file-1/content?download=true");
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
  await page.route(/\/api\/v1\/files\/scroll-\d+$/, (route) => {
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
        total_size: 0,
        created_at: time,
        updated_at: time,
      },
      {
        id: "photos",
        parent_id: "root",
        name: "照片",
        total_size: 0,
        created_at: time,
        updated_at: time,
      },
      {
        id: "day1",
        parent_id: "photos",
        name: "第一天",
        total_size: 0,
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
              total_size: 0,
              created_at: time,
              updated_at: time,
            },
            {
              id: "child",
              box_id: "box-1",
              parent_id: "folder-1",
              name: "旅行文档",
              total_size: 0,
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
  await page.route("**/api/v1/files/file-3/content", (route) =>
    route.fulfill({ contentType: "application/pdf", body: "invalid PDF" }),
  );
  await page.goto("/files?file=file-3");
  await expect(
    page.getByText(
      "无法读取此 PDF，文件可能已损坏或需要密码。请在新标签页打开或下载原文件。",
    ),
  ).toBeVisible();
  await expect(page.locator(".pdf-viewport")).toBeHidden();
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
    await expect(
      page.getByRole("button", { name: "下一页", exact: true }),
    ).toBeEnabled();
    await page.mouse.move(200, 100);
    await expect(toggle).toHaveCSS("opacity", "0");
    const edge = (await rail.boundingBox())!;
    const reader = (await page.locator(".pdf-viewport").boundingBox())!;
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

for (const mobile of [false, true]) {
  test(`pdf-reader-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile });
    await openReaderFromThumbnail(page, "/api/v1/files/file-3/content", mobile);
    const number = page.getByRole("spinbutton", { name: "页码" });
    await expect(
      page.getByRole("button", { name: "下一页", exact: true }),
    ).toBeEnabled();
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await expect(number).toHaveValue("2");
    await expect(
      page.locator('.page[data-page-number="2"] .textLayer'),
    ).toContainText("Kist PDF second page");
    await page.getByRole("button", { name: "上一页", exact: true }).click();
    await expect(number).toHaveValue("1");
    await page.getByRole("textbox", { name: "搜索 PDF 文本" }).fill("second");
    await expect(page.locator(".textLayer .highlight.selected")).toHaveText(
      "second",
    );
    await expect(number).toHaveValue("2");
    await page.getByRole("textbox", { name: "搜索 PDF 文本" }).fill("");
    await page.getByRole("button", { name: "上一页", exact: true }).click();
    const sheet = page.locator('.page[data-page-number="1"]');
    const before = (await sheet.boundingBox())!.width;
    const viewport = (await page.locator(".pdf-viewport").boundingBox())!;
    if (!mobile) {
      // The 600pt fixture stays at or below 125% on a wide screen.
      expect(before).toBeLessThanOrEqual(1002);
      expect(viewport.width - before).toBeGreaterThan(100);
    } else {
      expect(before).toBeLessThanOrEqual(viewport.width);
    }
    await page.getByRole("button", { name: "放大", exact: true }).click();
    await expect
      .poll(async () => (await sheet.boundingBox())!.width)
      .toBeGreaterThan(before);
    await page.getByRole("button", { name: "适合宽度", exact: true }).click();
    if (mobile) {
      await expect
        .poll(async () => Math.abs((await sheet.boundingBox())!.width - before))
        .toBeLessThan(2);
    } else {
      await expect
        .poll(async () => (await sheet.boundingBox())!.width)
        .toBeGreaterThan(before);
    }
    await page.locator(".pdf-viewport").focus();
    await page.keyboard.press("ArrowRight");
    await expect(page).toHaveURL(/file=file-3/);
    await capture(page, errors, info.title);
  });
}

for (const word of ["doc", "docx", "ppt", "pptx"] as const) {
  test(`${word.startsWith("ppt") ? "slides" : "word"}-reader-${word}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, {
      word,
      mode: word === "doc" || word === "ppt" ? "dark" : "light",
    });
    await openReaderFromThumbnail(
      page,
      `/api/v1/files/file-3/derived/${word.startsWith("ppt") ? "slides" : "office"}_pdf_v1/content`,
    );
    const reader = page.locator(".pdf-reader");
    await expect(reader).toBeVisible();
    await expect(
      page.getByRole("link", { name: "在新标签页打开 PDF" }),
    ).toHaveAttribute(
      "href",
      new RegExp(
        `${word.startsWith("ppt") ? "slides" : "office"}_pdf_v1/content`,
      ),
    );
    await expect(
      page.getByRole("link", { name: "下载原文件" }),
    ).toHaveAttribute("href", /file-3\/content.*download/);
    await expect(
      page.locator('.pdfViewer .page[data-page-number="1"] .textLayer'),
    ).toContainText("Kist PDF first page");
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await expect(page.getByRole("spinbutton", { name: "页码" })).toHaveValue(
      "2",
    );
    await capture(page, errors, info.title);
  });
}

// Hold PDF bytes to exercise the transition instead of only the settled page.
async function openReaderFromThumbnail(
  page: Page,
  content: string,
  reduced = false,
) {
  await page.emulateMedia({
    reducedMotion: reduced ? "reduce" : "no-preference",
  });
  await page.addInitScript(() => {
    const animate = Element.prototype.animate;
    (window as unknown as { readerFlights: number }).readerFlights = 0;
    Element.prototype.animate = function (...args) {
      if (this.classList.contains("gallery-flight"))
        (window as unknown as { readerFlights: number }).readerFlights++;
      return animate.apply(this, args);
    };
  });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(`**${content}*`, async (route) => {
    await pending;
    await route.fallback();
  });
  await page.goto("/boxes/box-1");
  await expect(page.locator(".file-count")).toContainText("4");
  const card = page.locator('.file-card[data-item-id="file-3"]');
  await card.scrollIntoViewIfNeeded();
  const thumbnail = card.locator("img");
  await expect(thumbnail).toBeVisible();
  await expect
    .poll(() =>
      thumbnail.evaluate(
        (image: HTMLImageElement) => image.complete && image.naturalWidth > 0,
      ),
    )
    .toBe(true);
  await card.click();
  const reader = page.locator(".pdf-reader");
  await expect(reader).toHaveAttribute("aria-busy", "true");
  await expect(reader.getByRole("status")).toHaveText("正在加载…");
  const viewport = page.locator(".pdf-viewport");
  const before = (await viewport.boundingBox())!;
  await expect(viewport).toHaveCSS("opacity", "0");
  release();
  await expect(reader).toHaveAttribute("aria-busy", "false");
  await expect(viewport).toHaveCSS("opacity", "1");
  expect(await viewport.boundingBox()).toEqual(before);
  expect(
    await page.evaluate(
      () => (window as unknown as { readerFlights: number }).readerFlights,
    ),
  ).toBe(0);
  // Reopening with warm query/module caches must use the same transition.
  await page.getByRole("button", { name: "返回列表", exact: true }).click();
  await expect(reader).toHaveCount(0);
  await card.click();
  await expect(reader).toHaveAttribute("aria-busy", "false");
  expect(
    await page.evaluate(
      () => (window as unknown as { readerFlights: number }).readerFlights,
    ),
  ).toBe(0);
}

for (const mode of ["light", "dark", "mobile"] as const) {
  test(`text-source-${mode}`, async ({ page }, info) => {
    const body =
      '#include <iostream>\n// archive sample\nint main() {\n  const char* message = "' +
      "long text ".repeat(18) +
      '";\n  std::cout << message;\n  return 0;\n}\n';
    const errors = await prepare(page, {
      text: "sample.cpp",
      textBody: body,
      mode: mode === "dark" ? "dark" : "light",
      mobile: mode === "mobile",
    });
    await page.goto("/files?file=file-3");
    const reader = page.getByRole("region", { name: "文本阅读器" });
    await expect(reader).toBeVisible();
    await expect(reader.locator(".cm-lineNumbers")).toBeVisible();
    await expect(
      reader.getByRole("textbox", { name: "sample.cpp" }),
    ).toHaveAttribute("aria-readonly", "true");
    await expect(reader.locator(".text-language")).toContainText("C++");
    await expect(reader.locator(".cm-line span").first()).toBeVisible();
    const original = await reader.locator(".cm-content").innerText();
    await reader.getByRole("textbox", { name: "sample.cpp" }).press("x");
    expect(await reader.locator(".cm-content").innerText()).toBe(original);
    await reader.getByRole("button", { name: "自动换行" }).click();
    await expect(reader.locator(".cm-content")).toHaveClass(/cm-lineWrapping/);
    await reader.getByRole("textbox", { name: "搜索文本" }).fill("return");
    await reader.getByRole("button", { name: "下一个匹配" }).click();
    await expect(reader.locator(".cm-searchMatch-selected")).toContainText(
      "return",
    );
    await reader
      .getByRole("textbox", { name: "sample.cpp" })
      .press("ArrowRight");
    await expect(
      page.getByRole("heading", { name: "sample.cpp", exact: true }),
    ).toBeVisible();
    await capture(page, errors, info.title);
    await page.getByRole("button", { name: "返回列表" }).click();
    await page
      .locator('.file-card[data-item-id="file-3"]')
      .scrollIntoViewIfNeeded();
    await expect(page.locator(".text-thumbnail")).toContainText("#include");
    await expect(page.locator(".cm-editor")).toHaveCount(0);
  });
}
for (const state of [
  "plain",
  "legacy",
  "empty",
  "truncated",
  "binary",
  "encoding",
] as const) {
  test(`text-reader-${state}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      text: state === "plain" ? "README" : "archive.log",
      textBody:
        state === "empty"
          ? ""
          : "<script>window.bad = true</script>\n中文日志\n2026-10-19 INFO Archive opened\n",
      textEncoding: state === "legacy" ? "GB18030" : "UTF-8",
      textTruncated: state === "truncated",
      textReason: ["binary", "encoding"].includes(state) ? state : undefined,
    });
    await page.goto("/files?file=file-3");
    if (["binary", "encoding"].includes(state)) {
      await expect(
        page.locator(".media-placeholder [role=status]"),
      ).toContainText(state === "binary" ? "暂不支持" : "暂时无法解码");
    } else {
      await expect(page.locator(".cm-editor")).toBeVisible();
      if (state === "legacy") {
        await expect(page.locator(".text-language")).toContainText("GB18030");
        await expect(
          page.getByRole("combobox", { name: "文本编码" }),
        ).toHaveValue("auto");
      }
      if (state === "empty")
        await expect(
          page.getByText("空文本文件", { exact: true }),
        ).toBeVisible();
      else await expect(page.locator(".cm-content")).toContainText("<script>");
      if (state === "truncated")
        await expect(
          page.locator(".text-reader").getByRole("status"),
        ).toContainText("1 MiB");
    }
    await capture(page, errors, info.title);
  });
}

for (const mode of ["light", "dark"] as const) {
  test(`text-yaml-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      text: mode === "light" ? "settings.yaml" : "settings.yml",
      textBody:
        "# Archive settings\narchive:\n  name: 测试归档\n  read_only: true\n  count: 12\n  formats:\n    - text\n    - source\n",
      mode,
    });
    await page.goto("/files?file=file-3");
    const reader = page.getByRole("region", { name: "文本阅读器" });
    await expect(reader.locator(".text-language")).toContainText("YAML");
    const key = reader
      .locator(".cm-line span")
      .filter({ hasText: /^archive$/ });
    const value = reader
      .locator(".cm-line span")
      .filter({ hasText: /^测试归档$/ });
    await expect(key).toBeVisible();
    await expect(value).toBeVisible();
    const baseColor = await reader
      .locator(".cm-content")
      .evaluate((node) => getComputedStyle(node).color);
    expect(await key.evaluate((node) => getComputedStyle(node).color)).not.toBe(
      baseColor,
    );
    expect(
      await value.evaluate((node) => getComputedStyle(node).color),
    ).not.toBe(baseColor);
    await capture(page, errors, info.title);
  });
}

for (const mode of ["light", "dark"] as const) {
  test(`text-line-selection-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      text: "lines.txt",
      textBody: Array.from(
        { length: 80 },
        (_, i) => `Archive entry ${i + 1}`,
      ).join("\n"),
      mode,
    });
    await page.goto("/files?file=file-3");
    const number = (n: number) =>
      page
        .locator(".cm-lineNumbers .cm-gutterElement")
        .filter({ hasText: new RegExp(`^${n}$`) });
    await number(2).click();
    await expect(page).toHaveURL(/#L2$/);
    await expect(page.locator(".cm-selected-line")).toHaveCount(1);
    await number(2).click();
    await expect(page).not.toHaveURL(/#L/);
    await expect(page.locator(".cm-selected-line")).toHaveCount(0);
    await number(2).click();
    await number(6).click({ modifiers: ["Shift"] });
    await expect(page).toHaveURL(/#L2-6$/);
    await expect(page.locator(".cm-selected-line")).toHaveCount(5);
    const from = (await number(6).boundingBox())!;
    const to = (await number(3).boundingBox())!;
    await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
    await page.mouse.down();
    await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, {
      steps: 6,
    });
    await page.mouse.up();
    await expect(page).toHaveURL(/#L3-6$/);
    await expect(page.locator(".cm-selected-line")).toHaveCount(4);
    await page.reload();
    await expect(page.locator(".cm-selected-line")).toHaveCount(4);
    await number(4).click();
    await expect(page).not.toHaveURL(/#L/);
    await expect(page.locator(".cm-selected-line")).toHaveCount(0);
    await page.reload();
    await expect(page.locator(".cm-selected-line")).toHaveCount(0);
    await page.goto("/files?file=file-3#L45-48");
    await expect(page.locator(".cm-selected-line")).toHaveCount(4);
    await expect(number(45)).toBeInViewport();
    await expect(page.locator(".cm-selected-line").first()).toHaveText(
      "Archive entry 45",
    );
    await capture(page, errors, info.title);
    await page.getByRole("button", { name: "下一个文件", exact: true }).click();
    await expect(page).not.toHaveURL(/#L/);
  });
}

for (const mode of ["light", "dark", "mobile"] as const) {
  test(`markdown-reader-${mode}`, async ({ page }, info) => {
    const body = `# 旅行归档\n\n这份 **Markdown** 说明与照片一起保存，可在预览和源码之间切换。\n\n## 清单\n\n- [x] 整理照片\n- [ ] 补充旅行记录\n\n| 日期 | 地点 |\n| --- | --- |\n| 2026-10-19 | 山间步道 |\n\n> 保存原始文件，也保留阅读体验。\n\n\`\`\`yaml\narchive:\n  name: 山海拾光\n  read_only: true\n\`\`\`\n\n[本地照片](山间清晨.jpg) · [缺失附件](missing.pdf)\n\n![远程图片](https://example.org/private-tracker.png)\n\n<script>window.bad = true</script>\n`;
    const errors = await prepare(page, {
      text: "README.md",
      textBody: body,
      mode: mode === "dark" ? "dark" : "light",
      mobile: mode === "mobile",
    });
    let remoteRequests = 0;
    await page.route("https://example.org/**", (route) => {
      remoteRequests++;
      return route.abort();
    });
    await page.goto("/files?file=file-3");
    await expect(
      page.getByRole("heading", { name: "旅行归档", exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("table")).toBeVisible();
    await expect(page.getByRole("link", { name: "本地照片" })).toHaveAttribute(
      "href",
      "/files?file=file-1",
    );
    await expect(page.getByText(/引用的文件暂时无法访问/)).toBeVisible();
    expect(remoteRequests).toBe(0);
    await expect(
      page.locator(".markdown-body script, .markdown-body img"),
    ).toHaveCount(0);
    await page.getByRole("button", { name: "源码", exact: true }).click();
    await expect(
      page.getByRole("textbox", { name: "README.md" }),
    ).toBeVisible();
    await page
      .locator(".cm-lineNumbers .cm-gutterElement")
      .filter({ hasText: /^2$/ })
      .click();
    await expect(page).toHaveURL(/#L2$/);
    await page.reload();
    await expect(page.locator(".cm-selected-line")).toHaveCount(1);
    await page.getByRole("button", { name: "预览", exact: true }).click();
    await expect(page).not.toHaveURL(/#L/);
    await expect(
      page.getByRole("heading", { name: "旅行归档", exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: "本地照片" })).toBeVisible();
    await capture(page, errors, info.title);
  });
}

for (const mode of ["light", "dark", "mobile"] as const) {
  test(`sheet-reader-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      text: "travel.xlsx",
      sheet: true,
      mode: mode === "dark" ? "dark" : "light",
      mobile: mode === "mobile",
    });
    await page.goto("/files?file=file-3");
    const reader = page.getByRole("region", { name: "表格阅读器" });
    await expect(reader.locator(".univer-sheet")).toHaveAttribute(
      "data-state",
      "ready",
    );
    await reader.getByLabel("工作表", { exact: true }).selectOption("1");
    await expect(reader.locator(".univer-sheet")).toHaveAttribute(
      "data-state",
      "ready",
    );
    await reader.getByLabel("搜索当前页", { exact: true }).fill("山间步道");
    await reader
      .getByRole("button", { name: "下一个匹配", exact: true })
      .click();
    await expect(reader.locator(".sheet-selection")).toContainText("山间步道");
    await reader.getByLabel("工作表", { exact: true }).selectOption("0");
    await expect(reader.locator(".univer-sheet")).toHaveAttribute(
      "data-state",
      "ready",
    );
    await reader.getByLabel("搜索当前页", { exact: true }).fill("交通");
    await reader
      .getByRole("button", { name: "下一个匹配", exact: true })
      .click();
    await expect(reader.locator(".sheet-selection")).toContainText(
      "A3–A3 · 交通",
    );
    await capture(page, errors, info.title);
  });
}

for (const kind of ["boxes", "files"] as const) {
  test(`background-menu-${kind}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: kind === "files" ? "dark" : "light",
    });
    await page.goto(kind === "boxes" ? "/" : "/boxes/box-1");
    if (kind === "boxes")
      await expect(page.locator(".box-card")).toHaveCount(2);
    else await grid(page);
    const main = page.locator("main");
    const bounds = await main.boundingBox();
    const background = { x: bounds!.width - 8, y: bounds!.height - 8 };
    await main.click({ button: "right", position: background });
    const create = page.getByRole("menuitem", {
      name: kind === "boxes" ? "新建归档盒" : "新建目录",
      exact: true,
    });
    await expect(create).toBeVisible();
    await create.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(
      page
        .getByRole("dialog")
        .getByRole("textbox", { name: "名称", exact: true }),
    ).toBeFocused();
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await main.click({ button: "right", position: background });
    await expect(create).toBeVisible();
    if (kind === "files") {
      await expect(
        page.getByRole("menuitem", { name: "上传文件", exact: true }),
      ).toBeEnabled();
      await expect(
        page.getByRole("menuitem", { name: "添加标签", exact: true }),
      ).toBeVisible();
      await page
        .getByRole("menuitem", { name: "添加标签", exact: true })
        .click();
      await expect(
        page.getByRole("dialog").getByRole("combobox"),
      ).toBeFocused();
      await expect(
        page
          .getByRole("dialog")
          .getByRole("heading", { name: "标签", exact: true }),
      ).toHaveCount(0);
      await capture(page, errors, `${info.title}-add-tag`);
      await page.getByRole("button", { name: "关闭", exact: true }).click();
      await expect(page.getByRole("dialog")).toBeHidden();
      await main.click({ button: "right", position: background });
      await expect(
        page.getByRole("menuitem", { name: "操作历史", exact: true }),
      ).toBeVisible();
      await expect(
        page.getByRole("menuitem", { name: "移入回收站", exact: true }),
      ).toBeVisible();
      await page
        .getByRole("menuitem", { name: "编辑归档盒", exact: true })
        .click();
      await expect(
        page.getByRole("dialog", { name: "编辑归档盒", exact: true }),
      ).toBeVisible();
      await page.getByRole("button", { name: "取消", exact: true }).click();
      await expect(page.getByRole("dialog")).toBeHidden();
      await main.click({ button: "right", position: background });
    }
    await capture(page, errors, info.title);
  });
}

for (const kind of ["box", "file", "folder", "breadcrumb"] as const) {
  test(`entry-menu-${kind}`, async ({ page }, info) => {
    const errors = await prepare(page);
    if (kind === "folder" || kind === "breadcrumb") {
      await page.route("**/api/v1/boxes/box-1/folders", (route) =>
        route.fulfill({
          json: {
            folders: [
              {
                id: "folder-1",
                parent_id: null,
                name: "",
                total_size: 0,
                created_at: time,
                updated_at: time,
              },
              {
                id: "folder-child",
                parent_id: "folder-1",
                name: "旅途",
                total_size: 0,
                created_at: time,
                updated_at: time,
              },
            ],
          },
        }),
      );
    }
    await page.goto(
      kind === "box"
        ? "/"
        : kind === "breadcrumb"
          ? "/boxes/box-1?folder=folder-child"
          : "/boxes/box-1",
    );
    if (kind === "box") {
      const card = page.locator('.box-card[data-item-id="box-1"]');
      await card.press("Shift+F10");
      await expect(
        page.getByRole("menuitem", { name: "编辑归档盒", exact: true }),
      ).toBeVisible();
    } else if (kind === "breadcrumb") {
      await expect(
        page
          .getByLabel("目录路径")
          .getByRole("button", { name: "更多操作", exact: true }),
      ).toHaveCount(0);
      await expect(page.getByLabel("目录路径")).toContainText("根目录");
      await expect(
        page
          .getByLabel("目录路径")
          .getByRole("button", { name: "归档盒", exact: true }),
      ).toHaveAttribute("title", /山海拾光/);
      await page
        .locator('.folder-breadcrumbs [aria-current="page"]')
        .press("Shift+F10");
      await expect(
        page.getByRole("menuitem", { name: "复制链接", exact: true }),
      ).toBeVisible();
    } else {
      await grid(page);
      const card = page.locator(
        `[data-item-id="${kind === "file" ? "file-1" : "folder-child"}"]`,
      );
      await card.click({ button: "right" });
      if (kind === "folder") {
        await page
          .getByRole("menuitem", { name: "上传到此目录", exact: true })
          .click();
        await expect(page.getByRole("dialog")).toContainText("旅途");
      } else
        await expect(
          page.getByRole("menuitem", { name: "在新标签页打开", exact: true }),
        ).toHaveAttribute("href", "/files?file=file-1");
    }
    await capture(page, errors, info.title);
  });
}

for (const mobile of [false, true]) {
  test(`empty-folder-layout-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile, empty: true });
    await page.goto("/boxes/box-1");
    await expect(page.locator(".file-empty-state")).toBeVisible();
    const header = (await page.locator(".page-header").boundingBox())!;
    const actions = (await page.locator(".header-actions").boundingBox())!;
    expect(Math.abs(header.y - actions.y)).toBeLessThan(2);
    const path = (await page.getByLabel("目录路径").boundingBox())!;
    const filters = (await page
      .getByRole("region", { name: "筛选", exact: true })
      .boundingBox())!;
    expect(path.y).toBeGreaterThanOrEqual(header.y + header.height);
    expect(filters.y).toBeGreaterThan(path.y + path.height);
    await capture(page, errors, info.title);
  });
}
