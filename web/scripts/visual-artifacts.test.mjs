import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import crypto from "node:crypto";
import { execFileSync } from "node:child_process";
import { PNG } from "pngjs";
import {
  captureNames,
  validateRun,
  validateCandidate,
  difference,
  validateReference,
  resolveReference,
  compareCandidates,
  acceptCandidate,
} from "./visual-artifacts.mjs";

const report = () => ({
  errors: [],
  suites: [
    {
      specs: [
        {
          title: "grid-light",
          ok: true,
          tests: [
            {
              expectedStatus: "passed",
              status: "expected",
              results: [{ status: "passed" }],
            },
          ],
        },
      ],
    },
  ],
});
test("only a complete, passing, unretried capture can become a candidate", () => {
  assert.deepEqual(captureNames(report()), ["grid-light.png"]);
  for (const mutate of [
    (r) => {
      r.errors.push({ message: "worker failed" });
    },
    (r) => {
      r.suites = [];
    },
    (r) => {
      r.suites[0].specs[0].title = "../bad";
    },
    (r) => {
      r.suites[0].specs[0].tests[0].results[0].status = "skipped";
    },
    (r) => {
      r.suites[0].specs[0].tests[0].results.push({ status: "passed" });
    },
    (r) => {
      r.suites[0].specs.push(r.suites[0].specs[0]);
    },
    (r) => {
      r.suites[0].specs[0].tests[0].expectedStatus = "failed";
    },
  ]) {
    const r = report();
    mutate(r);
    assert.throws(() => captureNames(r));
  }
});
test("source must be completed current-main Visual from this repository", () => {
  const expected = {
    repository: "owner/repo",
    sha: "a".repeat(40),
    workflowId: 10,
  };
  const run = {
    repository: { full_name: expected.repository },
    head_repository: { full_name: expected.repository },
    head_sha: expected.sha,
    head_branch: "main",
    workflow_id: 10,
    event: "push",
    status: "completed",
    conclusion: "failure",
  };
  validateRun(run, expected);
  for (const bad of [
    { head_sha: "b".repeat(40) },
    { workflow_id: 11 },
    { head_branch: "feature" },
    { event: "pull_request" },
    { status: "in_progress" },
    { conclusion: "cancelled" },
    { head_repository: { full_name: "fork/repo" } },
  ])
    assert.throws(() => validateRun({ ...run, ...bad }, expected));
});
test("candidate validates provenance, exact files and content before any write", (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "kist-visual-test-"));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  fs.mkdirSync(path.join(directory, "screenshots"));
  const bytes = PNG.sync.write(new PNG({ width: 2, height: 2 }));
  const file = path.join(directory, "screenshots/grid-light.png");
  fs.writeFileSync(file, bytes);
  fs.writeFileSync(
    path.join(directory, "capture.json"),
    JSON.stringify(report()),
  );
  const expected = {
    sha: "a".repeat(40),
    baselineSha: "b".repeat(40),
    runId: "42",
    attempt: "1",
    environmentHash: "c".repeat(64),
    repository: "owner/repo",
  };
  const manifest = {
    schema: 2,
    ...expected,
    files: [
      {
        name: "grid-light.png",
        width: 2,
        height: 2,
        sha256: crypto.createHash("sha256").update(bytes).digest("hex"),
      },
    ],
  };
  fs.writeFileSync(
    path.join(directory, "manifest.json"),
    JSON.stringify(manifest),
  );
  assert.equal(validateCandidate(directory, expected).files.length, 1);
  for (const key of Object.keys(expected))
    assert.throws(() =>
      validateCandidate(directory, { ...expected, [key]: "wrong" }),
    );
  fs.writeFileSync(path.join(directory, "screenshots/extra.png"), bytes);
  assert.throws(() => validateCandidate(directory, expected));
  fs.unlinkSync(path.join(directory, "screenshots/extra.png"));
  const altered = new PNG({ width: 2, height: 2 });
  altered.data.fill(255);
  fs.writeFileSync(file, PNG.sync.write(altered));
  assert.throws(() => validateCandidate(directory, expected));
  fs.unlinkSync(file);
  assert.throws(() => validateCandidate(directory, expected));
  if (process.platform !== "win32") {
    const target = path.join(directory, "outside.png");
    fs.writeFileSync(target, bytes);
    fs.symlinkSync(target, file);
    assert.throws(() => validateCandidate(directory, expected));
  }
});
test("comparison detects pixel and dimension changes", () => {
  const a = new PNG({ width: 2, height: 2 });
  a.data.fill(255);
  const b = new PNG({ width: 2, height: 2 });
  b.data.fill(255);
  assert.equal(difference(a, b).changed, false);
  b.data[0] = 0;
  b.data[1] = 0;
  b.data[2] = 0;
  assert.equal(difference(a, b).changed, true);
  assert.equal(difference(a, new PNG({ width: 3, height: 2 })).pixels, null);
});

