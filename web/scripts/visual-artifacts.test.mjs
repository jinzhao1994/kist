import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import crypto from "node:crypto";
import { PNG } from "pngjs";
import {
  captureNames,
  validateRun,
  validateCandidate,
  difference,
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
    runId: "42",
    attempt: "1",
    environmentHash: "environment",
    repository: "owner/repo",
  };
  const manifest = {
    schema: 1,
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
