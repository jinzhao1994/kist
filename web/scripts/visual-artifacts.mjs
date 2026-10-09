import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { fileURLToPath } from "node:url";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const results = path.join(web, "visual-results");
const baseline = path.join(web, "visual/baselines");
const environment = JSON.parse(
  fs.readFileSync(path.join(web, "visual/environment.json")),
);
const hash = (bytes) => crypto.createHash("sha256").update(bytes).digest("hex");
const environmentHash = hash(JSON.stringify(environment));
function requireThat(condition, message) {
  if (!condition) throw new Error(message);
}
function regular(file) {
  requireThat(fs.lstatSync(file).isFile(), `Not a regular file: ${file}`);
}
function png(file) {
  regular(file);
  requireThat(fs.statSync(file).size <= 10_000_000, "PNG too large");
  const bytes = fs.readFileSync(file);
  requireThat(
    bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])),
    "Invalid PNG signature",
  );
  const width = bytes.readUInt32BE(16),
    height = bytes.readUInt32BE(20);
  requireThat(
    width > 0 && height > 0 && width * height <= 20_000_000,
    "Invalid PNG dimensions",
  );
  const decoded = PNG.sync.read(bytes);
  return {
    bytes,
    decoded,
    name: path.basename(file),
    sha256: hash(bytes),
    width,
    height,
  };
}
export function captureNames(report) {
  requireThat(report.errors?.length === 0, "Capture has global errors");
  const specs = [];
  const visit = (suite) => {
    specs.push(...(suite.specs || []));
    (suite.suites || []).forEach(visit);
  };
  report.suites.forEach(visit);
  requireThat(specs.length > 0, "Empty capture suite");
  const names = specs
    .map((spec) => {
      requireThat(/^[a-z0-9-]+$/.test(spec.title), "Unsafe screenshot name");
      requireThat(spec.ok && spec.tests.length === 1, "Incomplete capture");
      const test = spec.tests[0];
      requireThat(
        test.expectedStatus === "passed" &&
          test.status === "expected" &&
          test.results.length === 1 &&
          test.results[0].status === "passed",
        "Skipped, retried or failed capture",
      );
      return `${spec.title}.png`;
    })
    .sort();
  requireThat(
    new Set(names).size === names.length,
    "Duplicate screenshot names",
  );
  return names;
}
export function validateRun(run, { repository, sha, workflowId }) {
  requireThat(
    run.head_repository?.full_name === repository &&
      run.repository?.full_name === repository,
    "Wrong repository",
  );
  requireThat(
    run.workflow_id === workflowId && run.head_branch === "main",
    "Wrong workflow or branch",
  );
  requireThat(
    ["push", "workflow_dispatch"].includes(run.event),
    "Untrusted run event",
  );
  requireThat(
    run.status === "completed" &&
      ["success", "failure"].includes(run.conclusion),
    "Run is not complete",
  );
  requireThat(
    run.head_sha === sha,
    "Stale source commit; rerun Visual on current main",
  );
}
export function validateCandidate(directory, expected) {
  requireThat(
    fs.lstatSync(directory).isDirectory(),
    "Invalid candidate directory",
  );
  requireThat(
    JSON.stringify(fs.readdirSync(directory).sort()) ===
      JSON.stringify(["capture.json", "manifest.json", "screenshots"]),
    "Unexpected candidate files",
  );
  for (const name of ["manifest.json", "capture.json"])
    regular(path.join(directory, name));
  const manifest = JSON.parse(
    fs.readFileSync(path.join(directory, "manifest.json")),
  );
  requireThat(manifest.schema === 1, "Unknown manifest schema");
  for (const [key, value] of Object.entries(expected))
    requireThat(manifest[key] === value, `Manifest mismatch: ${key}`);
  requireThat(/^[a-f0-9]{40}$/.test(manifest.sha), "Invalid source SHA");
  const names = captureNames(
    JSON.parse(fs.readFileSync(path.join(directory, "capture.json"))),
  );
  requireThat(
    Array.isArray(manifest.files) && manifest.files.length === names.length,
    "Incomplete manifest",
  );
  const screenshots = path.join(directory, "screenshots");
  requireThat(
    fs.lstatSync(screenshots).isDirectory(),
    "Invalid screenshots directory",
  );
  requireThat(
    JSON.stringify(fs.readdirSync(screenshots).sort()) ===
      JSON.stringify(names),
    "Missing or unexpected screenshots",
  );
  const files = names.map((name, index) => {
    const actual = png(path.join(screenshots, name));
    const entry = manifest.files[index];
    for (const key of ["name", "sha256", "width", "height"])
      requireThat(
        entry[key] === actual[key],
        `Screenshot mismatch: ${name} ${key}`,
      );
    return actual;
  });
  return { manifest, files };
}
function context() {
  return {
    repository: process.env.GITHUB_REPOSITORY,
    sha: process.env.SOURCE_SHA || process.env.GITHUB_SHA,
    runId: process.env.SOURCE_RUN_ID || process.env.GITHUB_RUN_ID,
    attempt: process.env.SOURCE_ATTEMPT || process.env.GITHUB_RUN_ATTEMPT,
    environmentHash,
  };
}
function prepare() {
  const directory = path.join(results, "candidate");
  const report = fs.readFileSync(path.join(results, "capture.json"));
  const names = captureNames(JSON.parse(report));
  const files = names.map((name) => {
    const { bytes, decoded, ...entry } = png(
      path.join(directory, "screenshots", name),
    );
    return entry;
  });
  fs.writeFileSync(path.join(directory, "capture.json"), report);
  fs.writeFileSync(
    path.join(directory, "manifest.json"),
    JSON.stringify({ schema: 1, ...context(), files }, null, 2) + "\n",
  );
  validateCandidate(directory, context());
}
export function difference(a, b) {
  if (a.width !== b.width || a.height !== b.height)
    return { changed: true, pixels: null };
  const diff = new PNG({ width: a.width, height: a.height });
  const pixels = pixelmatch(a.data, b.data, diff.data, a.width, a.height, {
    threshold: 0.1,
  });
  return { changed: pixels !== 0, pixels, image: PNG.sync.write(diff) };
}
const escape = (text) =>
  String(text).replace(
    /[&<>"']/g,
    (char) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        char
      ],
  );
