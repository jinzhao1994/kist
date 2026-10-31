import { aiServicesFixture } from "../../src/aiServices.fixture";
import { test, expect, type Page } from "@playwright/test";
import { colors, type Appearance } from "../../src/theme";
import type { ArchiveFile, Box, MediaJob, Tag } from "../../src/api";

const gpsSettingDefaults = {
  automatic: true,
  endpoint: "https://photon.komoot.io/reverse",
  request_interval_seconds: 2,
  pause_on_error: true,
};
function visualAIServices(connected = true) {
  const view = aiServicesFixture({ revision: "visual", connected });
  view.proxies = [
    {
      number: 1,
      id: "HTTP 示例",
      type: "http",
      url: "http://proxy.example:7890",
      host: "",
      port: 0,
      server_name: "",
      secret_configured: false,
    },
    {
      number: 2,
      id: "Trojan 示例",
      type: "trojan",
      url: "",
      host: "proxy.example",
      port: 443,
      server_name: "proxy.example",
      secret_configured: true,
    },
  ];
  view.accounts = [
    {
      number: 1,
      id: "日常 ChatGPT",
      type: "chatgpt",
      proxy: 1,
      connected,
      email: "reader@example.test",
    },
    {
      number: 2,
      id: "OpenAI API",
      type: "openai",
      proxy: 2,
      connected,
      email: "",
    },
  ];
  view.features.transcription.account = 2;
  return view;
}
function emptyAIServices() {
  const view = visualAIServices(false);
  view.accounts = [];
  view.proxies = [];
  for (const feature of Object.values(view.features)) {
    feature.account = 0;
    feature.model = "";
  }
  return view;
}

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
  aiConnected?: boolean;
  client?: boolean;
  cover?: boolean;
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
  gps?: boolean;
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
    if (new URL(route.request().url()).pathname === "/api/v1/ai")
      return route.fulfill({
        json: {
          supported: !options.client,
          connected: options.aiConnected || false,
          email: options.aiConnected ? "reader@example.test" : "",
          pending: false,
          error: "",
        },
      });
    const path = new URL(route.request().url()).pathname.slice(
      "/api/v1".length,
    );
    if (
      /^\/files\/[^/]+\/metadata$/.test(path) &&
      route.request().method() === "GET"
    )
      return route.fulfill({ json: { state: "ready" } });
    if (path === "/geo/settings")
      return route.fulfill({
        json: {
          defaults: gpsSettingDefaults,
          explicit: [],
          automatic: true,
          endpoint: "https://photon.komoot.io/reverse",
          request_interval_seconds: 2,
          pause_on_error: true,
          revision: "visual",
        },
      });
    if (path === "/ai/services")
      return route.fulfill({
        json: options.aiConnected ? visualAIServices() : emptyAIServices(),
      });
    if (/^\/ai\/accounts\/\d+\/models$/.test(path))
      return route.fulfill({
        json: [
          { id: "gpt-6-luna", name: "GPT-6 Luna" },
          { id: "gpt-transcribe", name: "GPT Transcribe" },
        ],
      });
    if (
      /^\/(?:ai\/(?:ocr|desc|facts|orientation|transcription)|geo|metadata)\/jobs$/.test(
        path,
      )
    )
      return route.fulfill({ json: { items: [], next_cursor: "" } });
    if (
      path === "/metadata" ||
      path === "/geo" ||
      path === "/ai/ocr" ||
      path === "/ai/desc" ||
      path === "/ai/facts" ||
      path === "/ai/orientation" ||
      path === "/ai/transcription"
    )
      return route.fulfill({
        json: {
          paused: false,
          scanning: false,
          queued: 0,
          failed: 0,
          running: "",
        },
      });
    if (
      /^\/files\/[^/]+\/(?:ocr|desc|facts|orientation|transcription|geo)$/.test(
        path,
      ) &&
      route.request().method() === "GET"
    )
      return route.fulfill({ json: { state: "missing" } });
    if (route.request().method() !== "GET") {
      errors.push(`Unexpected write: ${path}`);
      return route.abort();
    }
    let body: unknown;
    if (path === "/navigation") {
      const params = new URL(route.request().url()).searchParams;
      const root = "/boxes/2026-01-01_" + encodeURIComponent(box.name);
      const folders = [
        { id: "folder-child", name: "旅途" },
        { id: "photos", name: "照片" },
        { id: "day1", name: "照片/第一天" },
        { id: "folder-set", name: "旅行证件", fileset: true },
        { id: "conflict-pages", name: "证件", fileset: true },
      ];
      const locations = [
        { box: box.id, kind: "box", path: root },
        ...(options.manyFiles
          ? Array.from({ length: 100 }, (_, index) => ({
              ...mediaFiles[2],
              id: `scroll-${index}`,
              name: `document-${index}.txt`,
            }))
          : mediaFiles
        ).map((file) => ({
          box: box.id,
          folder: file.folder_id === "folder-1" ? undefined : file.folder_id,
          file: file.id,
          kind: "file",
          path: root + "/" + encodeURIComponent(file.name),
        })),
        ...folders.map((folder) => ({
          box: box.id,
          folder: folder.id,
          kind: "folder",
          view: folder.fileset ? "folder" : undefined,
          path:
            root +
            "/" +
            folder.name.split("/").map(encodeURIComponent).join("/"),
        })),
        ...folders
          .filter((folder) => folder.fileset)
          .map((folder) => ({
            box: box.id,
            folder: "folder-1",
            file: folder.id,
            kind: "fileset",
            path: root + "/" + encodeURIComponent(folder.name),
          })),
      ];
      const requestedPath = params.get("path");
      const file = params.get("file");
      const folder = params.get("folder");
      const location = locations.find((item) =>
        requestedPath
          ? item.path === requestedPath &&
            (params.get("view") === "folder"
              ? item.kind === "folder"
              : item.kind !== "folder" || !("view" in item && item.view))
          : file
            ? "file" in item && item.file === file
            : folder && folder !== "folder-1"
              ? item.kind === "folder" &&
                "folder" in item &&
                item.folder === folder
              : params.get("box") === box.id && item.kind === "box",
      );
      if (!location) {
        errors.push(`Unexpected navigation: ${params}`);
        return route.abort();
      }
      return route.fulfill({ json: location });
    }
    if (options.sheet && path === "/files/file-3/sheet-preview") {
      const params = new URL(route.request().url()).searchParams;
      expect(params.get("v")).toBe(
        `sheet-v1:${mediaFiles[2].blob_key}:${mediaFiles[2].name}`,
      );
      const sheet = Number(params.get("sheet") || 0);
      return route.fulfill({
        headers: { "Cache-Control": "private, max-age=31536000, immutable" },
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
    if ((options.gpx || options.gps) && path === "/map-provider")
      body = {
        style_url: "/api/v1/fixture-map-style",
        terrain_tiles: "",
        terrain_attribution: "",
        resource_origins: [],
      };
    else if ((options.gpx || options.gps) && path === "/fixture-map-style") {
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
    else if (path === "/boxes/box-1/cover" && options.cover)
      body = { file: files[1] };
    else if (path === "/saved-searches") body = { searches: [] };
    else if (path === "/boxes" || path === "/boxes/search")
      body = {
        boxes: options.empty
          ? []
          : [
              {
                ...box,
                cover_file_id: options.cover ? files[1].id : undefined,
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
    else if (path === "/client/connection-status")
      body = {
        state: "connected",
        server: "https://nas.example",
        pending_download: 3,
        pending_upload: 2,
      };
    else if (path === "/client/pending-history") body = { entries: [] };
    else if (path === "/catalog-revision")
      return route.fulfill({ json: { revision: "fixture-catalog" } });
    else if (path === "/info")
      body = {
        version: "visual",
        primary_color: options.primaryColor || "blue",
        client: options.client || false,
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
    else if (path === "/filesets") body = { filesets: [] };
    else if (path === "/files" || path === "/entries")
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
    ) {
      if (
        new URL(route.request().url()).searchParams.get("v") !==
        "fixture-generation"
      )
        errors.push(`Unexpected preview generation: ${route.request().url()}`);
      return route.fulfill({
        contentType: "image/svg+xml",
        headers: { "Cache-Control": "private, max-age=31536000, immutable" },
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
    } else if (
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
async function capture(
  page: Page,
  errors: string[],
  name: string,
  expectedAlerts: string[] = [],
) {
  if (await page.locator(".pdf-reader").count()) {
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
  }
  await page.evaluate(() => document.fonts.ready);
  // Allow opening menus to finish fading and closing menus to unmount.
  // Re-resolve on every poll: a locator captured during exit may disappear.
  await expect
    .poll(() =>
      page
        .getByRole("menu")
        .evaluateAll((menus) =>
          menus.every(
            (element) =>
              getComputedStyle(element).opacity === "1" &&
              element
                .getAnimations({ subtree: true })
                .every((animation) => animation.playState === "finished"),
          ),
        ),
    )
    .toBe(true);
  await expect(page.getByRole("alert")).toHaveCount(expectedAlerts.length);
  for (const [index, message] of expectedAlerts.entries())
    await expect(page.getByRole("alert").nth(index)).toContainText(message);
  await expect(page).toHaveScreenshot(`${name}.png`, {
    fullPage: (await page.locator(".gallery").count()) === 0,
  });
  // Preserve evidence for intermittent text raster/layout differences without
  // changing rendering, tolerances or the screenshot's readiness conditions.
  if (await page.getByRole("menu").count()) {
    const layout = await page.getByRole("menu").evaluateAll((menus) =>
      menus.map((menu) =>
        [...menu.querySelectorAll('[role="menuitem"]')].map((item) => {
          const style = getComputedStyle(item);
          const label = item.querySelector(".mantine-Menu-itemLabel") || item;
          const range = document.createRange();
          range.selectNodeContents(label);
          return {
            tag: item.tagName,
            text: item.textContent,
            rect: item.getBoundingClientRect().toJSON(),
            textRect: range.getBoundingClientRect().toJSON(),
            font: style.font,
            lineHeight: style.lineHeight,
            transform: style.transform,
            zoom: style.zoom,
          };
        }),
      ),
    );
    await test.info().attach("menu-layout", {
      body: JSON.stringify(layout, null, 2),
      contentType: "application/json",
    });
  }
  expect(errors).toEqual([]);
}
async function scrollToFileGrid(page: Page) {
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
test("theme-initial-html", async ({ page }, info) => {
  const errors = await prepare(page, { primaryColor: "pink" });
  await page.route("**/files", async (route) => {
    if (!route.request().isNavigationRequest()) return route.fallback();
    const response = await route.fetch();
    const body = (await response.text()).replace(
      /name="kist-primary-color" content="[^"]*"/,
      'name="kist-primary-color" content="pink"',
    );
    await route.fulfill({ response, body });
  });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/info", async (route) => {
    await pending;
    await route.fallback();
  });
  await page.goto("/files");
  await grid(page);
  await expect
    .poll(() =>
      page.evaluate(() => {
        const style = getComputedStyle(document.documentElement);
        return (
          style.getPropertyValue("--mantine-primary-color-filled") ===
          style.getPropertyValue("--mantine-color-pink-filled")
        );
      }),
    )
    .toBe(true);
  await capture(page, errors, info.title);
  release();
});

test("theme-initial-loading", async ({ page }, info) => {
  const errors = await prepare(page, { primaryColor: "pink" });
  // Exercise fallback for hosts without an HTML-injected theme.
  await page.route("**/files", async (route) => {
    if (!route.request().isNavigationRequest()) return route.fallback();
    const response = await route.fetch();
    const body = (await response.text()).replace(
      /name="kist-primary-color" content="[^"]*"/,
      'name="kist-primary-color" content=""',
    );
    await route.fulfill({ response, body });
  });
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/info", async (route) => {
    await pending;
    await route.fallback();
  });
  await page.goto("/files");
  await expect(page.getByRole("status")).toHaveText("正在加载…");
  await expect(page.locator(".app-shell")).toHaveCount(0);
  await capture(page, errors, info.title);
  release();
  await grid(page);
  await expect
    .poll(() =>
      page.evaluate(() => {
        const style = getComputedStyle(document.documentElement);
        return (
          style.getPropertyValue("--mantine-primary-color-filled") ===
          style.getPropertyValue("--mantine-color-pink-filled")
        );
      }),
    )
    .toBe(true);
});

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
        ? "/boxes/id/box-1"
        : scenario === "mobile"
          ? "/files"
          : "/",
    );
    if (scenario === "mobile") {
      await expect(page.locator(".file-count")).toContainText("4");
      await expect(page.locator(".file-card-source").first()).toContainText(
        "山海拾光",
      );
      await page.evaluate(() => document.fonts.ready);
      // Column measurement changes the document height after the first render.
      // Wait for the requested scroll position, not just the file-count text.
      await scrollToFileGrid(page);
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
      await expect(page.locator(".file-card-source")).toHaveCount(0);
      await expect(
        page.locator(".file-name-cell .file-card-size").first(),
      ).toBeVisible();
      await expect(page.locator(".file-card-size").first()).not.toBeEmpty();
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
    if (scenario === "boxes") {
      const dates = page.locator(
        '.box-card[data-item-id="box-1"] .box-activity time',
      );
      await expect(dates).toHaveText(["2026年1月1日", "3日"]);
    }
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
          : page
              .getByRole("dialog")
              .getByRole("combobox", { name: "明暗模式", exact: true }),
      ).toBeFocused();
    }
    await capture(page, errors, info.title);
  });
}
for (const failed of [false, true]) {
  test(`video-hls-${failed ? "failed" : "idle"}`, async ({ page }, info) => {
    const errors = await prepare(page);
    const fullConversions: string[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/derived/preview_1080p_v1"))
        fullConversions.push(request.url());
    });
    await page.route("**/api/v1/files/file-4/playback/index.m3u8?*", (route) =>
      route.fulfill({
        status: failed ? 503 : 200,
        contentType: failed
          ? "application/json"
          : "application/vnd.apple.mpegurl",
        body: failed
          ? JSON.stringify({
              code: "unavailable",
              message: "Video playback unavailable: dependency_missing",
            })
          : "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nsegments/0?v=synthetic\n#EXT-X-ENDLIST\n",
      }),
    );
    await page.goto("/files?file=file-4");
    if (failed) {
      await expect(
        page
          .getByRole("region", { name: "画廊", exact: true })
          .getByRole("status")
          .filter({ hasText: "预览生成失败" }),
      ).toBeVisible();
    } else {
      await expect
        .poll(() =>
          page.locator("video").evaluate((v) => ({
            duration: (v as HTMLVideoElement).duration,
            paused: (v as HTMLVideoElement).paused,
          })),
        )
        .toEqual({ duration: 6, paused: true });
    }
    expect(fullConversions).toEqual([]);
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
    const errors = await prepare(page, { state, aiConnected: true });
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
    await expect(
      page.getByRole("button", { name: "提取文字（OCR）", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "生成描述", exact: true }),
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
    await page.goto("/boxes/id/box-1");
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
      if (kind === "files")
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
    // The dismissed display menu can leave the pointer over an entry row.
    // Capture the normal list state, independent of hover update timing.
    await page.mouse.move(0, 0);
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
  await page.evaluate(() => document.fonts.ready);
  await expect
    .poll(() => title.evaluate((node) => node.scrollWidth <= node.clientWidth))
    .toBe(true);
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
  await gallery.getByRole("button", { name: "更多操作", exact: true }).click();
  const openImage = page.getByRole("menuitem", { name: "在新标签页打开图片" });
  await expect(openImage).toHaveAttribute(
    "href",
    "/api/v1/files/file-1/content",
  );
  await expect(openImage).toHaveAttribute("target", "_blank");
  await expect(
    page.getByRole("menuitem", { name: "下载原文件" }),
  ).toHaveAttribute("href", "/api/v1/files/file-1/content?download=true");
  await page.keyboard.press("Escape");
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
  await gallery.getByRole("button", { name: "更多操作", exact: true }).click();
  await expect(
    page.getByRole("menuitem", { name: "下载原文件" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
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
  let releaseRevision: () => void = () => {};
  const revisionReady = new Promise<void>((resolve) => {
    releaseRevision = resolve;
  });
  await page.route("**/api/v1/catalog-revision", async (route) => {
    await revisionReady;
    return route.fulfill({ json: { revision: "fixture-catalog" } });
  });
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
  await expect.poll(() => next).toBe(1);
  // A late initial catalog check invalidates the first prefetch. Wait for its
  // replacement before checking that navigation shares the in-flight request.
  releaseRevision();
  await expect.poll(() => next).toBe(2);
  await gallery.getByRole("button", { name: "下一个文件" }).click();
  release();
  await expect(
    gallery.getByRole("heading", { name: "旅行记录.pdf", exact: true }),
  ).toBeVisible();
  await expect(
    gallery.getByRole("button", { name: "下一个文件" }),
  ).toBeDisabled();
  await expect(gallery.locator("nav [role=status]")).toHaveText("3 / 3");
  expect(next).toBe(2);
  await expect(gallery.locator(".pdf-reader")).toBeVisible();
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
    await page.goto("/boxes/id/box-1");
    await page.getByRole("button", { name: "照片", exact: true }).click();
    await expect(page).toHaveURL(
      new RegExp(
        "/boxes/2026-01-01_" +
          encodeURIComponent(box.name) +
          "/" +
          encodeURIComponent("照片") +
          "$",
      ),
    );
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
    await page.goto("/boxes/id/box-1");
    await expect(page.getByText("已暂停，服务端已确认 40%。")).toBeVisible();
    await expect(
      page.locator("button").filter({ hasText: "重新选择原文件" }),
    ).toBeVisible();
    if (mobile) {
      // Off-screen virtual cards do not exist yet. Scroll the stable container
      // into view first, using the same settled position as the mobile grid case.
      await page.evaluate(() => document.fonts.ready);
      await scrollToFileGrid(page);
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
  await page.goto("/boxes/id/box-1");
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
    await page.goto("/boxes/id/box-1");
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
  await page
    .getByRole("region", { name: "画廊", exact: true })
    .getByRole("button", { name: "更多操作", exact: true })
    .click();
  await expect(
    page.getByRole("menuitem", { name: "在新标签页打开 PDF" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
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
    await page
      .getByRole("region", { name: "画廊", exact: true })
      .getByRole("button", { name: "更多操作", exact: true })
      .focus();
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
    await page
      .getByRole("region", { name: "画廊", exact: true })
      .getByRole("button", { name: "更多操作", exact: true })
      .click();
    await expect(
      page.getByRole("menuitem", { name: "在新标签页打开 PDF" }),
    ).toHaveAttribute(
      "href",
      new RegExp(
        `${word.startsWith("ppt") ? "slides" : "office"}_pdf_v1/content`,
      ),
    );
    await expect(
      page.getByRole("menuitem", { name: "下载原文件" }),
    ).toHaveAttribute("href", /file-3\/content.*download/);
    await page.keyboard.press("Escape");
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
  await page.goto("/boxes/id/box-1");
  await expect(page.locator(".file-count")).toContainText("4");
  // Bring the virtualized grid into view before waiting for an offscreen row.
  await page.locator(".file-scroll").scrollIntoViewIfNeeded();
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
    // The hash assertion above verifies automatic navigation. Pin the capture
    // viewport separately from CodeMirror's asynchronous measurement scroll.
    await page.evaluate(() => document.fonts.ready);
    await expect
      .poll(async () => {
        const selected = page.locator(".cm-selected-line").first();
        await selected.evaluate((node) => {
          node.scrollIntoView({ block: "center", behavior: "instant" });
        });
        // Let CodeMirror measure the newly visible lines before checking the
        // capture position; one scroll can be shifted by its height correction.
        await page.evaluate(
          () =>
            new Promise<void>((resolve) =>
              requestAnimationFrame(() =>
                requestAnimationFrame(() => resolve()),
              ),
            ),
        );
        return selected.evaluate((node) => {
          const line = node.getBoundingClientRect();
          const viewport = node
            .closest(".cm-scroller")!
            .getBoundingClientRect();
          return Math.abs(
            (line.top + line.bottom - viewport.top - viewport.bottom) / 2,
          );
        });
      })
      .toBeLessThan(1);
    await expect(page.locator(".cm-selected-line").last()).toBeInViewport();
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
    await page.goto(kind === "boxes" ? "/" : "/boxes/id/box-1");
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
          ? "/boxes/id/box-1?folder=folder-child"
          : "/boxes/id/box-1",
    );
    if (kind === "box") {
      const card = page.locator('.box-card[data-item-id="box-1"]');
      await card.press("Shift+F10");
      await expect(
        page.getByRole("menuitem", { name: "编辑归档盒", exact: true }),
      ).toBeVisible();
      await expect
        .poll(() =>
          page.getByRole("menu").evaluate((menu) => {
            const rect = menu.getBoundingClientRect();
            const scale = window.devicePixelRatio;
            return (
              Number.isInteger(rect.x * scale) &&
              Number.isInteger(rect.y * scale)
            );
          }),
        )
        .toBe(true);
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
        ).toHaveAttribute(
          "href",
          "/boxes/2026-01-01_" +
            encodeURIComponent(box.name) +
            "/" +
            encodeURIComponent(files[0].name),
        );
    }
    await capture(page, errors, info.title);
  });
}

for (const mobile of [false, true]) {
  test(`empty-folder-layout-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page, { mobile, empty: true });
    await page.goto("/boxes/id/box-1");
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

test("file-tags-dialog", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.goto("/files");
  await grid(page);
  await page
    .locator('.file-card[data-item-id="file-1"]')
    .click({ button: "right" });
  await page.getByRole("menuitem", { name: "添加标签", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "添加标签" });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByRole("combobox", { name: "选择标签" }),
  ).toBeFocused();
  await expect(dialog.locator(".tag-chip")).toContainText("山野");
  await capture(page, errors, info.title);
});

test("box-tags-dialog", async ({ page }, info) => {
  const errors = await prepare(page, { mode: "dark" });
  await page.goto("/boxes/id/box-1");
  await grid(page);
  const main = page.locator("main");
  const bounds = await main.boundingBox();
  await main.click({
    button: "right",
    position: { x: bounds!.width - 8, y: bounds!.height - 8 },
  });
  await page.getByRole("menuitem", { name: "添加标签", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "添加标签" });
  await expect(
    dialog.getByRole("combobox", { name: "选择标签" }),
  ).toBeFocused();
  await expect(
    dialog.getByRole("heading", { name: "标签", exact: true }),
  ).toHaveCount(0);
  await capture(page, errors, info.title);
});

test("trash-scope-mobile", async ({ page }, info) => {
  const errors = await prepare(page, { mobile: true });
  await page.goto("/trash?box=box-1");
  const scope = page.getByRole("combobox", { name: "历史范围" });
  await expect(scope).toContainText("山海拾光 · 2026-01-01");
  await expect(page.locator(".history-entry").first()).toBeVisible();
  const control = await scope.boundingBox();
  const value = await scope.locator(".select-value").boundingBox();
  expect(value!.y).toBeGreaterThanOrEqual(control!.y);
  expect(value!.y + value!.height).toBeLessThanOrEqual(
    control!.y + control!.height,
  );
  await capture(page, errors, info.title);
});

for (const mode of ["medium", "large"] as const) {
  test(`box-cover-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, { cover: true });
    await page.addInitScript(
      (mode) => localStorage.setItem("kist.display.boxes.v1", mode),
      mode,
    );
    await page.goto("/");
    const covered = page.locator('.box-card[data-item-id="box-1"]');
    await expect(covered.locator(".box-cover img")).toBeVisible();
    await expect(
      page.locator('.box-card[data-item-id="box-2"] .box-icon'),
    ).toBeVisible();
    await covered.click({ button: "right" });
    await expect(
      page.getByRole("menuitem", { name: "移除封面" }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await capture(page, errors, info.title);
  });
}

async function prepareFileset(page: Page, mobile = false) {
  const errors = await prepare(page, { mobile });
  const zero = "00000000-0000-0000-0000-000000000000";
  const set = {
    folder_id: "folder-set",
    kind: "document_pages",
    primary: zero,
    layout: "directory",
    members: files.slice(0, 2).map((f) => ({
      file_id: f.id,
      folder_id: zero,
      role: "page",
      language: "",
      default: false,
    })),
  };
  await page.route("**/api/v1/filesets", (route) =>
    route.fulfill({ json: { filesets: [set] } }),
  );
  await page.route("**/api/v1/filesets/folder-set", (route) =>
    route.fulfill({
      json: {
        fileset: set,
        box_id: "box-1",
        folder: {
          id: "folder-set",
          parent_id: "folder-1",
          name: "旅行证件",
          total_size: 246912,
          created_at: time,
          updated_at: time,
        },
      },
    }),
  );
  await page.route("**/api/v1/boxes/box-1/folders", (route) =>
    route.fulfill({
      json: {
        folders: [
          {
            id: "folder-1",
            parent_id: null,
            name: "",
            total_size: 493824,
            created_at: time,
            updated_at: time,
          },
          {
            id: "folder-set",
            parent_id: "folder-1",
            name: "旅行证件",
            total_size: 246912,
            created_at: time,
            updated_at: time,
          },
        ],
      },
    }),
  );
  await page.route(/\/api\/v1\/files\?/, (route) =>
    route.fulfill({
      json: { files: files.map((f) => ({ ...f, folder_id: "folder-set" })) },
    }),
  );
  await page.route(/\/api\/v1\/entries\?/, (route) =>
    route.fulfill({
      json: {
        files:
          new URL(route.request().url()).searchParams.get("folder") ===
          "folder-set"
            ? files.slice(0, 2)
            : [
                {
                  ...files[0],
                  folder_id: "folder-1",
                  name: "旅行证件",
                  fileset_id: "folder-set",
                  primary_id: files[0].id,
                  size: 246912,
                },
              ],
      },
    }),
  );
  return errors;
}
test("fileset-extraction-members", async ({ page }, info) => {
  const errors = await prepareFileset(page, false);
  await page.route(/\/api\/v1\/files\?/, (route) =>
    route.fulfill({
      json: {
        files: files.slice(0, 2).map((file, index) => ({
          ...file,
          folder_id: "folder-set",
          extraction_pending: index === 1 ? ["media"] : [],
        })),
      },
    }),
  );
  await page.goto("/boxes/id/box-1");
  await expect(
    page.getByRole("button", { name: "读取元数据", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("尚未完整读取元数据。", { exact: true }),
  ).toBeVisible();
  await capture(page, errors, info.title);
});

for (const mobile of [false, true]) {
  test(`fileset-pages-${mobile ? "mobile" : "desktop"}`, async ({
    page,
  }, info) => {
    const errors = await prepareFileset(page, mobile);
    await page.goto("/boxes/id/box-1?file=folder-set");
    const preview = page.getByRole("region", { name: "文件组合预览" });
    await expect(preview).toBeVisible();
    await expect(preview.getByRole("status", { name: "页码" })).toHaveText(
      "1 / 2",
    );
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await expect(preview.getByRole("status", { name: "页码" })).toHaveText(
      "2 / 2",
    );
    await expect(preview.locator("img")).toBeVisible();
    await capture(page, errors, info.title);
  });
}
test("fileset-editor", async ({ page }, info) => {
  const errors = await prepareFileset(page);
  await page.goto("/boxes/id/box-1?file=folder-set");
  await page.getByRole("button", { name: "更多操作", exact: true }).click();
  await page
    .getByRole("menuitem", { name: "配置文件组合", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("山间清晨.jpg");
  await expect(
    dialog.getByRole("button", { name: "保存", exact: true }),
  ).toBeEnabled();
  await capture(page, errors, info.title);
});

test("fileset-grid", async ({ page }, info) => {
  const errors = await prepareFileset(page);
  await page.goto("/boxes/id/box-1");
  await expect(page.locator(".file-count")).toContainText("1");
  await expect(page.locator(".fileset-indicator")).toBeVisible();
  await capture(page, errors, info.title);
});

test("fileset-gallery-navigation", async ({ page }, info) => {
  const errors = await prepareFileset(page);
  const zero = "00000000-0000-0000-0000-000000000000";
  await page.route("**/api/v1/filesets", (route) =>
    route.fulfill({
      json: {
        filesets: [
          {
            folder_id: "folder-set",
            kind: "document_pages",
            layout: "directory",
            primary: zero,
            members: [],
          },
        ],
      },
    }),
  );
  await page.route("**/api/v1/filesets/folder-set", (route) =>
    route.fulfill({
      json: {
        fileset: {
          folder_id: "folder-set",
          kind: "document_pages",
          layout: "directory",
          primary: zero,
          members: [],
        },
        box_id: "box-1",
        folder: {
          id: "folder-set",
          parent_id: "folder-1",
          name: "旅行证件",
          total_size: 0,
          created_at: time,
          updated_at: time,
        },
      },
    }),
  );
  await page.route(/\/api\/v1\/entries\?/, (route) =>
    route.fulfill({
      json: {
        files:
          new URL(route.request().url()).searchParams.get("folder") ===
          "folder-set"
            ? []
            : [
                files[0],
                {
                  ...files[1],
                  id: "folder-set",
                  name: "旅行证件",
                  fileset_id: "folder-set",
                  primary_id: zero,
                  no_primary: true,
                },
              ],
      },
    }),
  );
  await page.goto("/boxes/id/box-1?file=file-1");
  await expect(page.locator(".gallery")).toBeVisible();
  await page.getByRole("button", { name: "下一个文件", exact: true }).click();
  await expect(page).toHaveURL(/file=folder-set/);
  await expect(page.locator(".gallery")).toBeVisible();
  await expect(
    page.getByRole("region", { name: "文件组合预览" }),
  ).toContainText("/ 0");
  await expect(
    page.getByRole("button", { name: "下一页", exact: true }),
  ).toBeDisabled();
  await capture(page, errors, info.title);
});

for (const panel of [
  "sync",
  "relay",
  "syncing",
  "cache",
  "connection",
  "history",
  "history-list",
  "history-list-mobile",
  "history-undo",
  "history-undo-mobile",
  "conflict",
  "fileset-conflict",
  "cover-conflict",
  "cover-missing-conflict",
  "deletion-conflict",
  "deletion-mobile-conflict",
  "deletion-missing-conflict",
] as const) {
  test(`offline-client-${panel}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mobile:
        panel === "history-list-mobile" ||
        panel === "history-undo-mobile" ||
        panel === "deletion-mobile-conflict",
    });
    const historyUndo = panel.startsWith("history-undo");
    const conflict = panel.endsWith("conflict");
    const deletionConflict = panel.startsWith("deletion-");
    const coverConflict =
      panel === "cover-conflict" || panel === "cover-missing-conflict";
    const config = {
      folder_id: "conflict-pages",
      kind: "document_pages",
      layout: "flat",
      primary: "00000000-0000-0000-0000-000000000000",
      members: ["one", "two"].map((file_id) => ({
        file_id,
        folder_id: "00000000-0000-0000-0000-000000000000",
        role: "page",
        language: "",
        default: false,
      })),
    };
    const currentConfig = { ...config, layout: "directory" };
    const proposedConfig = {
      ...config,
      members: [...config.members].reverse(),
    };
    if (panel === "fileset-conflict") {
      await page.route("**/api/v1/filesets", (route) =>
        route.fulfill({ json: { filesets: [currentConfig] } }),
      );
      await page.route("**/api/v1/files?**", (route) =>
        route.fulfill({
          json: {
            files: ["one", "two"].map((id, i) => ({
              id,
              name: i === 0 ? "正面.png" : "背面.png",
              folder_id: "conflict-pages",
              box_id: "box-1",
            })),
          },
        }),
      );
      await page.route("**/api/v1/boxes/box-1/folders", (route) =>
        route.fulfill({
          json: {
            folders: [
              { id: "root", parent_id: null, name: "" },
              { id: "conflict-pages", parent_id: "root", name: "证件" },
            ],
          },
        }),
      );
    }
    await page.route("**/api/v1/info", (route) =>
      route.fulfill({
        json: {
          client: true,
          version: "visual",
          timezone: "UTC",
          projection_dirty: false,
          max_upload_bytes: 1073741824,
          max_upload_files: 100,
          resumable_upload_version: 2,
          upload_chunk_bytes: 8388608,
        },
      }),
    );
    await page.route("**/api/v1/settings/storage", (route) =>
      route.fulfill({
        json: {
          originals_bytes: 2147483648,
          metadata_bytes: 8388608,
          previews_bytes: 134217728,
          video_bytes: 536870912,
        },
      }),
    );
    await page.route("**/api/v1/settings/credentials", (route) =>
      route.fulfill({ json: [] }),
    );
    await page.route("**/api/v1/settings", (route) =>
      route.fulfill({
        json: {
          revision: "visual",
          mode: "client",
          id: "client",
          library_id: "remote",
          credential_configured: true,
          fields: [],
          effective: {},
          overrides: [],
          restart_required: [],
          connection: { server: "https://nas.example", allow_http: false },
        },
      }),
    );
    await page.route("**/api/v1/client/**", (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/connection-status"))
        return route.fulfill({
          json: {
            state: "connected",
            server: "https://nas.example",
            pending_download: panel === "relay" ? 0 : 3,
            pending_upload: panel === "relay" ? 0 : 2,
            awaiting_root: panel === "relay" ? 1 : 0,
            syncing: panel === "syncing",
            progress:
              panel === "syncing"
                ? { direction: "download", count: 3 }
                : undefined,
            manual_required: panel !== "syncing" && panel !== "relay",
          },
        });
      if (route.request().method() !== "GET") {
        errors.push(`Unexpected client mutation: ${path}`);
        return route.abort();
      }
      if (path.endsWith("/status"))
        return route.fulfill({
          json: {
            settings: {
              version: 1,
              id: "laptop",
              library_id: "nas",
              timezone: "UTC",
              cache_max_age: "720h",
              cache_max_bytes: 10737418240,
            },
            catalog: {
              complete: true,
              confirmed: { library_id: "nas", op_id: 42, sum: "fixture" },
              downloaded: 42,
            },
            connection: { server: "https://nas.example", allow_http: false },
            blocked_id: conflict
              ? "01a15921-bef1-76e5-bddc-7c6b1024ff5b"
              : undefined,
            pending: [
              {
                id: "01a15921-bef1-76e5-bddc-7c6b1024ff5b",
                sequence: 1,
                occurred_at: time,
                action: deletionConflict
                  ? "delete_entry_contents"
                  : panel === "fileset-conflict"
                    ? "set_fileset"
                    : panel === "conflict" || coverConflict
                      ? "update_box"
                      : "create_box",
                conflict: conflict ? "配置已被修改" : undefined,
                awaiting_receipt: false,
                awaiting_root: panel === "relay",
              },
            ],
          },
        });
      if (path.endsWith("/edit") && deletionConflict)
        return route.fulfill({
          json: {
            fields: [],
            summary: {
              name: "旅行证件与交通住宿凭证",
              count: 1,
              current_names: false,
            },
            deletion: {
              box_id: "box-1",
              id: "folder-1",
              folder: true,
              whole_box: false,
              before: {
                name: "旅行证件与交通住宿凭证",
                file_count: 4,
                folder_count: 1,
                revision: "",
              },
              current:
                panel === "deletion-missing-conflict"
                  ? undefined
                  : {
                      name: "旅行证件与交通住宿凭证",
                      file_count: 6,
                      folder_count: 2,
                      revision: "reviewed",
                    },
            },
            replacement:
              panel === "deletion-missing-conflict"
                ? undefined
                : {
                    version: 2,
                    id: "deletion-replacement",
                    occurred_at: time,
                    origin: "user",
                    action: "delete_entry_contents",
                    payload: {},
                    deletion: { box: "box-1", last: "op:42" },
                  },
          },
        });
      if (path.endsWith("/edit") && coverConflict)
        return route.fulfill({
          json: {
            fields: [],
            summary: { name: "旅行归档", count: 1, current_names: true },
            cover: {
              before: { id: "previous", name: "出发前.png", available: true },
              current: { id: "current", name: "山间清晨.jpg", available: true },
              proposed: {
                id: "proposed",
                name: panel === "cover-conflict" ? "归途.jpg" : "",
                available: panel === "cover-conflict",
              },
            },
            replacement: {
              version: 1,
              id: "cover-replacement",
              occurred_at: time,
              action: "update_box",
              origin: "user",
              payload: {
                box_id: "box-1",
                cover_file_id: { before: "current", after: "proposed" },
              },
            },
          },
        });
      if (path.endsWith("/edit") && panel === "fileset-conflict")
        return route.fulfill({
          json: {
            fields: [],
            summary: { name: "证件", count: 1, current_names: true },
            fileset: {
              box_id: "box-1",
              folder_id: config.folder_id,
              name: "证件",
              before: config,
              current: currentConfig,
              proposed: proposedConfig,
            },
            replacement: {
              version: 1,
              id: "fileset-replacement",
              occurred_at: time,
              action: "set_fileset",
              origin: "user",
              payload: {
                box_id: "box-1",
                folder_id: config.folder_id,
                before: currentConfig,
                after: proposedConfig,
              },
            },
          },
        });
      if (path.endsWith("/edit"))
        return route.fulfill({
          json: {
            summary: { name: "旅行归档", count: 1, current_names: true },
            fields: [
              {
                key: "name",
                path: ["name", "after"],
                before: "旅行记录",
                current: "秋日旅行",
                proposed: "川西旅行",
              },
            ],
            replacement: {
              version: 1,
              id: "replacement",
              occurred_at: time,
              action: "update_box",
              origin: "user",
              payload: {
                box_id: "box",
                name: { before: "秋日旅行", after: "川西旅行" },
              },
            },
          },
        });
      if (path.endsWith("/cache"))
        return route.fulfill({
          json: {
            cached_bytes: 536870912,
            partial_bytes: 0,
            reserved_bytes: 0,
            pinned_bytes: 104857600,
            corrupt_bytes: 0,
            limit_bytes: 10737418240,
            over_limit: false,
          },
        });
      if (path.endsWith("/protected-storage"))
        return route.fulfill({
          json: { originals_bytes: 10485760, pending_journal_bytes: 4096 },
        });
      if (path.endsWith("/history"))
        return route.fulfill({
          json: {
            entries: [
              {
                id: "local-travel",
                action: historyUndo ? "undo" : "create_box",
                undo: historyUndo ? { op_id: 41 } : undefined,
                origin: "user",
                state: "accepted",
                occurred_at: time,
                receipt: { op_id: 42, accepted_at: time },
                summary: { name: "旅行记录", count: 1, current_names: false },
              },
              ...(panel.startsWith("history-list")
                ? [
                    {
                      id: "local-long-name",
                      action: "rename_entry",
                      origin: "user",
                      state: "pending",
                      occurred_at: time,
                      summary: {
                        name: "旅行票据",
                        count: 1,
                        before: "收据.png",
                        after:
                          "2026年秋季旅行交通住宿及景点门票原始凭证扫描归档.png",
                        current_names: false,
                      },
                    },
                    {
                      id: "local-tag",
                      action: "tag_files",
                      origin: "user",
                      state: "conflict",
                      occurred_at: time,
                      summary: {
                        name: "山间清晨.jpg",
                        count: 4,
                        tag: "旅行",
                        current_names: true,
                      },
                    },
                  ]
                : []),
            ],
          },
        });
      if (path.endsWith("/history/local-travel"))
        return route.fulfill({
          json: {
            id: "local-travel",
            action: historyUndo ? "undo" : "create_box",
            undo: historyUndo ? { op_id: 41 } : undefined,
            origin: "user",
            state: "accepted",
            occurred_at: time,
            receipt: { op_id: 42, accepted_at: time },
            summary: { name: "旅行记录", count: 1, current_names: false },
            proposal: {},
          },
        });
      if (path.endsWith("/offline"))
        return route.fulfill({
          json: {
            "file:file-1": { files: ["file-1"], filesets: [] },
            旅行资料: { files: ["file-1", "file-2"], filesets: [] },
          },
        });
      if (path.includes("/offline/"))
        return route.fulfill({
          json: {
            ready: true,
            available: 1,
            originals: [{ key: "fixture" }],
            previews: [],
            previews_available: 0,
          },
        });
      errors.push(`Unexpected client API: ${path}`);
      return route.abort();
    });
    await page.goto("/");
    await page
      .getByRole("navigation", { name: "个人归档" })
      .getByRole("button", { name: "数据同步", exact: true })
      .click();
    await expect(page.locator(".client-connection-status")).toContainText(
      panel === "syncing"
        ? "正在同步下载 3 个操作"
        : panel === "relay"
          ? "远端已连接"
          : "需要手动合并",
    );
    let dialog = page.getByRole("region", { name: "数据同步", exact: true });
    await expect(dialog.locator(".storage-usage-values")).toContainText(
      "本机原文件",
    );
    await expect(dialog.locator(".retention-table")).toBeVisible();
    await expect(dialog.locator(".retention-table")).toContainText(
      "可离线使用",
    );
    await expect(
      dialog
        .locator(".retention-table")
        .getByRole("button", { name: "准备离线" }),
    ).toHaveCount(0);
    await expect(
      dialog
        .locator("#sync-cache-title")
        .getByRole("button", { name: "缓存设置" }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/sync$/);
    await expect(
      dialog.getByRole("button", { name: "清理到期缓存", exact: true }),
    ).toHaveCount(0);
    await expect(dialog.locator(".settings-page-header")).toBeVisible();
    await expect(dialog.locator(".settings-page-header")).not.toContainText(
      "https://nas.example",
    );
    await expect(dialog.locator("#sync-status")).toContainText(
      "远端地址: https://nas.example",
    );
    await expect(dialog.locator("#sync-status").getByRole("status")).toHaveText(
      panel === "syncing"
        ? "正在同步下载 3 个操作"
        : panel === "relay"
          ? "远端已连接"
          : "需要手动合并",
    );
    await expect(
      dialog.getByText("可供浏览和检索", { exact: true }),
    ).toHaveCount(0);

    await expect(dialog.getByRole("button", { name: "立即同步" }))[
      panel === "syncing" ? "toBeDisabled" : "toBeEnabled"
    ]();
    if (panel === "relay") {
      await expect(dialog).toContainText("已交给远端，等待主库确认");
      await expect(page.locator(".client-connection-status")).toContainText(
        "待主库确认 · 1",
      );
      await expect(
        dialog.getByRole("button", { name: "查看本机修改", exact: true }),
      ).toHaveCount(0);
    }
    if (deletionConflict) {
      await dialog
        .getByRole("button", { name: "查看本机修改", exact: true })
        .click();
      if (panel === "deletion-missing-conflict") {
        await expect(dialog).toContainText("此对象已不存在");
        await expect(
          dialog.getByRole("button", { name: "保存已确认的删除方案" }),
        ).toHaveCount(0);
      } else {
        await expect(dialog).toContainText("6 个文件");
        await expect(
          dialog.getByRole("button", { name: "保存已确认的删除方案" }),
        ).toBeDisabled();
      }
    } else if (coverConflict) {
      await dialog
        .getByRole("button", { name: "查看本机修改", exact: true })
        .click();
      await expect(dialog).toContainText("当前值 · 山间清晨.jpg");
      const save = dialog.getByRole("button", { name: "保存调整后的修改" });
      if (panel === "cover-missing-conflict") {
        await expect(save).toBeDisabled();
        await expect(dialog).toContainText("此文件已不可用");
      } else {
        await expect(save).toBeEnabled();
        await expect(
          dialog.getByRole("combobox", { name: "准备提交的值" }),
        ).toHaveValue("归途.jpg");
      }
    } else if (panel === "fileset-conflict") {
      await dialog
        .getByRole("button", { name: "查看本机修改", exact: true })
        .click();
      await expect(
        dialog.getByRole("button", { name: "调整 背面.png 的顺序" }),
      ).toBeVisible();
      await expect(
        dialog.getByRole("button", { name: "保存", exact: true }),
      ).toBeEnabled();
    } else if (panel === "conflict") {
      await dialog
        .getByRole("button", { name: "查看本机修改", exact: true })
        .click();
      await expect(dialog.getByLabel("准备提交的值")).toHaveValue("川西旅行");
      await expect(dialog).toContainText("当前值 · 秋日旅行");
      await expect(
        dialog.getByRole("button", { name: "保存调整后的修改" }),
      ).toBeEnabled();
    } else if (panel === "cache") {
      await dialog.getByRole("link", { name: "离线内容", exact: true }).click();
      await expect(
        dialog.getByText("山间清晨.jpg", { exact: true }),
      ).toBeVisible();
      await expect(dialog.getByText("旅行资料", { exact: true })).toBeVisible();
      await expect(
        dialog.getByRole("button", { name: "缓存设置" }),
      ).toBeVisible();
    } else if (panel === "connection") {
      await page
        .getByRole("navigation", { name: "个人归档" })
        .getByRole("button", { name: "设置", exact: true })
        .click();
      dialog = page.getByRole("region", { name: "设置", exact: true });
      await dialog.getByRole("link", { name: "连接", exact: true }).click();
      await expect(dialog.getByLabel(/^远端地址/)).toHaveValue(
        "https://nas.example",
      );
      await expect(dialog.getByLabel(/^同步凭据/)).toHaveValue("");
      await expect(
        dialog.getByRole("button", { name: "凭据说明", exact: true }),
      ).toBeVisible();
      await expect(
        dialog.getByText("已保存凭据", { exact: true }),
      ).toBeVisible();
    } else if (panel.startsWith("history")) {
      await dialog.getByRole("link", { name: "本机操作", exact: true }).click();
      await expect(dialog).toContainText("远端已接纳");
      if (historyUndo) {
        await dialog.getByRole("button", { name: "撤销", exact: true }).click();
        await expect(
          dialog.getByRole("button", { name: "返回本机操作", exact: true }),
        ).toBeVisible();
        await dialog.getByRole("button", { name: "撤销", exact: true }).click();
        await expect(
          dialog.getByRole("button", { name: "取消", exact: true }),
        ).toBeVisible();
      } else if (panel === "history")
        await dialog
          .getByRole("button", { name: /创建归档箱.*旅行记录/ })
          .click();
      if (!historyUndo) await expect(dialog).toContainText("旅行记录");
      await expect(dialog).toContainText("远端操作 #42");
    } else {
      await expect(
        dialog.locator("#sync-status .client-pending-item"),
      ).toHaveCount(1);
      await expect(
        dialog.locator("#sync-status .client-pending-item"),
      ).toContainText("01a15921-bef1-76e5-bddc-7c6b1024ff5b");
    }
    await capture(page, errors, info.title);
  });
}

test("offline-retention-file", async ({ page }, info) => {
  const errors = await prepare(page);
  let retained = false,
    ready = false;
  await page.route("**/api/v1/info", (route) =>
    route.fulfill({
      json: {
        client: true,
        version: "visual",
        timezone: "UTC",
        projection_dirty: false,
        max_upload_bytes: 1073741824,
        max_upload_files: 100,
        resumable_upload_version: 2,
        upload_chunk_bytes: 8388608,
      },
    }),
  );
  await page.route("**/api/v1/client/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/connection-status"))
      return route.fulfill({
        json: {
          state: "connected",
          server: "https://nas.example",
          pending_download: 3,
          pending_upload: 2,
        },
      });
    if (path.endsWith("/status"))
      return route.fulfill({
        json: {
          settings: {
            version: 1,
            id: "laptop",
            library_id: "nas",
            timezone: "UTC",
            cache_max_age: "720h",
            cache_max_bytes: 10737418240,
          },
          catalog: {
            complete: true,
            confirmed: { library_id: "nas", op_id: 42, sum: "fixture" },
            downloaded: 42,
          },
          pending: [],
        },
      });
    if (path.endsWith("/offline"))
      return route.fulfill({
        json: retained
          ? { "file:file-1": { files: ["file-1"], filesets: [] } }
          : {},
      });
    if (path.includes("/offline/file%3Afile-1")) {
      if (route.request().method() === "PUT") {
        expect(route.request().postDataJSON()).toEqual({
          files: ["file-1"],
          filesets: [],
        });
        retained = true;
      }
      if (path.endsWith("/prepare")) ready = true;
      return route.fulfill({
        json: {
          ready,
          available: ready ? 1 : 0,
          previews_available: ready ? 1 : 0,
          previews: [
            {
              key: "photo",
              recipe: "thumb_512_v1",
              files: ["file-1"],
              available: ready,
            },
          ],
          originals: [{ key: "photo", size: 1024, files: ["file-1"] }],
          missing: ready ? [] : ["photo"],
          catalog: { complete: true },
        },
      });
    }
    errors.push(`Unexpected client API: ${path}`);
    return route.abort();
  });
  await page.goto("/files");
  await grid(page);
  await page
    .getByText("山间清晨.jpg", { exact: true })
    .first()
    .click({ button: "right" });
  await page.getByRole("menuitem", { name: "保留离线", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("button", { name: "保留并下载" }),
  ).toBeEnabled();
  await dialog.getByRole("button", { name: "保留并下载" }).click();
  await expect(dialog).toContainText(
    "所需原文件和预览已全部准备好，可离线使用。",
  );
  await expect(
    dialog.locator("footer").getByRole("button", { name: "关闭", exact: true }),
  ).toBeEnabled();
  await capture(page, errors, info.title);
});

test("history-synced-operation", async ({ page }, info) => {
  const errors = await prepare(page);
  await page.route("**/api/v1/ops/12", (route) =>
    route.fulfill({
      json: {
        id: 12,
        action: "delete_files",
        ts: time,
        accepted_at: "2026-01-09T08:00:00Z",
        origin: "user",
        reverts_id: 10,
        undone_by: null,
        undoable: true,
        boxes: [box.id],
        files: [files[0].id],
        payload: { files: [{ file_id: files[0].id, name: files[0].name }] },
      },
    }),
  );
  await page.goto("/history");
  await page.locator(".history-entry").first().click();
  await expect(
    page.getByRole("dialog").getByText(/远端接纳时间/),
  ).toBeVisible();
  await capture(page, errors, info.title);
});

for (const mode of ["grid", "list", "mobile", "dark", "prepare"] as const) {
  test(`offline-batch-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: mode === "dark" ? "dark" : "light",
    });
    if (mode === "mobile")
      await page.setViewportSize({ width: 390, height: 844 });
    if (mode === "list")
      await page.addInitScript(() =>
        localStorage.setItem("kist.display.files.v1", "list"),
      );
    await page.route("**/api/v1/info", (route) =>
      route.fulfill({
        json: {
          client: true,
          version: "visual",
          timezone: "UTC",
          projection_dirty: false,
          max_upload_bytes: 1073741824,
          max_upload_files: 100,
          resumable_upload_version: 2,
          upload_chunk_bytes: 8388608,
        },
      }),
    );
    await page.route("**/api/v1/client/status", (route) => {
      expect(route.request().method()).toBe("GET");
      return route.fulfill({
        json: {
          settings: {
            version: 1,
            id: "laptop",
            library_id: "nas",
            timezone: "UTC",
            cache_max_age: "720h",
            cache_max_bytes: 10737418240,
          },
          catalog: {
            complete: true,
            confirmed: { library_id: "nas", op_id: 42, sum: "fixture" },
            downloaded: 42,
          },
          pending: [],
        },
      });
    });
    await page.goto("/files");
    if (mode === "mobile") {
      // Off-screen virtual cards are not mounted. Use the same stable scroll
      // boundary as the existing mobile grid before waiting for its previews.
      await page.evaluate(() => document.fonts.ready);
      await scrollToFileGrid(page);
    }
    await grid(page, mode === "list" ? 0 : 4);
    await page
      .getByRole("button", { name: "选择离线文件", exact: true })
      .click();
    await page.locator('[data-item-id="file-1"]').click();
    await page.locator('[data-item-id="file-2"]').click();
    await expect(page.locator('[data-item-id="file-1"]')).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(page.getByText("已选 2 项", { exact: true })).toBeVisible();
    if (mode === "mobile") {
      // Clicking the first two cards scrolls the window and can unmount later
      // virtual rows. Restore the capture boundary and recheck every preview.
      await scrollToFileGrid(page);
      await grid(page);
    }
    if (mode === "prepare") {
      await page.getByRole("button", { name: "保留离线", exact: true }).click();
      await expect(page.getByRole("dialog")).toContainText("已保留 0 / 2 项");
    }
    await capture(page, errors, info.title);
  });
}

for (const scenario of [
  "appearance",
  "ai",
  "ai-loading",
  "ai-error",
  "ai-empty",
  "ai-paused",
  "ai-description",
  "ai-facts",
  "ai-trojan",
  "ai-http-auth",
  "ai-api-account",
  "ai-proxy-create",
  "ai-proxy-invalid",
  "ai-proxy-test",
  "ai-proxy-test-error",
  "ai-account-create",
  "ai-account-test",
  "ai-account-test-error",
  "ai-transcription",
  "service",
  "maps",
  "location",
  "credentials",
  "credentials-create",
  "credentials-empty",
  "hint",
  "client-storage",
  "storage",
  "storage-mobile",
  "storage-dark",
  "unsaved",
  "unsaved-dark",
  "unsaved-mobile",
  "save-hint",
  "save-invalid",
  "mobile",
  "dark",
] as const) {
  test(`instance-settings-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode:
        scenario === "dark" ||
        scenario === "storage-dark" ||
        scenario === "unsaved-dark"
          ? "dark"
          : "light",
      client: scenario === "client-storage",
    });
    if (
      scenario === "mobile" ||
      scenario === "storage-mobile" ||
      scenario === "unsaved-mobile"
    )
      await page.setViewportSize({ width: 390, height: 844 });
    const client = scenario === "client-storage";
    const fields = [
      {
        key: "appearance.primary_color",
        kind: "select",
        value: "blue",
        default: "blue",
        explicit: false,
        restart: false,
      },
      {
        key: "server.map_style",
        kind: "text",
        value: "https://tiles.openfreemap.org/styles/liberty",
        default: "https://tiles.openfreemap.org/styles/liberty",
        explicit: false,
        restart: false,
      },
      {
        key: "server.listen",
        kind: "text",
        value: "127.0.0.1:8473",
        default: "127.0.0.1:8473",
        explicit: false,
        restart: true,
      },
      {
        key: "server.allowed_origins",
        kind: "list",
        value: ["https://archive.example"],
        default: [],
        explicit: true,
        restart: false,
      },
      {
        key: "server.max_upload_bytes",
        kind: "bytes",
        value: 34359738368,
        default: 34359738368,
        explicit: false,
        restart: false,
      },
      {
        key: "server.max_upload_files",
        kind: "integer",
        value: 1000,
        default: 1000,
        explicit: false,
        restart: false,
      },
      ...(client
        ? [
            {
              key: "client.cache_max_age",
              kind: "text",
              value: "720h",
              default: "720h",
              explicit: false,
              restart: false,
            },
            {
              key: "client.cache_max_bytes",
              kind: "bytes",
              value: 10737418240,
              default: 10737418240,
              explicit: false,
              restart: false,
            },
          ]
        : [
            {
              key: "library.timezone",
              kind: "text",
              value: "UTC",
              default: "Asia/Shanghai",
              explicit: true,
              restart: false,
            },
            {
              key: "library.language",
              kind: "text",
              value: "zh-CN",
              default: "zh-CN",
              explicit: false,
              restart: false,
            },
            {
              key: "storage.link_mode",
              kind: "select",
              value: "",
              default: "",
              explicit: false,
              restart: false,
              options: ["", "copy", "reflink", "hardlink", "symlink"],
            },
            {
              key: "previews.cache_max_age",
              kind: "text",
              value: "inf",
              default: "inf",
              unlimited: true,
              explicit: false,
              restart: false,
            },
            {
              key: "previews.cache_max_bytes",
              kind: "bytes",
              value: 32000000000,
              default: 32000000000,
              unlimited: true,
              explicit: false,
              restart: false,
            },
            {
              key: "video.cache_max_age",
              kind: "text",
              value: "30d",
              default: "30d",
              explicit: false,
              restart: false,
            },
            {
              key: "video.cache_max_bytes",
              kind: "bytes",
              value: 8589934592,
              default: 8589934592,
              explicit: false,
              restart: false,
            },
          ]),
    ];
    await page.route("**/api/v1/settings/storage", (route) =>
      route.fulfill({
        json: {
          originals_bytes: 2147483648,
          metadata_bytes: 8388608,
          previews_bytes: 134217728,
          video_bytes: 536870912,
        },
      }),
    );
    await page.route("**/api/v1/settings/credentials", (route) =>
      route.fulfill({
        json:
          scenario === "credentials"
            ? [
                {
                  id: "0123456789abcdef0123456789abcdef",
                  note: "旅行备用机",
                  created_at: "2026-10-23T00:00:00Z",
                },
                {
                  id: "fedcba9876543210fedcba9876543210",
                  created_at: "2026-10-21T00:00:00Z",
                },
              ]
            : [],
      }),
    );
    await page.route("**/api/v1/settings", (route) => {
      if (scenario === "save-invalid" && route.request().method() === "PUT")
        return route.fulfill({
          status: 400,
          json: {
            code: "invalid_request",
            class: "invalid",
            message: "Invalid settings",
            fields: [
              {
                field: "previews.cache_max_age",
                code: "duration",
                message: "Invalid duration",
              },
            ],
          },
        });
      return route.fulfill({
        json: {
          revision: "visual",
          mode: client ? "client" : "library",
          id: "instance",
          library_id: "library",
          credential_configured: false,
          fields,
          effective: { "server.listen": "127.0.0.1:19000" },
          overrides: ["server.listen"],
          restart_required: ["server.listen"],
        },
      });
    });
    if (scenario.startsWith("ai-")) {
      await page.route("**/api/v1/ai", (route) =>
        route.fulfill({
          json: {
            supported: true,
            connected: true,
            email: "reader@example.test",
            pending: false,
            error: "",
          },
        }),
      );
      await page.route("**/api/v1/ai/services", (route) => {
        const view = visualAIServices();
        if (scenario === "ai-http-auth")
          view.proxies![0] = { ...view.proxies![0], secret_configured: true };
        view.features.ocr.automatic = scenario === "ai-paused";
        view.features.desc.automatic = scenario === "ai-description";
        return route.fulfill({ json: view });
      });
      await page.route("**/api/v1/ai/accounts/*/models", async (route) => {
        if (scenario === "ai-loading") return;
        if (scenario === "ai-error")
          return route.fulfill({
            status: 502,
            json: { code: "internal", message: "Model service unavailable" },
          });
        await route.fulfill({
          json:
            scenario === "ai-empty"
              ? []
              : [{ id: "gpt-6-luna", name: "GPT-6 Luna" }],
        });
      });
      if (
        scenario === "ai-account-test" ||
        scenario === "ai-account-test-error"
      )
        await page.route("**/api/v1/ai/accounts/2/test", (route) =>
          route.fulfill({
            json: {
              models: ["synthetic-vision", "synthetic-text", "synthetic-audio"],
              elapsed_ms: 120,
              error:
                scenario === "ai-account-test-error" ? "authentication" : "",
            },
          }),
        );
      if (scenario === "ai-proxy-test" || scenario === "ai-proxy-test-error")
        await page.route("**/api/v1/ai/proxies/*/test", (route) =>
          route.fulfill({
            json: {
              ip: "192.0.2.1",
              country: "示例国家",
              region: "示例地区",
              city: "示例城市",
              elapsed_ms: 120,
              error: scenario === "ai-proxy-test-error" ? "timeout" : "",
            },
          }),
        );
      if (scenario === "ai-paused")
        await page.route("**/api/v1/ai/ocr", (route) =>
          route.fulfill({
            json: {
              paused: true,
              scanning: false,
              queued: 12,
              failed: 1,
              running: "",
              error: "context canceled",
            },
          }),
        );
    }
    await page.goto("/settings");
    const settings = page.getByRole("region", { name: "设置", exact: true });
    const fieldBounds = async (key: string) =>
      (await settings.locator(`[data-setting-field="${key}"]`).boundingBox())!;
    const pairedFields = async (
      first: string,
      second: string,
      stacked = false,
    ) => {
      const a = await fieldBounds(first),
        b = await fieldBounds(second);
      expect(Math.abs(a.width - b.width)).toBeLessThan(1);
      if (stacked) {
        expect(Math.abs(a.x - b.x)).toBeLessThan(1);
        expect(b.y).toBeGreaterThanOrEqual(a.y + a.height);
      } else {
        expect(Math.abs(a.y - b.y)).toBeLessThan(1);
        expect(b.x).toBeGreaterThanOrEqual(a.x + a.width);
      }
    };
    await expect(settings.locator(".color-choice")).toHaveCount(colors.length);
    await expect(
      settings.getByRole("radio", { name: "蓝色", exact: true }),
    ).toBeChecked();
    if (scenario === "save-invalid") {
      await settings.getByRole("link", { name: "存储", exact: true }).click();
      const retention = settings.getByRole("combobox", {
        name: "缩略图与预览保留时间",
        exact: true,
      });
      await retention.fill("30da21");
      await retention.press("Escape");
      const save = settings.getByRole("button", { name: "保存", exact: true });
      await expect(save).toBeEnabled();
      await expect(retention).not.toHaveAttribute("aria-invalid", "true");
      await save.click();
      await expect(retention).toHaveAttribute("aria-invalid", "true");
      await expect(page.getByRole("alert")).toHaveText(
        "设置未保存，请检查填写的内容。",
      );
      await expect(
        page
          .getByRole("alert")
          .getByRole("button", { name: "关闭", exact: true }),
      ).toBeVisible();
      await capture(page, errors, info.title, [
        "设置未保存，请检查填写的内容。",
      ]);
      return;
    }
    if (scenario === "save-hint") {
      const save = settings.getByRole("button", { name: "保存", exact: true });
      await expect(save).toBeDisabled();
      await save.locator("..").hover();
      await expect(page.getByRole("tooltip")).toHaveText("没有未保存的修改");
      await capture(page, errors, info.title);
      return;
    }
    await expect(
      page
        .getByRole("navigation", { name: "个人归档" })
        .getByRole("button", { name: "设置", exact: true }),
    ).toHaveClass(/active/);
    if (scenario.startsWith("unsaved")) {
      await settings
        .getByRole("textbox", { name: "资料库语言", exact: true })
        .fill("en");
      await page
        .getByRole("navigation", { name: "个人归档" })
        .getByRole("button", { name: "全部文件", exact: true })
        .click();
      const dialog = page.getByRole("dialog", {
        name: "有未保存的设置",
        exact: true,
      });
      await expect(dialog).toBeVisible();
      await expect(
        dialog.getByRole("button", { name: "继续编辑", exact: true }),
      ).toBeFocused();
      await expect(
        dialog.getByRole("button", { name: "保存并离开", exact: true }),
      ).toBeEnabled();
      await expect(page).toHaveURL(/\/settings(?:#.*)?$/);
      await capture(page, errors, info.title);
      return;
    }
    if (scenario === "ai" || scenario.startsWith("ai-")) {
      await settings
        .getByRole("link", { name: "AI 服务", exact: true })
        .click();
      await expect(
        settings.getByRole("button", { name: "添加账户", exact: true }),
      ).toBeEnabled();
      await expect(settings.locator(".section-index-subsection")).toHaveText([
        "代理",
        "AI 账户",
        "图片方向",
        "OCR",
        "描述",
        "结构化信息",
        "语音转文字",
      ]);
      await expect(
        settings.getByRole("heading", { name: "AI 功能", exact: true }),
      ).toHaveCount(0);
      await pairedFields(
        "ai_services.features.ocr.account",
        "ai_services.features.ocr.model",
      );
      await pairedFields(
        "ai_services.features.ocr.reasoning_effort",
        "ai_services.features.ocr.image_detail",
      );
    }
    if (scenario === "ai-proxy-test" || scenario === "ai-proxy-test-error") {
      const table = settings.getByRole("table", { name: "代理", exact: true });
      const row = table.getByRole("row").filter({
        has: page.getByRole("rowheader", { name: "HTTP 示例", exact: true }),
      });
      await row.hover();
      const test = row.getByRole("button", { name: "测试", exact: true });
      await expect(test).toBeEnabled();
      await test.click();
      if (scenario === "ai-proxy-test-error")
        await expect(row.getByRole("alert")).toHaveText(
          "测试超过 15 秒，请检查代理或稍后重试。",
        );
      else
        await expect(row.getByRole("status")).toContainText(
          "出口 IP：192.0.2.1 · 示例国家 · 示例地区 · 示例城市 · 120 ms",
        );
      await expect(test).toBeEnabled();
      if (scenario === "ai-proxy-test") {
        await test.hover();
        await expect(page.getByRole("tooltip")).toHaveText(
          "查询此代理的出口 IP 和大致位置，最多等待 15 秒。",
        );
      } else {
        await page.mouse.move(0, 0);
        await expect(page.getByRole("tooltip")).toHaveCount(0);
      }
    }
    if (
      scenario === "ai-account-test" ||
      scenario === "ai-account-test-error"
    ) {
      const row = settings
        .getByRole("table", { name: "AI 账户", exact: true })
        .getByRole("row")
        .filter({
          has: page.getByRole("rowheader", { name: "OpenAI API", exact: true }),
        });
      await row.hover();
      const test = row.getByRole("button", { name: "测试", exact: true });
      await test.click();
      if (scenario === "ai-account-test-error")
        await expect(row.getByRole("alert")).toHaveText(
          "认证失败，请检查 API Key 或重新登录。",
        );
      else {
        await expect(row.getByRole("status")).toContainText(
          "测试通过 · 3 个模型 · 120 ms",
        );
        await expect(
          row.getByText("synthetic-vision, synthetic-text, synthetic-audio", {
            exact: true,
          }),
        ).toHaveCount(0);
      }
      await expect(test).toBeEnabled();
      await expect(row.getByText("已配置", { exact: true })).toBeVisible();
      await page.mouse.move(0, 0);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
    }
    if (scenario === "ai-proxy-invalid") {
      const table = settings.getByRole("table", { name: "代理", exact: true });
      const count = await table.getByRole("rowheader").count();
      await settings
        .getByRole("button", { name: "添加代理", exact: true })
        .click();
      const editor = settings.getByRole("region", {
        name: "添加代理",
        exact: true,
      });
      await editor
        .getByRole("textbox", { name: "名称", exact: true })
        .fill("123");
      const address = editor.getByRole("textbox", {
        name: "代理地址",
        exact: true,
      });
      await address.fill("456");
      await editor
        .getByRole("button", { name: "添加代理", exact: true })
        .click();
      await expect(address).toHaveAttribute("aria-invalid", "true");
      await expect(
        editor.getByText("请输入有效的 HTTP(S) 或 SOCKS 代理地址。", {
          exact: true,
        }),
      ).toBeVisible();
      await expect(table.getByRole("rowheader")).toHaveCount(count);
    }
    if (["ai-proxy-create", "ai-account-create"].includes(scenario)) {
      const proxy = scenario === "ai-proxy-create";
      const title = proxy ? "代理" : "AI 账户";
      const add = proxy ? "添加代理" : "添加账户";
      const table = settings.getByRole("table", { name: title, exact: true });
      const count = await table.getByRole("rowheader").count();
      await settings.getByRole("button", { name: add, exact: true }).click();
      const editor = settings.getByRole("region", { name: add, exact: true });
      await expect(
        editor.getByRole("textbox", { name: "名称", exact: true }),
      ).toBeFocused();
      await editor.getByRole("button", { name: "取消", exact: true }).click();
      const trigger = settings.getByRole("button", { name: add, exact: true });
      await expect(trigger).toBeFocused();
      await expect(editor).toHaveCount(0);
      await trigger.click();
      await expect(
        editor.getByRole("textbox", { name: "名称", exact: true }),
      ).toBeFocused();
      await editor
        .getByRole("textbox", { name: "名称", exact: true })
        .fill(proxy ? "家庭代理" : "备用账户");
      if (proxy)
        await expect(
          editor.getByRole("textbox", { name: "名称", exact: true }),
        ).toHaveAttribute("aria-required", "true");
      if (proxy)
        await expect(
          editor.getByRole("textbox", { name: "代理地址", exact: true }),
        ).toHaveAttribute("type", "text");
      if (proxy)
        await editor
          .getByLabel("代理地址", { exact: true })
          .and(editor.locator("input"))
          .fill("http://proxy.example:7890");
      await expect(table.getByRole("rowheader")).toHaveCount(count);
      await expect(
        settings.getByRole("button", { name: "保存", exact: true }),
      ).toBeEnabled();
      const editorBounds = await editor.boundingBox(),
        tableBounds = await table.boundingBox();
      expect(
        editorBounds &&
          tableBounds &&
          editorBounds.y + editorBounds.height <= tableBounds.y,
      ).toBeTruthy();
    }
    if (["ai-trojan", "ai-http-auth", "ai-api-account"].includes(scenario)) {
      const table = settings.getByRole("table", {
        name: scenario === "ai-api-account" ? "AI 账户" : "代理",
        exact: true,
      });
      const row = table.getByRole("row").filter({
        has: page.getByRole("rowheader", {
          name:
            scenario === "ai-trojan"
              ? "Trojan 示例"
              : scenario === "ai-http-auth"
                ? "HTTP 示例"
                : "OpenAI API",
          exact: true,
        }),
      });
      await row.hover();
      await row.getByRole("button", { name: "编辑", exact: true }).click();
      const editor = settings.getByRole("region", {
        name: scenario === "ai-api-account" ? "编辑 AI 账户" : "编辑代理",
        exact: true,
      });
      await expect(editor).toBeVisible();
      await expect(
        table.getByRole("region", {
          name: scenario === "ai-api-account" ? "编辑 AI 账户" : "编辑代理",
          exact: true,
        }),
      ).toBeVisible();
      await expect(
        editor.getByRole("button", { name: "完成", exact: true }),
      ).toBeVisible();
      await editor.scrollIntoViewIfNeeded();
      if (scenario === "ai-trojan") {
        const typeIcon = editor.locator(".input-chevron");
        const numberControls = editor.locator(".input-number-controls");
        const eye = editor.getByRole("button", {
          name: "Toggle password visibility",
        });
        const icon = await typeIcon.evaluate((node) => {
          const style = getComputedStyle(node);
          return {
            width: style.width,
            height: style.height,
            color: style.color,
          };
        });
        const stepper = await numberControls
          .locator('button[data-direction="up"]')
          .evaluate((node) => {
            const style = getComputedStyle(node, "::after");
            return {
              width: style.width,
              height: style.height,
              color: style.color,
              maskSize: style.maskSize,
              divider: getComputedStyle(node).borderBottomWidth,
            };
          });
        expect(stepper.width).toBe(icon.width);
        expect(stepper.color).toBe(icon.color);
        expect(stepper.maskSize).toBe(`${icon.width} ${icon.height}`);
        expect(parseFloat(stepper.height) * 2).toBe(parseFloat(icon.height));
        expect(parseFloat(stepper.divider)).toBeGreaterThan(0);
        expect(
          await eye
            .locator("svg")
            .evaluate((node) => getComputedStyle(node).color),
        ).toBe(icon.color);
        const port = editor
          .getByLabel("端口", { exact: true })
          .and(editor.locator("input"));
        await numberControls.locator('button[data-direction="up"]').click();
        await expect(port).toHaveValue("444");
        await numberControls.locator('button[data-direction="down"]').click();
        await expect(port).toHaveValue("443");
        const password = editor
          .getByLabel("密码", { exact: true })
          .and(editor.locator("input"));
        await eye.click();
        await expect(password).toHaveAttribute("type", "text");
        await eye.click();
        await expect(password).toHaveAttribute("type", "password");
        await editor
          .getByRole("textbox", { name: "名称", exact: true })
          .focus();
        await page.mouse.move(0, 0);
      }
      if (scenario === "ai-http-auth") {
        const address = editor.getByRole("textbox", {
          name: "代理地址",
          exact: true,
        });
        await expect(address).toHaveAttribute("type", "text");
        await expect(address).toHaveValue("http://proxy.example:7890");
        await expect(editor.locator('input[type="password"]')).toHaveCount(0);
      } else if (scenario === "ai-api-account") {
        const key = editor
          .getByLabel("API Key", { exact: true })
          .and(editor.locator("input"));
        await expect(key).toHaveAttribute("type", "text");
        await expect(key).toHaveAttribute("autocomplete", "off");
        await expect(key).toHaveValue("");
        await expect(key).toHaveCSS("-webkit-text-security", "disc");
        expect(
          await key.evaluate((node) =>
            getComputedStyle(node, "::placeholder").getPropertyValue(
              "-webkit-text-security",
            ),
          ),
        ).toBe("none");
        await key.fill("synthetic-api-key-not-real");
        await editor
          .getByRole("button", { name: "显示密钥", exact: true })
          .click();
        await expect(key).toHaveAttribute("type", "text");
        await expect(key).not.toHaveCSS("-webkit-text-security", "disc");
        await editor
          .getByRole("button", { name: "隐藏密钥", exact: true })
          .click();
        await expect(key).toHaveCSS("-webkit-text-security", "disc");
        await key.clear();
        await expect(key).toHaveAttribute("placeholder", "已配置，留空保留");
        await expect(editor.locator('input[type="password"]')).toHaveCount(0);
        await editor
          .getByRole("textbox", { name: "名称", exact: true })
          .focus();
      } else {
        await expect(editor.locator('input[type="password"]')).toHaveValue("");
      }
      if (scenario === "ai-http-auth")
        await expect(
          editor.getByRole("checkbox", {
            name: "移除已保存的代理认证",
            exact: true,
          }),
        ).not.toBeChecked();
    }
    if (scenario === "ai-transcription") {
      await settings
        .getByRole("link", { name: "语音转文字", exact: true })
        .click();
      await expect(page).toHaveURL(/#settings-ai-transcription$/);
      const feature = settings.getByRole("region", {
        name: "语音转文字",
        exact: true,
      });
      await feature.scrollIntoViewIfNeeded();
      await expect(
        feature.getByRole("combobox", { name: "模型", exact: true }),
      ).toBeVisible();
      await expect(
        feature.getByRole("combobox", { name: "思考强度", exact: true }),
      ).toHaveCount(0);
      await expect(
        feature.getByRole("combobox", { name: "图像精度", exact: true }),
      ).toHaveCount(0);
    }
    if (scenario === "ai-description") {
      const row = settings
        .getByRole("row")
        .filter({ has: page.getByRole("rowheader", { name: /^描述/ }) });
      await row.hover();
      const backfill = row.getByRole("button", { name: "补全", exact: true });
      await expect(backfill).toBeEnabled();
      await expect(
        settings.getByRole("checkbox", {
          name: "自动描述新增文件",
          exact: true,
        }),
      ).toBeChecked();
      await expect(row.getByText("空闲", { exact: true })).toBeVisible();
    }
    if (scenario === "ai-facts") {
      await expect(
        settings.getByRole("checkbox", {
          name: "自动提取新增文件的结构化信息",
          exact: true,
        }),
      ).not.toBeChecked();
      const row = settings
        .getByRole("row")
        .filter({ has: page.getByRole("rowheader", { name: /^结构化信息/ }) });
      await row.hover();
      const backfill = row.getByRole("button", { name: "补全", exact: true });
      await expect(backfill).toBeEnabled();
      await expect(row.getByText("空闲", { exact: true })).toBeVisible();
    }
    if (
      ["storage", "storage-mobile", "storage-dark", "client-storage"].includes(
        scenario,
      )
    ) {
      const mobile = scenario === "storage-mobile";
      if (client)
        await pairedFields("client.cache_max_age", "client.cache_max_bytes");
      else {
        await pairedFields("library.timezone", "library.language", mobile);
        await pairedFields(
          "previews.cache_max_age",
          "previews.cache_max_bytes",
          mobile,
        );
        await pairedFields(
          "video.cache_max_age",
          "video.cache_max_bytes",
          mobile,
        );
        const row = settings
          .locator('[data-setting-field="storage.link_mode"]')
          .locator("../..");
        const mode = await fieldBounds("storage.link_mode"),
          container = (await row.boundingBox())!;
        expect(mode.width).toBeLessThanOrEqual(container.width);
        if (!mobile) expect(mode.width).toBeLessThan(container.width * 0.6);
      }
    }
    if (scenario === "location") {
      await settings.getByRole("link", { name: "GPS", exact: true }).click();
      const endpoint = await fieldBounds("geocoding.endpoint"),
        interval = await fieldBounds("geocoding.request_interval_seconds");
      expect(interval.width).toBeLessThan(endpoint.width * 0.6);
      await expect(
        settings.locator("#settings-location-title"),
      ).toBeInViewport();
    }
    if (scenario === "service") {
      await pairedFields("server.max_upload_bytes", "server.max_upload_files");
      const listenBounds = await fieldBounds("server.listen"),
        originsBounds = await fieldBounds("server.allowed_origins");
      expect(listenBounds.width).toBeLessThan(originsBounds.width * 0.6);
      await settings.getByRole("link", { name: "服务", exact: true }).click();
      const origins = settings.getByRole("textbox", {
        name: "允许使用的访问网址",
        exact: true,
      });
      const field = settings.locator(".setting-field").filter({
        has: page.getByRole("textbox", {
          name: "允许使用的访问网址",
          exact: true,
        }),
      });
      await origins.fill("https://temporary.example");
      await origins.fill("https://archive.example");
      await expect(
        settings.getByRole("button", { name: "保存", exact: true }),
      ).toBeDisabled();
      const reset = field.getByRole("button", {
        name: "恢复默认",
        exact: true,
      });
      await expect(reset).toBeEnabled();
      const specified = field.getByText("已显式指定", { exact: true });
      await expect(specified).toBeVisible();
      await expect(field.locator(".setting-field-footer")).toHaveCount(0);
      const defaultField = settings.locator(".setting-field").filter({
        has: page.getByRole("textbox", {
          name: "单次上传体积上限",
          exact: true,
        }),
      });
      await expect(
        defaultField.getByRole("button", { name: "恢复默认", exact: true }),
      ).toHaveCount(0);
      const inherited = defaultField.locator(".setting-source--default");
      await expect(inherited).toHaveCSS("opacity", "0");
      await defaultField.hover();
      await expect(inherited).toHaveCSS("opacity", "1");
      await expect(
        defaultField.getByText("已跟随默认", { exact: true }),
      ).toBeVisible();
      // Moving the pointer does not blur the origins editor. Focus keeps its
      // reset action visible until the user clicks elsewhere.
      await expect(field.locator(".setting-reset")).toHaveCSS("opacity", "1");
      await settings
        .getByRole("heading", { name: "设置", exact: true, level: 1 })
        .click();
      await page.mouse.move(0, 0);
      await expect(inherited).toHaveCSS("opacity", "0");
      await expect(field.locator(".setting-reset")).toHaveCSS("opacity", "0");
      await expect(specified).toBeVisible();
      await expect(
        defaultField.getByText("已显式指定", { exact: true }),
      ).toHaveCount(0);
      await origins.hover();
      await expect(field.locator(".setting-reset")).toHaveCSS("opacity", "1");
      const labelBox = await field
        .locator(".setting-field-heading label")
        .boundingBox();
      const resetBox = await reset.boundingBox();
      expect(
        Math.abs(
          labelBox!.y +
            labelBox!.height / 2 -
            resetBox!.y -
            resetBox!.height / 2,
        ),
      ).toBeLessThan(1);

      await expect(
        settings.getByRole("heading", { name: "设置", exact: true }),
      ).toBeInViewport();
      await expect(
        settings.getByRole("button", { name: "保存", exact: true }),
      ).toBeInViewport();
      await expect(
        settings.getByRole("button", { name: "重新加载设置", exact: true }),
      ).toBeInViewport();
      await expect(settings).toContainText("127.0.0.1:19000");
      await expect(settings).toContainText("重启");
    }
    if (scenario === "credentials-empty") {
      await settings
        .getByRole("link", { name: "访问凭据", exact: true })
        .click();
      const access = settings.locator("#settings-access");
      await expect(access.getByRole("table")).toHaveCount(0);
      await expect(
        access.getByText("暂无访问凭据", { exact: true }),
      ).toBeVisible();
      await expect(
        access.getByRole("button", { name: "创建凭据", exact: true }),
      ).toBeVisible();
    }
    if (scenario === "credentials-create") {
      await settings
        .getByRole("link", { name: "访问凭据", exact: true })
        .click();
      await expect(
        settings.getByRole("textbox", { name: "备注（可选）", exact: true }),
      ).toHaveCount(0);
      await settings
        .getByRole("button", { name: "创建凭据", exact: true })
        .click();
      await expect(
        settings.getByRole("textbox", { name: "备注（可选）", exact: true }),
      ).toBeFocused();
      await settings
        .getByRole("textbox", { name: "备注（可选）", exact: true })
        .fill("旅行备用机");
      await expect(
        settings.getByRole("button", { name: "保存", exact: true }),
      ).toBeEnabled();
      await expect(settings.locator(".setting-secret")).toHaveCount(0);
    }
    if (scenario === "credentials") {
      await page.goto("/settings#settings-service");
      for (let attempt = 0; attempt < 3; attempt++) {
        await expect
          .poll(async () => {
            const header = await page
              .locator(".settings-page-header")
              .boundingBox();
            const title = await page
              .locator("#settings-service-title")
              .boundingBox();
            return header && title ? title.y - header.y - header.height : -1;
          })
          .toBeGreaterThan(16);
        await page.reload();
      }
      await page.goto("/settings#settings-access");
      await expect(settings.locator("#settings-access-title")).toBeInViewport();
      await page.reload();
      await expect(settings.locator("#settings-access-title")).toBeInViewport();
      const help = settings.getByRole("button", {
        name: "访问凭据",
        exact: true,
      });
      await help.hover();
      await expect(page.getByRole("tooltip")).toContainText("用于连接此实例");
      await page.mouse.move(0, 0);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await expect(settings).toContainText("旅行备用机");
      await expect(
        settings.getByRole("cell", {
          name: "0123456789abcdef0123456789abcdef",
          exact: true,
        }),
      ).toBeVisible();
      await expect(settings).toContainText("未填写备注");
      await settings
        .getByRole("button", { name: "编辑备注", exact: true })
        .first()
        .click();
      await expect(
        settings
          .getByRole("textbox", { name: "备注（可选）", exact: true })
          .first(),
      ).toHaveValue("旅行备用机");
      await expect(
        settings
          .locator("#settings-access")
          .getByRole("button", { name: "完成", exact: true }),
      ).toBeVisible();
      await expect(
        settings.getByRole("button", { name: "保存", exact: true }),
      ).toBeDisabled();
    }
    if (scenario === "maps") {
      await settings.getByRole("link", { name: "地图", exact: true }).click();
      await expect(
        settings.getByRole("heading", { name: "地图", exact: true }),
      ).toBeInViewport();
    }
    if (scenario === "hint") {
      await settings.getByRole("link", { name: "服务", exact: true }).click();
      await settings
        .getByRole("button", { name: "允许使用的访问网址", exact: true })
        .focus();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press("Tab");
      await expect(page.getByRole("tooltip")).toContainText(
        "用于打开 kist 的网址",
      );
    }
    if (client || scenario.startsWith("storage")) {
      await settings.getByRole("link", { name: "存储", exact: true }).click();
      const usage = settings.getByRole("region", { name: "本机存储用量" });
      await expect(usage.locator(".storage-usage-values > div")).toHaveCount(4);
      await expect(usage).toContainText("原文件");
    }
    if (scenario.startsWith("ai-")) {
      if (["ai-loading", "ai-error", "ai-empty"].includes(scenario))
        await page
          .getByRole("region", { name: "OCR", exact: true })
          .getByRole("combobox", { name: "模型", exact: true })
          .click();
      await expect(
        page.getByRole("checkbox", { name: "自动识别新增图片" }),
      ).toBeVisible();
      if (scenario === "ai-loading")
        await expect(
          page.getByText("正在加载模型列表…", { exact: true }),
        ).toBeVisible();
      if (scenario === "ai-error")
        await expect(page.getByText(/Model service unavailable/)).toBeVisible();
      if (scenario === "ai-empty")
        await expect(
          page.getByText("当前账号没有可用模型。", { exact: true }),
        ).toBeVisible();
      if (scenario === "ai-paused") {
        await expect(
          page.getByRole("checkbox", { name: "自动识别新增图片" }),
        ).toBeChecked();
        await expect(
          page
            .getByRole("row")
            .filter({ has: page.getByRole("rowheader", { name: /^OCR/ }) })
            .getByRole("button", { name: "继续", exact: true }),
        ).toBeEnabled();
      }
    }
    if (["ai-description", "ai-facts", "ai-paused"].includes(scenario))
      await settings
        .getByRole("link", { name: "后台任务", exact: true })
        .click();
    await capture(
      page,
      errors,
      info.title,
      scenario === "ai-error"
        ? ["Model service unavailable"]
        : scenario === "ai-proxy-test-error"
          ? ["测试超过 15 秒，请检查代理或稍后重试。"]
          : scenario === "ai-account-test-error"
            ? ["认证失败，请检查 API Key 或重新登录。"]
            : scenario === "ai-paused"
              ? ["上次失败：任务已取消。"]
              : [],
    );
  });
}

for (const variant of ["light", "dark", "mobile"] as const) {
  test(`file-more-actions-${variant}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mode: variant === "dark" ? "dark" : "light",
      mobile: variant === "mobile",
    });
    await page
      .context()
      .grantPermissions(["clipboard-read", "clipboard-write"]);
    await page.goto("/files");
    await grid(page, variant === "mobile" ? 2 : 4);
    await page.getByRole("button", { name: /山间清晨.jpg/ }).click();
    const gallery = page.getByRole("region", { name: "画廊", exact: true });
    await expect(gallery).toBeVisible();
    const readable =
      "/boxes/2026-01-01_" +
      encodeURIComponent(box.name) +
      "/" +
      encodeURIComponent(files[0].name);
    await expect(page).toHaveURL(new RegExp(readable + "$"));
    const trigger = gallery.getByRole("button", {
      name: "更多操作",
      exact: true,
    });
    await trigger.focus();
    await page.keyboard.press("Enter");
    await expect(
      page.getByRole("menuitem", { name: "在新标签页打开图片" }),
    ).toHaveAttribute("target", "_blank");
    await expect(
      page.getByRole("menuitem", { name: "下载原文件", exact: true }),
    ).toHaveAttribute("href", "/api/v1/files/file-1/content?download=true");
    await page.getByRole("menuitem", { name: "复制链接", exact: true }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
      new URL(readable, page.url()).href,
    );
    await page
      .getByRole("menuitem", { name: "复制永久链接", exact: true })
      .click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
      new URL("/files?file=file-1", page.url()).href,
    );
    await capture(page, errors, info.title);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    await expect(gallery).toBeVisible();
    await expect(trigger).toBeFocused();
    await trigger.click();
    await page
      .getByRole("menuitem", { name: "切换到永久链接", exact: true })
      .click();
    await expect(page).toHaveURL(/\/files\?file=file-1$/);
    await page.reload();
    await expect(
      gallery.getByRole("heading", { name: "山间清晨.jpg", exact: true }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/files\?file=file-1$/);
    expect(errors).toEqual([]);
  });
}

test("catalog-external-changes", async ({ page }, info) => {
  const errors = await prepare(page);
  let revision = "before";
  let name = "外部新增文件.txt";
  let added = false;
  await page.route("**/api/v1/catalog-revision", (route) =>
    route.fulfill({ json: { revision } }),
  );
  await page.route(/\/api\/v1\/(files|entries)\?/, (route) =>
    route.fulfill({
      json: {
        files: added
          ? [...files, { ...files[0], id: "visible-99", name }]
          : files,
        cursor: "",
      },
    }),
  );
  await page.goto("/files");
  await grid(page);
  await page.getByRole("button", { name: "归档盒", exact: true }).click();
  await page
    .getByRole("button", { name: /山海拾光/ })
    .first()
    .click();
  await grid(page);
  added = true;
  revision = "after-navigation";
  await page.getByRole("button", { name: "全部文件", exact: true }).click();
  await expect(
    page.getByRole("button", { name: /外部新增文件.txt/ }),
  ).toBeVisible();
  name = "后台更新文件.txt";
  revision = "after-poll";
  await expect(
    page.getByRole("button", { name: /后台更新文件.txt/ }),
  ).toBeVisible({ timeout: 15000 });
  await expect(
    page.getByRole("button", { name: /外部新增文件.txt/ }),
  ).toHaveCount(0);
  await capture(page, errors, info.title);
});

for (const scope of ["history", "trash"] as const) {
  test(`offline-pending-${scope}`, async ({ page }, info) => {
    const errors = await prepare(page);
    await page.route("**/api/v1/info", (route) =>
      route.fulfill({
        json: {
          client: true,
          version: "visual",
          timezone: "UTC",
          projection_dirty: false,
        },
      }),
    );
    await page.route("**/api/v1/client/connection-status", (route) =>
      route.fulfill({ json: { state: "unreachable", pending_upload: 4 } }),
    );
    await page.route("**/api/v1/client/status", (route) =>
      route.fulfill({ json: { catalog: { complete: true }, pending: [] } }),
    );
    await page.route("**/api/v1/ops?**", (route) =>
      route.fulfill({ json: { ops: [] } }),
    );
    await page.route("**/api/v1/client/pending-history?**", (route) =>
      route.fulfill({
        json: {
          entries: [
            {
              id: "local-delete",
              action: "delete_entry_contents",
              occurred_at: time,
              state: "pending",
              origin: "user",
              summary: { name: "旅行记录.pdf", count: 1 },
            },
            ...(scope === "history"
              ? [
                  {
                    id: "local-upload",
                    action: "import_files",
                    occurred_at: time,
                    state: "pending",
                    origin: "user",
                    summary: { name: "新照片.jpg", count: 1 },
                  },
                ]
              : []),
          ],
        },
      }),
    );
    await page.goto(`/${scope}`);
    await expect(page.locator(".history-entry")).toHaveCount(
      scope === "trash" ? 1 : 2,
    );
    await expect(
      page.getByText("待上传 4 个操作", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText(/待下载.*个操作/)).toHaveCount(0);
    if (scope === "trash")
      await expect(
        page.getByRole("button", { name: "恢复", exact: true }),
      ).toBeVisible();
    await capture(page, errors, info.title);
  });
}

for (const scenario of [
  "ready",
  "disconnected",
  "error",
  "text",
  "text-tooltip",
  "empty",
] as const) {
  test(`ocr-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page);
    let descriptionDone = false;
    if (scenario === "text-tooltip") {
      await page.route("**/api/v1/files/file-1/desc", (route) =>
        route.fulfill({
          json: { state: descriptionDone ? "ready" : "running" },
        }),
      );
    }
    await page.route("**/api/v1/ai", (route) =>
      route.fulfill({
        json: {
          supported: true,
          connected: scenario !== "disconnected",
          email: "reader@example.test",
          pending: false,
          error: "",
        },
      }),
    );
    await page.route("**/api/v1/ai/accounts/*/models", (route) =>
      route.fulfill({ json: [{ id: "gpt-6-luna", name: "GPT-6 Luna" }] }),
    );
    await page.route("**/api/v1/ai/services", (route) =>
      route.fulfill({ json: visualAIServices(scenario !== "disconnected") }),
    );
    if (scenario === "error")
      await page.route("**/api/v1/files/file-1/ocr", (route) =>
        route.fulfill({
          json: {
            state: "failed",
            manual: true,
            error: "OpenAI did not complete OCR; no result was saved",
          },
        }),
      );
    if (scenario.startsWith("text") || scenario === "empty")
      await page.route("**/api/v1/files/file-1", (route) =>
        route.fulfill({
          json: {
            file: files[0],
            tags: ["tag-2"],
            extracted: ["ocr"],
            metadata: [
              ...(descriptionDone
                ? [
                    {
                      key: "desc:text",
                      type: "text",
                      source: "desc",
                      value:
                        "异步完成的图片描述，用于验证布局变化后的悬停提示。",
                    },
                  ]
                : []),
              {
                key: "ocr:text",
                type: "text",
                source: "ocr",
                value:
                  scenario === "empty"
                    ? ""
                    : "档案收据\n姓名：张三\n编号：2026-001\n保存日期：2026 年 10 月 25 日\n\n" +
                      "这是完整保存的识别文字，用于核对长文本的换行与滚动。\n".repeat(
                        25,
                      ),
              },
              {
                key: "ocr:model",
                type: "text",
                source: "ocr",
                value: "openai/gpt-6-luna",
              },
              { key: "ocr:recipe", type: "text", source: "ocr", value: "1" },
            ],
          },
        }),
      );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    if (scenario === "empty") {
      await expect(
        page.getByText("未识别到文字。", { exact: true }),
      ).toBeVisible();
      await expect(page.getByRole("textbox", { name: "识别文字" })).toHaveCount(
        0,
      );
    } else if (scenario.startsWith("text")) {
      await expect(
        page.getByRole("heading", { name: "OCR", exact: true }),
      ).toBeVisible();
      await expect(
        page.getByText("openai/gpt-6-luna", { exact: true }),
      ).toBeVisible();
      await expect(page.getByText("服务商", { exact: true })).toHaveCount(0);
      await expect(page.getByText("源文件校验值", { exact: true })).toHaveCount(
        0,
      );
      await expect(page.locator(".copy-text-preview")).toContainText(
        "档案收据↵ 姓名：张三↵ ",
      );
      const layout = await page
        .locator(".copy-text-preview")
        .evaluate((element) => {
          const lineHeight = parseFloat(getComputedStyle(element).lineHeight);
          const bounds = element.getBoundingClientRect();
          const action = element
            .querySelector(".copy-text-action")!
            .getBoundingClientRect();
          return {
            lines: bounds.height / lineHeight,
            lastLine: bounds.bottom - action.bottom < lineHeight / 2,
            rightAligned: Math.abs(bounds.right - action.right) < 1,
          };
        });
      expect(layout.lines).toBeLessThanOrEqual(3.05);
      expect(layout.lastLine).toBe(true);
      expect(layout.rightAligned).toBe(true);
    } else {
      await expect(
        page.getByRole("button", { name: "编辑补充信息", exact: true }),
      ).toHaveCount(0);
      const action = page.getByRole("button", {
        name: scenario === "error" ? "重试" : "提取文字（OCR）",
        exact: true,
      });
      if (scenario === "disconnected") {
        await expect(action).toHaveCount(0);
      } else await expect(action).toBeEnabled();
      if (scenario === "error")
        await expect(page.getByRole("alert")).toContainText(
          "no result was saved",
        );
      await expect(
        page
          .locator(".file-metadata-group")
          .filter({
            has: page.getByRole("heading", { name: "OCR", exact: true }),
          })
          .getByRole("link", { name: "AI 设置", exact: true }),
      ).toHaveCount(scenario === "disconnected" ? 1 : 0);
    }
    if (scenario === "text-tooltip") {
      await page.locator(".copy-text-preview bdi").hover();
      const tip = page.getByRole("tooltip");
      await expect(tip).toBeVisible();
      expect(await tip.textContent()).toContain("档案收据\n姓名：张三\n");
      expect(await tip.evaluate((el) => getComputedStyle(el).whiteSpace)).toBe(
        "pre-wrap",
      );
      expect(
        await tip.evaluate((el) => getComputedStyle(el).backgroundColor),
      ).toBe("rgba(33, 37, 41, 0.85)");
      // The line gap belongs to the preview block, not the inline text.
      const preview = page.locator(".copy-text-preview");
      const gap = await preview.evaluate((el) => {
        const block = el.getBoundingClientRect();
        const line = el.querySelector("bdi")!.getClientRects()[0];
        return { x: 2, y: line.bottom - block.top + 1 };
      });
      await preview.hover({ position: gap });
      await expect(tip).toBeVisible();
      await expect
        .poll(async () => (await tip.boundingBox())?.y ?? 0)
        .toBeGreaterThan(0);
    }
    await capture(
      page,
      errors,
      info.title,
      scenario === "error" ? ["no result was saved"] : [],
    );

    if (scenario === "text-tooltip") {
      descriptionDone = true;
      await expect(
        page
          .locator(".copy-text-preview")
          .getByText("异步完成的图片描述，用于验证布局变化后的悬停提示。", {
            exact: true,
          }),
      ).toBeVisible();
      await page.mouse.move(5, 5);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      const ocrPreview = page
        .locator(".file-metadata-group")
        .filter({
          has: page.getByRole("heading", { name: "OCR", exact: true }),
        })
        .locator(".copy-text-preview");
      await ocrPreview.hover();
      await expect(page.getByRole("tooltip")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await page.mouse.move(5, 5);
      await ocrPreview.focus();
      await expect(page.getByRole("tooltip")).toBeVisible();
      await ocrPreview.evaluate((el) => el.blur());
      await expect(page.getByRole("tooltip")).toHaveCount(0);
    }
  });
}

for (const mode of ["light", "dark"] as const) {
  test(`navigation-missing-file-${mode}`, async ({ page }, info) => {
    const errors = await prepare(page, { mode });
    await page.route("**/api/v1/navigation?*", (route) =>
      route.fulfill({
        status: 404,
        json: { code: "not_found", class: "not_found", message: "Not found" },
      }),
    );
    await page.goto("/boxes/missing/photo.png");
    await expect(page.getByRole("alert")).toContainText("此项已不存在");
    const home = page.getByRole("link", { name: "回到首页" });
    await expect(home).toHaveAttribute("href", "/");
    await capture(page, errors, info.title, ["此项已不存在"]);
    await home.click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole("alert")).toHaveCount(0);
  });
}

for (const scenario of [
  "text",
  "text-tooltip",
  "text-refresh",
  "text-paused",
  "text-queued",
  "background-queued",
  "ready",
  "unclassified",
  "error",
  "disconnected",
] as const) {
  test(`description-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      aiConnected: scenario !== "disconnected",
    });
    if (scenario === "error")
      await page.route("**/api/v1/files/file-1/desc", (route) =>
        route.fulfill({
          json: {
            state: "failed",
            manual: true,
            error: "Description was interrupted; retry explicitly",
          },
        }),
      );
    if (scenario === "unclassified")
      await page.route("**/api/v1/files/file-1", (route) =>
        route.fulfill({
          json: {
            file: {
              ...files[0],
              media_type: "",
              kind: "",
              extraction_pending: ["media"],
            },
            tags: [],
            extracted: [],
            metadata: [],
          },
        }),
      );
    if (scenario.startsWith("text"))
      await page.route("**/api/v1/files/file-1", (route) =>
        route.fulfill({
          json: {
            file: files[0],
            tags: [],
            extracted: ["ocr", "desc"],
            metadata: [
              {
                key: "desc:text",
                type: "text",
                source: "desc",
                value:
                  "一张白底的测试文档，列有姓名、文档编号以及生效和到期日期。\n页面注明这些内容仅为合成测试数据。",
              },
              {
                key: "desc:model",
                type: "text",
                source: "desc",
                value: "openai/gpt-6-luna",
              },
              { key: "desc:recipe", type: "int", source: "desc", value: "1" },
            ],
          },
        }),
      );
    let refreshRequested = false;
    if (scenario === "text-refresh") {
      await page.route("**/api/v1/files/file-1/desc", (route) => {
        if (route.request().method() === "POST") {
          const body = route.request().postDataJSON();
          expect(body.force).toBe(true);
          expect(body.retry).toBe(false);
          expect(body.request_id).toMatch(/^[a-f0-9-]{36}$/);
          refreshRequested = true;
        }
        return route.fulfill({
          json: refreshRequested
            ? {
                state: "failed",
                error: "Refresh failed; previous description retained",
              }
            : { state: "ready" },
        });
      });
    }
    if (
      scenario === "text-paused" ||
      scenario === "text-queued" ||
      scenario === "background-queued"
    ) {
      await page.route("**/api/v1/ai/desc", (route) =>
        route.fulfill({
          json: {
            paused: scenario === "text-paused",
            queued: 1,
            failed: 0,
            scanning: false,
            running: "",
          },
        }),
      );
      await page.route("**/api/v1/files/file-1/desc", (route) => {
        if (route.request().method() === "POST") {
          expect(route.request().postDataJSON().force).toBe(true);
          refreshRequested = true;
        }
        return route.fulfill({
          json: { state: "queued", manual: refreshRequested },
        });
      });
    }
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page.locator(".file-metadata-group").filter({
      has: page.getByRole("heading", { name: "描述", exact: true }),
    });
    if (scenario === "background-queued") {
      await expect(group).toHaveCount(0);
      await capture(page, errors, info.title);
      return;
    }
    await expect(group).toBeVisible();
    if (scenario.startsWith("text")) {
      await expect(
        group.locator("dt bdi").filter({ hasText: /^正文$/ }),
      ).toBeVisible();
      await expect(
        group.getByRole("button", { name: "复制描述", exact: true }),
      ).toBeVisible();
      await expect(group).toContainText("openai/gpt-6-luna");
      await expect(group.getByText("生成语言", { exact: true })).toHaveCount(0);
      await expect(group.getByText("服务商", { exact: true })).toHaveCount(0);
      expect(
        await group
          .locator(".copy-text-preview")
          .evaluate(
            (el) =>
              el.getBoundingClientRect().height /
              parseFloat(getComputedStyle(el).lineHeight),
          ),
      ).toBeLessThanOrEqual(3.05);
    } else {
      const action = group.getByRole("button", {
        name: scenario === "error" ? "重试" : "生成描述",
        exact: true,
      });
      if (scenario === "disconnected") {
        await expect(action).toHaveCount(0);
      } else await expect(action).toBeEnabled();
    }
    if (scenario === "text-paused") {
      const action = group.getByRole("button", {
        name: "重新提取描述",
        exact: true,
      });
      await expect(group.getByText(/已暂停/)).toHaveCount(0);
      await expect(
        group.locator(".metadata-group-heading").getByRole("button"),
      ).toHaveCount(1);
      await expect(group.getByText("已排队", { exact: true })).toHaveCount(0);
      await expect(group.getByRole("status")).toHaveCount(0);
      await expect(action).toBeEnabled();
      await action.click();
      await expect(action).toBeDisabled();
      await expect(action).toContainText("已排队");
      expect(refreshRequested).toBe(true);
    }
    if (scenario === "text-queued") {
      const action = group.getByRole("button", {
        name: "重新提取描述",
        exact: true,
      });
      await expect(action).toBeDisabled();
      await expect(action).toHaveText("已排队");
      await expect(
        group.locator(".metadata-group-heading").getByRole("button"),
      ).toHaveCount(1);
      await expect(group.getByRole("status")).toHaveCount(0);
    }
    if (scenario === "text-tooltip") {
      await group.locator(".copy-text-preview bdi").hover();
      const tip = page.getByRole("tooltip");
      await expect(tip).toBeVisible();
      expect(await tip.textContent()).toContain("日期。\n页面");
      await expect
        .poll(async () => (await tip.boundingBox())?.y ?? 0)
        .toBeGreaterThan(0);
    }
    if (scenario === "text-refresh") {
      await group.hover();
      await group
        .getByRole("button", { name: "重新提取描述", exact: true })
        .click();
      await expect(group.getByRole("alert")).toContainText("Refresh failed");
      await expect(
        group.getByRole("button", { name: "重新提取描述", exact: true }),
      ).toBeEnabled();
      await expect(group.locator(".copy-text-preview")).toContainText(
        "一张白底的测试文档",
      );
    }
    if (scenario === "error" || scenario === "text-refresh") {
      await expect(group.getByRole("alert")).toHaveAttribute(
        "style",
        /--text-fz: var\(--mantine-font-size-xs\)/,
      );
    }
    await capture(
      page,
      errors,
      info.title,
      scenario === "error"
        ? ["Description was interrupted"]
        : scenario === "text-refresh"
          ? ["Refresh failed; previous description retained"]
          : [],
    );
  });
}

for (const refresh of [false, true]) {
  test(
    refresh ? "metadata-reextract" : "detail-exif-labels",
    async ({ page }, info) => {
      const errors = await prepare(page);
      await page.route("**/api/v1/files/file-1", (route) =>
        route.fulfill({
          json: {
            file: files[0],
            tags: [],
            extracted: ["file", "media", "exif"],
            metadata: [
              ["exif:DateTimeOriginal", "2026:01:02 12:00:00"],
              ["exif:ExposureTime", "1/125"],
              ["exif:FNumber", "28/10"],
              ["exif:ISO", "100"],
              ["exif:FocalLength", "50/1"],
              ["exif:LensModel", "Example 50mm"],
              ["exif:WhiteBalance", "0"],
              ["gps:LatitudeRef", "N"],
              ["exif:UnknownVendorTag", "Original value"],
            ].map(([key, value]) => ({
              key,
              value,
              type: "text",
              source: "exif",
            })),
          },
        }),
      );
      let requests = 0;
      await page.route("**/api/v1/files/file-1/metadata", (route) => {
        if (route.request().method() === "GET")
          return route.fulfill({ json: { state: "ready" } });
        expect(route.request().postDataJSON()).toMatchObject({
          force: true,
          extractor: "exif",
        });
        requests++;
        return route.fulfill({ json: { state: "ready", manual: true } });
      });
      await page.goto("/files?file=file-1");
      await page.getByRole("button", { name: "文件详情", exact: true }).click();
      const group = page.locator(".file-metadata-group").filter({
        has: page.getByRole("heading", { name: "EXIF", exact: true }),
      });
      for (const label of [
        "拍摄时间",
        "曝光时间",
        "光圈系数",
        "ISO 感光度",
        "焦距",
        "镜头型号",
        "白平衡",
        "UnknownVendorTag",
      ]) {
        await expect(group.getByText(label, { exact: true })).toBeVisible();
      }
      const gps = page.locator(".file-metadata-group").filter({
        has: page.getByRole("heading", { name: "GPS", exact: true }),
      });
      await expect(gps.getByText("南北纬", { exact: true })).toBeVisible();
      await expect(group.getByText("南北纬", { exact: true })).toHaveCount(0);
      await expect(gps.getByText("曝光时间", { exact: true })).toHaveCount(0);
      await expect(
        group.getByRole("button", {
          name: "编辑UnknownVendorTag",
          exact: true,
        }),
      ).toHaveCount(1);
      await group.getByText("UnknownVendorTag", { exact: true }).hover();
      await expect(page.getByRole("tooltip")).toHaveText(
        "exif:UnknownVendorTag",
      );
      await page.mouse.move(0, 0);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await group.getByText("曝光时间", { exact: true }).hover();
      await expect(page.getByRole("tooltip")).toHaveText("exif:ExposureTime");
      await page.mouse.move(0, 0);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      if (refresh) {
        const button = group.getByRole("button", { name: "重新提取EXIF" });
        await expect(button).toHaveCSS("opacity", "0");
        await group.hover();
        await expect(button).toHaveCSS("opacity", "1");
        // Positioning must not be replaced by Mantine's pressed transform.
        const pressedOffset = async (target: typeof button) => {
          await target.hover();
          const before = (await target.boundingBox())!;
          await page.mouse.down();
          const after = (await target.boundingBox())!;
          await page.mouse.move(0, 0);
          await page.mouse.up();
          return after.y - before.y;
        };
        const reference = await pressedOffset(
          page.getByRole("button", { name: "文件版本", exact: true }),
        );
        expect(await pressedOffset(button)).toBeCloseTo(reference, 5);
        expect(requests).toBe(0);
        await button.click();
        await expect.poll(() => requests).toBe(1);
        await expect(button).toBeEnabled();
        await expect(
          group.getByText("曝光时间", { exact: true }),
        ).toBeVisible();
        await button.focus();
        await page.mouse.move(0, 0);
        await expect(button).toHaveCSS("opacity", "1");
        await button.evaluate((el) => el.blur());
        await gps.hover();
        await expect(
          gps.getByRole("button", { name: "重新提取GPS" }),
        ).toHaveCSS("opacity", "1");
      }
      await capture(page, errors, info.title);
    },
  );
}

for (const manual of [false, true]) {
  test(`image-orientation-${manual ? "manual" : "automatic"}`, async ({
    page,
  }, info) => {
    const errors = await prepare(page);
    const file = {
      ...files[0],
      image_rotation: 90,
      image_rotation_manual: manual,
    };
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file,
          tags: [],
          extracted: ["file", "media", "orientation"],
          metadata: [
            {
              key: "orientation:clockwise_degrees",
              value: "90",
              type: "int",
              source: manual ? "user" : "orientation",
            },
            {
              key: "orientation:method",
              value: "overview",
              type: "text",
              source: "orientation",
            },
            {
              key: "orientation:confidence",
              value: "0.94",
              type: "real",
              source: "orientation",
            },
          ],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/orientation", (route) =>
      route.fulfill({ json: { state: "ready" } }),
    );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    await expect(
      page.locator('img[src*="image_upright_v1_cw90"]'),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "图片方向", exact: true }),
    ).toBeVisible();
    const group = page.locator(".file-metadata-group").filter({
      has: page.getByRole("heading", { name: "图片方向", exact: true }),
    });
    await expect(group.getByText("方式", { exact: true })).toBeVisible();
    await group.getByText("置信度", { exact: true }).hover();
    await expect(page.getByRole("tooltip")).toHaveText(
      "orientation:confidence",
    );
    await page.mouse.move(0, 0);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await group.hover();
    await expect(
      group.getByRole("button", { name: "调整方向", exact: true }),
    ).toHaveCount(0);
    if (!manual)
      await expect(
        group.getByRole("button", { name: "重新提取图片方向", exact: true }),
      ).toBeVisible();
    if (manual) {
      await expect(
        page.getByRole("button", { name: "重新提取图片方向" }),
      ).toHaveCount(0);
      await page.getByRole("button", { name: "更多操作", exact: true }).click();
      await page
        .getByRole("menuitem", { name: "调整方向", exact: true })
        .click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await expect(
        page.getByRole("button", { name: "恢复 AI 判断", exact: true }),
      ).toBeVisible();
    }
    await capture(page, errors, info.title);
  });
}