const provenance = {
  repository: "owner/repo",
  sha: "a".repeat(40),
  baselineSha: "b".repeat(40),
  environmentHash: "c".repeat(64),
  runId: "42",
  attempt: "1",
};
const reference = {
  schema: 1,
  approvedCommit: provenance.baselineSha,
  environmentHash: provenance.environmentHash,
  reviewedRunId: "40",
};
function fixture(directory, expected, names = ["grid-light"]) {
  fs.mkdirSync(path.join(directory, "screenshots"), { recursive: true });
  const capture = report();
  capture.suites[0].specs = names.map((title) => ({
    ...report().suites[0].specs[0],
    title,
  }));
  fs.writeFileSync(
    path.join(directory, "capture.json"),
    JSON.stringify(capture),
  );
  const bitmap = new PNG({ width: 2, height: 2 });
  bitmap.data.fill(255);
  const bytes = PNG.sync.write(bitmap);
  const files = names
    .map((title) => {
      const name = `${title}.png`;
      fs.writeFileSync(path.join(directory, "screenshots", name), bytes);
      return {
        name,
        width: 2,
        height: 2,
        sha256: crypto.createHash("sha256").update(bytes).digest("hex"),
      };
    })
    .sort((a, b) => a.name.localeCompare(b.name));
  fs.writeFileSync(
    path.join(directory, "manifest.json"),
    JSON.stringify({ schema: 2, ...expected, files }),
  );
}
function temporary(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "kist-visual-test-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  return root;
}
test("fresh captures compare across revisions and expose added/removed scenarios", (t) => {
  const root = temporary(t);
  const current = path.join(root, "candidate"),
    baseline = path.join(root, "baseline"),
    output = path.join(root, "report");
  fixture(current, provenance, ["grid-light", "new-scene"]);
  fixture(baseline, { ...provenance, sha: reference.approvedCommit }, [
    "grid-light",
    "old-scene",
  ]);
  const result = compareCandidates(
    current,
    baseline,
    output,
    provenance,
    reference,
  );
  assert.deepEqual(result.changes, [
    "new-scene.png: NEW",
    "old-scene.png: REMOVED",
  ]);
  const html = fs.readFileSync(path.join(output, "index.html"), "utf8");
  assert.ok(html.includes(reference.approvedCommit));
  assert.ok(html.includes(provenance.sha));
  assert.ok(fs.existsSync(path.join(output, "expected-old-scene.png")));
  assert.throws(
    () =>
      compareCandidates(
        current,
        baseline,
        output,
        { ...provenance, baselineSha: provenance.sha },
        reference,
      ),
    /Baseline reference changed/,
  );
  // A candidate cannot be compared with itself or a baseline from another run.
  assert.throws(
    () => compareCandidates(current, current, output, provenance, reference),
    /Manifest mismatch: sha/,
  );
  const manifestFile = path.join(baseline, "manifest.json");
  const manifest = JSON.parse(fs.readFileSync(manifestFile));
  fs.writeFileSync(manifestFile, JSON.stringify({ ...manifest, runId: "41" }));
  assert.throws(
    () => compareCandidates(current, baseline, output, provenance, reference),
    /Manifest mismatch: runId/,
  );
});
test("identical captures pass but a new capture environment requires review", (t) => {
  const root = temporary(t);
  const current = path.join(root, "candidate"),
    baseline = path.join(root, "baseline"),
    output = path.join(root, "report");
  fixture(current, provenance);
  fixture(baseline, { ...provenance, sha: reference.approvedCommit });
  assert.deepEqual(
    compareCandidates(current, baseline, output, provenance, reference).changes,
    [],
  );
  assert.equal(
    compareCandidates(current, baseline, output, provenance, {
      ...reference,
      environmentHash: "d".repeat(64),
    }).changes.length,
    1,
  );
});
test("approval writes only a commit reference and rejects stale or invalid artifacts before writing", (t) => {
  const root = temporary(t),
    candidate = path.join(root, "candidate"),
    destination = path.join(root, "baseline.json");
  fixture(candidate, provenance);
  const original = JSON.stringify(reference);
  fs.writeFileSync(destination, original);
  for (const bad of [
    { ...provenance, baselineSha: "d".repeat(40) },
    { ...provenance, sha: "d".repeat(40) },
    { ...provenance, environmentHash: "d".repeat(64) },
    { ...provenance, attempt: "2" },
  ]) {
    assert.throws(() => acceptCandidate(candidate, bad, destination));
    assert.equal(fs.readFileSync(destination, "utf8"), original);
  }
  acceptCandidate(candidate, provenance, destination);
  assert.deepEqual(JSON.parse(fs.readFileSync(destination)), {
    schema: 1,
    approvedCommit: provenance.sha,
    environmentHash: provenance.environmentHash,
    reviewedRunId: provenance.runId,
  });
  assert.deepEqual(fs.readdirSync(root).sort(), ["baseline.json", "candidate"]);
  for (const bad of [
    { schema: 2 },
    { approvedCommit: "main" },
    { environmentHash: "invalid" },
    { reviewedRunId: "" },
  ])
    assert.throws(() => validateReference({ ...reference, ...bad }));
});

test("PR resolution uses the base revision reference and rejects unavailable history", (t) => {
  const root = temporary(t);
  const git = (...args) =>
    execFileSync("git", args, {
      cwd: root,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    }).trim();
  git("init", "-q");
  git("config", "user.name", "Visual test");
  git("config", "user.email", "visual@example.invalid");
  git("config", "commit.gpgsign", "false");
  git(
    "-c",
    "core.hooksPath=/dev/null",
    "commit",
    "--allow-empty",
    "-qm",
    "approved source",
  );
  const approved = git("rev-parse", "HEAD");
  const destination = path.join(root, "web/visual/baseline.json");
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  const commitReference = (value) => {
    fs.writeFileSync(
      destination,
      JSON.stringify({ ...reference, approvedCommit: value }),
    );
    git("add", "web/visual/baseline.json");
    git("-c", "core.hooksPath=/dev/null", "commit", "-qm", "reference");
    return git("rev-parse", "HEAD");
  };
  const base = commitReference(approved);
  commitReference(base);
  assert.equal(resolveReference(root, base).approvedCommit, approved);
  assert.equal(resolveReference(root).approvedCommit, base);
  assert.throws(
    () => resolveReference(root, "--help"),
    /Invalid reference revision/,
  );
  commitReference("f".repeat(40));
  assert.throws(() => resolveReference(root));
});
