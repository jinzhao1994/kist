# Visual regression checks

`Visual` builds the production UI and kist binary, serves an isolated empty library, and captures deterministic UI fixtures in a pinned Linux amd64 Playwright image. API responses and preview artwork are synthetic; this is a visual check, not a substitute for backend or media integration tests.

The suite covers every built-in theme in light/dark mode, boxes, box files/upload controls, settings, tags, an empty library, English, a mobile grid, and six media detail states. Fixtures use fixed dates, locale, timezone and preview artwork. No personal archive is accessed.

## Review a change

1. Open the failed **Visual** run in GitHub Actions.
2. Read its summary. Download `visual-report-<attempt>`, extract it, and open `report/index.html`. Compare expected/current/diff images. Click an image for full size. Capture failures include Playwright traces under `test-results/` and cannot be accepted.
3. If every change is intentional, run **Accept visual baselines** on `main`, entering the reviewed Visual **run ID**. The equivalent CLI command is:

   ```sh
   gh workflow run visual-accept.yml --ref main -f run_id=RUN_ID
   ```

4. The workflow downloads the candidate, validates its provenance and completeness, commits the PNGs and manifest, and pushes normally. It explicitly dispatches **CI** and **Visual** after the bot push. Confirm both pass on the new commit.
5. Fast-forward your local branch before continuing development:

   ```sh
   git pull --ff-only
   ```

If the source run is stale, rerun Visual on current main and review that run instead. If validation or capture failed, fix the cause; do not accept a partial suite. If the bot pushed but dispatch failed, manually run CI and Visual on main. Branch protection may require an approved repository workflow change; this workflow never bypasses it or force-pushes.

Only `web/visual/baselines/` PNGs and their provenance manifest are committed. Candidates expire after 14 days; reports and traces after 30 days. Formal baselines are never generated on macOS. The first run intentionally reports missing baselines and uses the same reviewed acceptance path.

## Maintain the suite

Add cases to `web/visual/tests/`. Each test has a unique lowercase hyphenated title and exactly one screenshot. Theme cases derive from the application's theme registry. Unexpected API requests and page errors fail capture. Keep loading/error states explicit and deterministic.

`make check` validates TypeScript and the artifact guards without launching a browser. Visual capture runs in Actions. Update `web/visual/environment.json` together with the exact `@playwright/test` dependency when changing browser versions, then review regenerated baselines. Comparison uses pixelmatch's 0.1 color threshold and default antialias filtering, with zero differing pixels allowed after filtering. Missing, removed and resized images fail too.

The acceptance workflow only accepts a completed Visual push/manual run from this repository's current main commit, checks the latest attempt and exact artifact contents, and verifies PNG hashes. It never automatically decides that a visual change is correct.