for (const variant of [
  "desktop",
  "mobile",
  "failure",
  "help",
  "wheel",
] as const) {
  test(`gps-location-${variant}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      gps: true,
      mobile: variant === "mobile",
      mapFailure: variant === "failure",
    });
    // Wheel behavior needs a scrollable details panel, even with few fields.
    if (variant === "wheel")
      await page.setViewportSize({ width: 1280, height: 560 });
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file: files[0],
          tags: [],
          extracted: ["file", "media", "exif", "geo"],
          gps: { latitude: 12, longitude: 34, accuracy: 35 },
          metadata: [
            {
              key: "geo:address",
              value: "示例市 · 示例片区 · 示例街道",
              type: "text",
              source: "geo",
            },
            {
              key: "geo:provider",
              value: "photon",
              type: "text",
              source: "geo",
            },
            { key: "gps:Latitude", value: "12", type: "real", source: "exif" },
            { key: "gps:Longitude", value: "34", type: "real", source: "exif" },
          ],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/geo", (route) =>
      route.fulfill({ json: { state: "ready" } }),
    );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page.locator(".file-metadata-group").filter({
      has: page.getByRole("heading", { name: "GPS", exact: true }),
    });
    await group.scrollIntoViewIfNeeded();
    const headingColor = await group
      .getByRole("heading", { name: "GPS", exact: true })
      .evaluate((el) => getComputedStyle(el).color);
    await expect(
      group.getByRole("button", { name: "GPS", exact: true }),
    ).toHaveCSS("color", headingColor);
    await expect(
      group.getByText("示例市 · 示例片区 · 示例街道附近", { exact: true }),
    ).toBeVisible();
    if (variant === "failure")
      await expect(group.locator(".gpx-fallback circle")).toBeVisible();
    else {
      await expect(group.locator("canvas")).toBeVisible();
      await expect(group.locator(".gpx-status")).toHaveCount(0);
    }
    await expect(
      group.getByRole("link", {
        name: /OpenStreetMap/,
        exact: true,
      }),
    ).toBeVisible();
    if (variant === "wheel") {
      const panel = page.locator("#gallery-details");
      await group.locator("canvas").hover();
      await expect
        .poll(() => panel.evaluate((el) => el.scrollTop))
        .toBeGreaterThan(0);
      const before = await panel.evaluate((el) => el.scrollTop);
      await page.mouse.wheel(0, -160);
      await expect
        .poll(() => panel.evaluate((el) => el.scrollTop))
        .toBeLessThan(before);
      await group.scrollIntoViewIfNeeded();
    }
    if (variant === "help") {
      await group.getByRole("button", { name: "GPS", exact: true }).hover();
      await expect(page.getByRole("tooltip")).toContainText(
        "圆圈表示记录的定位误差",
      );
    } else await page.mouse.move(0, 0);
    await capture(page, errors, info.title);
  });
}

for (const anchor of [
  "location",
  "background-tasks",
  "background-tasks-expanded",
  "background-tasks-expanded-dark",
  "background-tasks-expanded-mobile",
] as const) {
  test(`settings-${anchor}`, async ({ page }, info) => {
    const taskSection = anchor.startsWith("background-tasks");
    const expanded = anchor.includes("expanded");
    if (anchor.endsWith("mobile"))
      await page.setViewportSize({ width: 390, height: 844 });
    const errors = await prepare(page, {
      mode: anchor.endsWith("dark") ? "dark" : "light",
    });
    await page.route("**/api/v1/settings", (route) =>
      route.fulfill({
        json: {
          revision: "visual",
          mode: "library",
          id: "instance",
          library_id: "library",
          credential_configured: false,
          fields: [],
          effective: {},
          overrides: [],
          restart_required: [],
        },
      }),
    );
    await page.route("**/api/v1/settings/storage", (route) =>
      route.fulfill({
        json: {
          originals_bytes: 1024,
          metadata_bytes: 512,
          previews_bytes: 256,
          video_bytes: 0,
        },
      }),
    );
    await page.route("**/api/v1/settings/credentials", (route) =>
      route.fulfill({ json: [] }),
    );
    if (taskSection)
      await page.route("**/api/v1/ai/ocr", (route) =>
        route.fulfill({
          json: {
            paused: true,
            scanning: true,
            queued: 1,
            failed: expanded ? 1 : 0,
            running: "",
            error: "context canceled",
          },
        }),
      );
    if (expanded)
      await page.route("**/api/v1/ai/ocr/jobs?*", (route) =>
        route.fulfill({
          json: {
            items: [
              {
                file_id: files[0].id,
                request_id: "synthetic-queued",
                name: "申请材料扫描.pdf",
                state: "queued",
                file_available: true,
                box: { name: "申请材料", start_at: "2026-01" },
              },
              {
                file_id: files[1].id,
                request_id: "synthetic-failed",
                name: "一份名称比较长的证书照片与相关附件.png",
                state: "failed",
                error: "context canceled",
                file_available: true,
                box: { name: "考试与证书", start_at: "2025-09-12T14:30" },
              },
            ],
            next_cursor: "",
          },
        }),
      );
    await page.goto(
      `/settings#settings-${taskSection ? "background-tasks" : "location"}`,
    );
    if (taskSection) {
      const tasks = page.locator("#settings-background-tasks");
      await expect(tasks.getByRole("rowheader")).toHaveCount(7);
      const ocr = tasks
        .getByRole("row")
        .filter({ has: page.getByRole("rowheader", { name: /^OCR/ }) });
      await expect(ocr.getByRole("status")).toHaveText("已暂停");
      await expect(ocr.getByRole("alert")).toHaveCount(expanded ? 1 : 0);
      await expect(
        ocr.getByRole("button", { name: "重试", exact: true }),
      ).toBeDisabled();
      await expect(
        ocr.getByRole("button", { name: "继续", exact: true }),
      ).toBeEnabled();
      await expect(tasks.locator(".task-table-state").first()).toBeVisible();
      const actions = tasks.locator(".table-row-actions").first();
      const rowHelp = tasks.locator(".table-row-help").first();
      await page.mouse.move(5, 5);
      await expect(actions).toHaveCSS("opacity", "0");
      await expect(rowHelp).toHaveCSS("opacity", "0");
      await tasks.getByRole("row").nth(1).hover();
      await expect(actions).toHaveCSS("opacity", "1");
      await expect(rowHelp).toHaveCSS("opacity", "1");
      await expect(
        actions.getByRole("button", { name: "重试", exact: true }),
      ).toBeDisabled();
      await expect(
        actions.getByRole("button", { name: "重试", exact: true }),
      ).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
      await expect(
        actions.getByRole("button", { name: "暂停", exact: true }),
      ).toBeEnabled();
      await expect(
        tasks.getByRole("heading", { name: "后台任务", exact: true }),
      ).toBeInViewport();
      await tasks
        .getByRole("button", { name: "后台任务", exact: true })
        .hover();
      await expect(page.getByRole("tooltip")).toContainText("共用调度器");
      await expect(page.getByRole("tooltip")).toContainText("独立的按需执行器");
      await page.mouse.move(5, 5);
      const arrow = ocr.getByRole("button", {
        name: "展开OCR任务",
        exact: true,
      });
      await expect(arrow.locator("..")).toHaveCSS("opacity", "0");
      await arrow.focus();
      await expect(arrow.locator("..")).toHaveCSS("opacity", "1");
      if (expanded) {
        await ocr.getByRole("rowheader", { name: /^OCR/ }).click();
        await expect(tasks.locator(".task-table > tbody > tr")).toHaveCount(7);
        const details = ocr.getByRole("table", {
          name: "OCR任务",
          exact: true,
        });
        await expect(
          details.getByRole("link", { name: "申请材料扫描.pdf" }),
        ).toHaveAttribute("href", `/files?file=${files[0].id}`);
        await expect(details.locator("time").first()).toHaveText(" · 2026-01");
        await expect(details.locator(".task-table-message")).toHaveText(
          "任务已取消。",
        );
        await expect(
          ocr.getByRole("button", { name: "收起OCR任务", exact: true }),
        ).toHaveAttribute("aria-expanded", "true");
      }
      await page.mouse.move(5, 5);
      await capture(
        page,
        errors,
        info.title,
        expanded ? ["上次失败：任务已取消。"] : [],
      );
      return;
    }
    const section = page.locator("#settings-location");
    await expect(
      section.getByRole("button", { name: "保存 GPS 设置", exact: true }),
    ).toHaveCount(0);
    await expect(
      section.getByRole("button", { name: "重新加载 GPS 设置", exact: true }),
    ).toHaveCount(0);
    await expect(
      page
        .locator("#settings-ai")
        .getByRole("button", { name: "保存 AI 设置", exact: true }),
    ).toHaveCount(0);
    await expect(
      page
        .locator("#settings-ai")
        .getByRole("button", { name: "重新加载 AI 设置", exact: true }),
    ).toHaveCount(0);
    const autoField = section.locator(".setting-field").filter({
      has: page.getByRole("checkbox", {
        name: "自动查询新增文件的位置",
        exact: true,
      }),
    });
    await expect(
      section.getByRole("button", { name: "恢复默认", exact: true }),
    ).toHaveCount(0);
    await autoField.hover();
    await expect(
      autoField.getByText("已跟随默认", { exact: true }),
    ).toBeVisible();
    await expect(
      autoField.getByRole("button", { name: "恢复默认", exact: true }),
    ).toHaveCount(0);
    await autoField.getByRole("checkbox").uncheck();
    await expect(
      autoField.getByText("已显式指定", { exact: true }),
    ).toBeVisible();
    await autoField
      .getByRole("button", { name: "恢复默认", exact: true })
      .click();
    await expect(autoField.getByRole("checkbox")).toBeChecked();
    await expect(
      page.getByRole("button", { name: "保存", exact: true }),
    ).toBeDisabled();
    await expect(
      autoField.getByText("已显式指定", { exact: true }),
    ).toHaveCount(0);
    await expect(
      section.getByRole("checkbox", { name: "自动查询新增文件的位置" }),
    ).toBeChecked();
    await expect(
      section.getByRole("button", { name: "暂停处理", exact: true }),
    ).toHaveCount(0);
    await expect(
      section.getByText("从支持的图片中读取 GPS", { exact: false }),
    ).toHaveCount(0);

    await expect(
      section.getByRole("button", { name: "补全已有文件的位置", exact: true }),
    ).toHaveCount(0);
    await expect(
      section.getByRole("textbox", { name: "请求间隔（秒）", exact: true }),
    ).toHaveValue("2");
    await expect(
      section.getByRole("checkbox", {
        name: "查询失败后暂停后台处理",
        exact: true,
      }),
    ).toBeChecked();
    await expect(
      section.getByText("请求串行执行", { exact: false }),
    ).toHaveCount(0);
    const sectionHelp = section.getByRole("button", {
      name: "GPS",
      exact: true,
    });
    await sectionHelp.hover();
    await expect(page.getByRole("tooltip")).toContainText("仅发送经纬度");
    await expect(page.getByRole("tooltip")).toContainText(
      "没有 GPS 的文件会跳过",
    );
    await page.mouse.move(5, 5);
    await expect(page.getByRole("tooltip")).toHaveCount(0);

    await expect(section.getByRole("heading", { level: 3 })).toHaveCount(0);
    await expect(
      section.getByRole("textbox", { name: "Photon 接口地址", exact: true }),
    ).toHaveValue("https://photon.komoot.io/reverse");
    const help = section.getByRole("button", {
      name: "Photon 接口地址",
      exact: true,
    });
    const toggleHelp = section.getByRole("button", {
      name: "自动查询新增文件的位置",
      exact: true,
    });
    const iconBox = await toggleHelp.boundingBox();
    const labelBox = await section
      .locator(".setting-checkbox label")
      .first()
      .boundingBox();
    expect(iconBox).not.toBeNull();
    expect(labelBox).not.toBeNull();
    expect(
      Math.abs(
        iconBox!.y + iconBox!.height / 2 - labelBox!.y - labelBox!.height / 2,
      ),
    ).toBeLessThan(1);
    await expect(help).toHaveCSS("color", "rgb(134, 142, 150)");
    await help.hover();
    await expect(help).toHaveCSS("color", "rgb(134, 142, 150)");
    await expect(page.getByRole("tooltip")).toContainText("可使用公共服务");
    await page.mouse.move(5, 5);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await page.reload();
    await expect(
      section.getByRole("textbox", { name: "Photon 接口地址", exact: true }),
    ).toHaveValue("https://photon.komoot.io/reverse");
    await expect(
      section.getByRole("heading", { name: "GPS", exact: true }),
    ).toBeInViewport();
    await capture(page, errors, info.title);
  });
}