function compare() {
  const { files } = validateCandidate(
    path.join(results, "candidate"),
    context(),
  );
  const report = path.join(results, "report");
  fs.mkdirSync(report, { recursive: true });
  const oldNames = fs.existsSync(baseline)
    ? fs.readdirSync(baseline).filter((name) => name.endsWith(".png"))
    : [];
  const changes = [],
    sections = [];
  const baselineManifest = path.join(baseline, "manifest.json");
  if (
    fs.existsSync(baselineManifest) &&
    JSON.parse(fs.readFileSync(baselineManifest)).environmentHash !==
      environmentHash
  )
    changes.push(
      "Capture environment changed; review and regenerate baselines",
    );
  for (const file of files) {
    fs.writeFileSync(path.join(report, `current-${file.name}`), file.bytes);
    const exists = oldNames.includes(file.name);
    let result = { changed: true, pixels: null },
      status = "NEW";
    if (exists) {
      const old = png(path.join(baseline, file.name));
      fs.writeFileSync(path.join(report, `expected-${file.name}`), old.bytes);
      result = difference(old.decoded, file.decoded);
      status = result.changed
        ? result.pixels === null
          ? "DIMENSIONS CHANGED"
          : `${result.pixels} CHANGED PIXELS`
        : "MATCH";
      if (result.image)
        fs.writeFileSync(path.join(report, `diff-${file.name}`), result.image);
    }
    if (result.changed) changes.push(`${file.name}: ${status}`);
    sections.push(
      `<section><h2>${escape(file.name)} — ${status}</h2><div class="images">${[...(exists ? ["expected"] : []), "current", ...(result.image ? ["diff"] : [])].map((kind) => `<figure><figcaption>${kind}</figcaption><a href="${kind}-${file.name}"><img src="${kind}-${file.name}"></a></figure>`).join("")}</div></section>`,
    );
  }
  for (const name of oldNames)
    if (!files.some((file) => file.name === name))
      changes.push(`${name}: REMOVED`);
  const summary = `${files.length} screenshots; ${changes.length} differences`;
  fs.writeFileSync(
    path.join(report, "index.html"),
    `<!doctype html><meta charset="utf-8"><title>kist visual report</title><style>body{font:16px system-ui;margin:24px;background:#fafafa;color:#222}.images{display:flex;align-items:flex-start;gap:12px}figure{margin:0;flex:1;min-width:0}img{width:100%;border:1px solid #ccc}section{border-top:1px solid #bbb;margin-top:24px}figcaption{padding:8px}</style><h1>${summary}</h1><p>Commit ${escape(context().sha)} · run ${escape(context().runId)}</p><pre>${escape(changes.join("\n"))}</pre>${sections.join("")}`,
  );
  fs.writeFileSync(
    path.join(results, "summary.md"),
    `## Visual: ${summary}\n\nSource: \`${context().sha}\`\n\n${changes.map((change) => `- ${change}`).join("\n")}\n\nDownload the visual report artifact and open report/index.html for expected/current/diff images. Baselines are never accepted automatically.\n`,
  );
  console.log(summary);
  if (changes.length) process.exitCode = 1;
}
function accept(directory) {
  const { manifest, files } = validateCandidate(directory, context());
  fs.mkdirSync(baseline, { recursive: true });
  // Validate the entire candidate before touching any tracked baseline.
  for (const name of fs.readdirSync(baseline)) {
    requireThat(
      /^[a-z0-9-]+\.png$/.test(name) || name === "manifest.json",
      "Unexpected tracked baseline file",
    );
    regular(path.join(baseline, name));
  }
  for (const name of fs.readdirSync(baseline))
    fs.unlinkSync(path.join(baseline, name));
  for (const file of files)
    fs.writeFileSync(path.join(baseline, file.name), file.bytes);
  fs.writeFileSync(
    path.join(baseline, "manifest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
}
if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const command = process.argv[2];
  if (command === "prepare") prepare();
  else if (command === "compare") compare();
  else if (command === "accept") accept(path.resolve(process.argv[3]));
  else
    throw new Error(
      "Expected prepare, compare, or accept <candidate-directory>",
    );
}