for (const variant of [
  "edit",
  "saved",
  "mobile",
  "dark",
  "error",
  "hover",
  "hover-short",
  "hover-single",
  "hover-single-user",
  "remove-confirm",
  "remove-mobile",
  "remove-dark",
  "remove-saved",
  "remove-error",
] as const) {
  test(`metadata-inline-${variant}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mobile: variant === "mobile" || variant === "remove-mobile",
      mode: variant === "dark" || variant === "remove-dark" ? "dark" : "light",
    });
    let value = "示例文档\n姓名：测试用户\n日期：2026-01-02";
    let source = "ocr";
    if (variant.startsWith("hover-single"))
      value = "这是用于验证复制位置的文本";
    if (variant === "hover-short") value = "短文字";
    if (variant === "hover-single-user") source = "user";
    let removed = false;
    let writes = 0;
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file: files[0],
          tags: [],
          extracted: ["file", "media", "exif", "ocr", "desc", "orientation"],
          metadata: [
            {
              key: "exif:Make",
              value: "示例相机",
              source: "user",
              type: "text",
            },
            ...(!removed
              ? [{ key: "ocr:text", value, source, type: "text" }]
              : []),
            {
              key: "desc:text",
              value: "用于验证编辑功能的合成文档。",
              source: "desc",
              type: "text",
            },
          ],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/metadata", (route) => {
      if (route.request().method() === "GET") return route.fallback();
      const body = route.request().postDataJSON();
      writes++;
      expect(body.before_value).toBe(value);
      expect(body.key).toBe("ocr:text");
      expect(body.before_source).toBe(source);
      expect(body.before_type).toBe("text");
      expect(body.blob_key).toBe(files[0].blob_key);
      if (variant === "error" || variant === "remove-error")
        return route.fulfill({
          status: 409,
          json: { code: "conflict", message: "Changed", class: "conflict" },
        });
      if (body.remove) removed = true;
      else {
        value = body.value;
        source = "user";
      }
      return route.fulfill({ json: { op_id: 42 } });
    });
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page
      .locator(".file-metadata-group")
      .filter({ has: page.getByRole("heading", { name: "OCR", exact: true }) });
    await group
      .locator(".metadata-value")
      .filter({
        has: page.getByRole("button", { name: "编辑识别文字", exact: true }),
      })
      .hover();
    if (variant.startsWith("remove-")) {
      const trigger = group.getByRole("button", {
        name: "删除识别文字",
        exact: true,
      });
      await trigger.click();
      const popup = page.getByRole("dialog", {
        name: "删除「识别文字」？",
        exact: true,
      });
      await expect(popup).toBeVisible();
      expect(writes).toBe(0);
      const cancel = popup.getByRole("button", { name: "取消", exact: true });
      const confirm = popup.getByRole("button", {
        name: "确认删除",
        exact: true,
      });
      await expect(cancel).toBeFocused();
      await cancel.press("Shift+Tab");
      await expect(confirm).toBeFocused();
      await confirm.press("Tab");
      await expect(cancel).toBeFocused();
      const actionBox = await trigger.boundingBox();
      const popupBox = await popup.boundingBox();
      expect(popupBox!.x).toBeGreaterThanOrEqual(0);
      expect(popupBox!.x + popupBox!.width).toBeLessThanOrEqual(
        page.viewportSize()!.width,
      );
      expect(Math.abs(popupBox!.y - actionBox!.y)).toBeLessThan(180);
      await expect(popup).toHaveAttribute("data-position", /^top-/);
      const hint = popup.getByText("不会自动恢复，可在历史中撤销。", {
        exact: true,
      });
      expect(
        await hint.evaluate(
          (element) =>
            element.getBoundingClientRect().height <=
            parseFloat(getComputedStyle(element).lineHeight) + 1,
        ),
      ).toBe(true);
      await page.getByRole("button", { name: "取消", exact: true }).click();
      await expect(popup).toHaveCount(0);
      expect(writes).toBe(0);
      await trigger.click();
      if (variant === "remove-saved" || variant === "remove-error") {
        await popup
          .getByRole("button", { name: "确认删除", exact: true })
          .click();
        await expect.poll(() => writes).toBe(1);
        if (variant === "remove-saved") {
          await expect(popup).toHaveCount(0);
          await expect(
            group.getByRole("button", { name: "复制识别文字", exact: true }),
          ).toHaveCount(0);
        } else {
          await expect(popup.getByRole("alert")).toBeVisible();
          await expect(
            popup.getByRole("button", { name: "确认删除", exact: true }),
          ).toBeDisabled();
        }
      }
      await capture(
        page,
        errors,
        info.title,
        variant === "remove-error" ? ["归档状态已变化"] : [],
      );
      return;
    }
    if (variant.startsWith("hover")) {
      const cell = group.locator(".metadata-value").filter({
        has: page.getByRole("button", { name: "编辑识别文字", exact: true }),
      });
      const value = await cell.locator(".metadata-value-content").boundingBox();
      const bounds = await cell.boundingBox();
      const padding = await cell.evaluate((el) =>
        parseFloat(getComputedStyle(el).paddingInlineEnd),
      );
      expect(value!.width + padding).toBeCloseTo(bounds!.width, 1);
      const action = await cell
        .getByRole("button", { name: "编辑识别文字" })
        .boundingBox();
      const lineHeight = await cell.evaluate((el) =>
        parseFloat(getComputedStyle(el).lineHeight),
      );
      expect(
        Math.abs(action!.y + action!.height / 2 - bounds!.y - lineHeight / 2),
      ).toBeLessThan(1);
      const overlapsCopy = () =>
        cell.evaluate((element) => {
          const copy = element
            .querySelector(".copy-text-action")!
            .getBoundingClientRect();
          const actions = element
            .querySelector(".metadata-value-transient-actions")!
            .getBoundingClientRect();
          return (
            copy.left < actions.right &&
            copy.right > actions.left &&
            copy.top < actions.bottom &&
            copy.bottom > actions.top
          );
        });
      await expect.poll(overlapsCopy).toBe(false);
      if (variant === "hover") {
        const toolbar = await cell
          .locator(".metadata-value-transient-actions")
          .boundingBox();
        expect(toolbar!.x + toolbar!.width).toBeCloseTo(
          value!.x + value!.width,
          1,
        );
      }
      if (variant.startsWith("hover-single")) {
        await page
          .context()
          .grantPermissions(["clipboard-read", "clipboard-write"]);
        await cell.locator(".copy-text-action").click();
        await expect(cell.locator(".copy-text-action")).toHaveText("已复制");
        await expect.poll(overlapsCopy).toBe(false);
        expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
          "这是用于验证复制位置的文本",
        );
        await expect(cell.locator(".copy-text-action")).toHaveText("复制");
        await expect.poll(overlapsCopy).toBe(false);
        await cell.hover();
      }
      await capture(page, errors, info.title);
      return;
    }
    await group
      .getByRole("button", { name: "编辑识别文字", exact: true })
      .click();
    const input = group.getByRole("textbox", { name: "编辑识别文字" });
    await expect(input).toHaveValue(value);
    await input.fill("示例文档\n姓名：合成姓名\n日期：2026-01-02");
    if (variant === "saved" || variant === "error") {
      await group.getByRole("button", { name: "保存", exact: true }).click();
      if (variant === "saved") {
        await expect(input).toHaveCount(0);
        await expect(
          group.getByRole("img", { name: /人工编辑/ }),
        ).toBeVisible();
        await expect(
          page.getByRole("heading", { name: "user", exact: true }),
        ).toHaveCount(0);
      } else await expect(group.getByRole("alert")).toBeVisible();
    }
    await capture(
      page,
      errors,
      info.title,
      variant === "error" ? ["归档状态已变化"] : [],
    );
  });
}

for (const variant of [
  "empty",
  "edit",
  "saved",
  "manual",
  "mobile",
  "dark",
] as const) {
  test(`ai-supplement-${variant}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mobile: variant === "mobile",
      mode: variant === "dark" ? "dark" : "light",
    });
    let hint = "";
    let hintSource = "ocr";
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file: files[0],
          tags: [],
          extracted: ["file", "media", "exif", "orientation", "ocr", "desc"],
          metadata: [
            { key: "ocr:hint", value: hint, source: hintSource, type: "text" },
            { key: "desc:hint", value: "", source: "desc", type: "text" },
            {
              key: "ocr:model",
              value: "openai/gpt-6-luna",
              source: "ocr",
              type: "text",
            },
            {
              key: "ocr:text",
              value: "合成测试文档\n示例姓名",
              source: variant === "manual" ? "user" : "ocr",
              type: "text",
            },
            {
              key: "desc:text",
              value: "一份用于验收的合成文档。",
              source: "desc",
              type: "text",
            },
          ],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/metadata", (route) => {
      if (route.request().method() === "GET") return route.fallback();
      const body = route.request().postDataJSON();
      expect(body.key).toBe("ocr:hint");
      expect(body.before_value).toBe(hint);
      expect(body.before_source).toBe(hintSource);
      expect(body.before_type).toBe("text");
      hint = body.value;
      hintSource = "user";
      return route.fulfill({ json: { op_id: 51 } });
    });
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page
      .locator(".file-metadata-group")
      .filter({ has: page.getByRole("heading", { name: "OCR", exact: true }) });
    if (variant !== "manual")
      await expect(group.getByRole("img", { name: /人工编辑/ })).toHaveCount(0);
    const fieldRows = group.locator(".metadata-list > div");
    const hintBounds = await fieldRows.nth(0).boundingBox();
    const modelRow = fieldRows.filter({
      has: page.locator("dt bdi").filter({ hasText: /^模型$/ }),
    });
    const modelBounds = await modelRow.boundingBox();
    expect(hintBounds).not.toBeNull();
    expect(modelBounds).not.toBeNull();
    expect(hintBounds!.height).toBeCloseTo(modelBounds!.height, 1);
    expect(modelBounds!.y - hintBounds!.y - hintBounds!.height).toBeCloseTo(
      0,
      0,
    );
    const hintLabel = group.locator("dt bdi").filter({ hasText: /^补充信息$/ });
    await hintLabel.hover();
    await expect(
      page.getByRole("tooltip", { name: "ocr:hint", exact: true }),
    ).toBeVisible();
    await page.mouse.move(0, 0);
    await expect(
      page.getByRole("tooltip", { name: "ocr:hint", exact: true }),
    ).toHaveCount(0);
    if (variant !== "empty") {
      await group
        .getByRole("button", { name: "编辑补充信息", exact: true })
        .focus();
      await group
        .getByRole("button", { name: "编辑补充信息", exact: true })
        .click();
      await group
        .getByRole("textbox", { name: "补充信息", exact: true })
        .fill("示例姓名");
      if (variant === "saved") {
        await group.getByRole("button", { name: "保存", exact: true }).click();
        await expect(group.getByRole("textbox")).toHaveCount(0);
        await expect(
          group.getByRole("img", { name: /人工编辑/ }),
        ).toBeVisible();
      }
      if (variant === "saved") {
        await expect(
          group.getByRole("button", { name: "复制", exact: true }),
        ).toHaveCount(0);
        const hintText = group.getByText("示例姓名", { exact: true });
        await hintText.hover();
        await expect(
          page.getByRole("tooltip", { name: "示例姓名", exact: true }),
        ).toBeVisible();
        await page.mouse.move(0, 0);
        await expect(page.getByRole("tooltip")).toHaveCount(0);
      }
      if (variant === "manual")
        await expect(
          group.getByRole("button", { name: "保存并重新提取", exact: true }),
        ).toBeDisabled();
    }
    await capture(page, errors, info.title);
  });
}

for (const extension of ["pdf", "doc", "docx", "ppt", "pptx", "pdf-result"]) {
  test(`document-description-${extension}`, async ({ page }, info) => {
    const errors = await prepare(page, { aiConnected: true });
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file: {
            ...files[0],
            name: `synthetic.${extension === "pdf-result" ? "pdf" : extension}`,
            kind: "other",
            media_type: "",
            extraction_pending: [],
          },
          tags: [],
          extracted: [],
          metadata:
            extension === "pdf-result"
              ? [
                  {
                    key: "desc:text",
                    source: "desc",
                    type: "text",
                    value: "综合全文的合成文档描述，用于验证描述正文展示。",
                  },
                ]
              : [],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/content", (route) =>
      route.fulfill({
        contentType: "application/pdf",
        body: pdfFixture(extension.startsWith("ppt")),
      }),
    );
    await page.route(
      /\/api\/v1\/files\/file-1\/derived\/(office|slides)_pdf_v1\/content(?:\?.*)?$/,
      (route) =>
        route.fulfill({
          contentType: "application/pdf",
          body: pdfFixture(extension.startsWith("ppt")),
        }),
    );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const panel = page.locator(".file-info-panel");
    await expect(
      panel.getByRole("button", { name: "生成描述", exact: true }),
    ).toHaveCount(extension !== "pdf-result" ? 1 : 0);
    if (extension !== "pdf-result")
      await expect(
        panel.getByRole("button", { name: "生成描述", exact: true }),
      ).toBeEnabled();
    if (extension === "pdf-result")
      await expect(
        panel.getByText("综合全文的合成文档描述，用于验证描述正文展示。", {
          exact: true,
        }),
      ).toBeVisible();
    await expect(
      panel.getByRole("heading", { name: "OCR", exact: true }),
    ).toHaveCount(0);
    await expect(
      panel.getByRole("heading", { name: "图片方向", exact: true }),
    ).toHaveCount(0);
    await capture(page, errors, info.title);
  });
}

for (const scenario of ["queued", "running", "mobile", "dark"] as const) {
  test(`description-initial-${scenario}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      aiConnected: true,
      mobile: scenario === "mobile",
      mode: scenario === "dark" ? "dark" : "light",
    });
    await page.route("**/api/v1/files/file-1/desc", (route) =>
      route.fulfill({
        json: {
          state: scenario === "queued" ? "queued" : "running",
          manual: true,
        },
      }),
    );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page.locator(".file-metadata-group").filter({
      has: page.getByRole("heading", { name: "描述", exact: true }),
    });
    const action = group
      .locator(".metadata-group-heading")
      .getByRole("button", {
        name: scenario === "queued" ? "已排队" : "正在提取",
        exact: true,
      });
    await expect(action).toBeDisabled();
    await expect(group.getByRole("button")).toHaveCount(1);
    await expect(group.getByRole("status")).toHaveCount(0);
    await expect(group.getByRole("link")).toHaveCount(0);
    await action.scrollIntoViewIfNeeded();
    await capture(page, errors, info.title);
  });
}

for (const variant of [
  "fields",
  "edit",
  "delete",
  "mobile",
  "dark",
  "queued",
] as const) {
  test(`structured-${variant}`, async ({ page }, info) => {
    const errors = await prepare(page, {
      mobile: variant === "mobile",
      mode: variant === "dark" ? "dark" : "light",
    });
    const value = JSON.stringify({
      version: 1,
      fields: [
        {
          key: "title",
          label: "名称",
          type: "text",
          value: "合成数学考试",
          source: "ai",
        },
        {
          key: "name",
          label: "姓名",
          type: "text",
          value: "示例用户",
          source: "user",
        },
        {
          key: "score",
          label: "分数",
          type: "number",
          value: "92.5",
          unit: "分",
          source: "ai",
        },
        {
          key: "date",
          label: "日期",
          type: "date",
          value: "2026-01",
          source: "ai",
        },
        {
          key: "location",
          label: "地点",
          type: "text",
          value: "示例考场",
          source: "ai",
        },
        {
          key: "examiner",
          label: "评定人",
          type: "text",
          value: "",
          uncertain: true,
          source: "ai",
        },
      ],
    });
    await page.route("**/api/v1/files/file-1", (route) =>
      route.fulfill({
        json: {
          file: files[0],
          tags: [],
          extracted: ["file", "media", "ocr", "desc", "orientation", "facts"],
          metadata: [
            { key: "facts:hint", value: "", type: "text", source: "facts" },
            {
              key: "facts:model",
              value: "openai/gpt-6-luna",
              type: "text",
              source: "facts",
            },
            { key: "facts:recipe", value: "1", type: "int", source: "facts" },
            { key: "facts:data", value, type: "text", source: "facts" },
          ],
        },
      }),
    );
    await page.route("**/api/v1/files/file-1/facts", (route) =>
      route.fulfill({
        json: {
          state: variant === "queued" ? "queued" : "ready",
          manual: true,
        },
      }),
    );
    await page.goto("/files?file=file-1");
    await page.getByRole("button", { name: "文件详情", exact: true }).click();
    const group = page.locator(".file-metadata-group").filter({
      has: page.getByRole("heading", { name: "结构化信息", exact: true }),
    });
    await group.scrollIntoViewIfNeeded();
    await expect(
      group.getByText("92.5 分", { exact: true }).locator("visible=true"),
    ).toBeVisible();
    await expect(group.getByText("无法辨认", { exact: true })).toBeVisible();
    const contextRow = await group
      .locator(".metadata-list")
      .first()
      .locator(":scope > div")
      .last()
      .boundingBox();
    const firstField = await group
      .locator(".metadata-field-tree .metadata-list > div")
      .first()
      .boundingBox();
    expect(firstField!.y - contextRow!.y - contextRow!.height).toBeCloseTo(
      0,
      1,
    );
    await expect(group.getByRole("heading")).toHaveCount(1);
    await expect(
      group.getByRole("button", { name: "复制", exact: true }),
    ).toHaveCount(0);
    await expect(group.locator(".metadata-field-tree dt bdi")).toHaveText([
      "名称",
      "姓名",
      "分数",
      "日期",
      "地点",
      "评定人",
    ]);
    if (variant !== "queued")
      await group
        .locator(".metadata-value")
        .filter({
          has: page.getByRole("button", { name: "编辑姓名", exact: true }),
        })
        .hover();
    if (variant === "edit") {
      await group
        .getByRole("button", { name: "编辑姓名", exact: true })
        .click();
      await group.getByRole("textbox").fill("更正后的示例姓名");
    } else if (variant === "delete") {
      await group
        .getByRole("button", { name: "删除姓名", exact: true })
        .click();
      await expect(
        page.getByRole("button", { name: "确认删除", exact: true }),
      ).toBeVisible();
    } else if (variant === "queued") {
      await expect(
        group.getByRole("button", { name: "重新提取结构化信息", exact: true }),
      ).toBeDisabled();
      await expect(group.getByText("已排队", { exact: true })).toBeVisible();
    } else {
      await group
        .getByRole("button", { name: "编辑姓名", exact: true })
        .hover();
    }
    await capture(page, errors, info.title);
  });
}
