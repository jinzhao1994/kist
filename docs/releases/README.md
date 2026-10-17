# Release procedure

Release notes are reviewed in version-named Markdown files in this directory.
Do not create or push a version tag until the maintainer explicitly authorizes
publication. A tag triggers publication of a GitHub Release; workflow dispatch on
`main` only tests and uploads development artifacts.

1. Finish release notes (`docs/releases/vX.Y.Z.md`), installation/upgrade guidance,
   code and visual fixtures first. Use the existing three-part version format;
   do not introduce a VERSION/CHANGELOG file or a PR solely to fit a generic tool.
2. Run `make check`, commit and push. Review CI and Visual on that exact SHA.
   Resolve capture failures and accept intentional visual changes through the
   [visual procedure](../testing/visual.md). Wait for CI and Visual on the final
   acceptance commit; that commit becomes the frozen release candidate.
3. Dispatch `Release` on the candidate's branch. Confirm the run's `headSha`
   equals the candidate SHA. All four artifact smoke jobs must pass; download
   `dist`, verify its `SHA256SUMS`, and exercise the local matching package.
4. After publication authorization, create and push the annotated `vX.Y.Z` tag
   at the reviewed commit. Do not move an existing release tag.
5. The workflow reruns checks, packages and tests the tagged binaries before
   creating the GitHub Release. No registry image or repository visibility
   change is part of this workflow.
6. Download the published assets, verify checksums, confirm the embedded version
   and run the artifact smoke test on the local matching platform. Verify the
   Release URL, tag commit and all four smoke results before announcing success.

If a pre-publication job fails, fix forward and repeat the rehearsal. If the
publication job fails, inspect the tag and existing Release/assets before
retrying; do not silently overwrite already-published files.

## Avoid repeated rehearsals

Record the candidate SHA and run IDs rather than repeatedly consulting the most
recent workflow of any branch. A workflow dispatch resolves the branch at start;
confirm its SHA before using the result. If source, fixtures, release notes or the
baseline commit change, finalize them before rehearsing the new candidate.
Keep the working tree and remote main aligned before tagging. Do not cancel or
restart unrelated release jobs just because a development CI run was superseded.

The tag-triggered check/build/smoke pass is intentional: it verifies binaries
with the real release version embedded. Do not remove it to save time. The
avoidable cost is starting a rehearsal while tests, notes and visual review are
still changing. Pure documentation edits require `git diff --check` locally;
code changes still require `make check`. Do not rerun unchanged local suites
solely while waiting for remote jobs.

Use the current workflows and scripts as the source of truth for jobs and
platforms. Poll active runs at a measured interval, reporting stage transitions,
failures and what remains; do not treat queued/in-progress as passed. Existing
explicit publication authorization covers the normal tag and downstream release
chain. If the maintainer pauses publication, stop before the next publishing step
and wait for a new instruction. Repository visibility and credential expansion
are separate actions, not implied by a version release.

## Completion evidence

| Stage | Evidence before claiming completion |
|---|---|
| Candidate | Exact SHA, clean tree, local check, CI and Visual results |
| Rehearsal | Same SHA, four smoke results, downloaded archive checksums |
| Binary release | Tag resolves to candidate, non-draft Release URL, all assets, published checksums, embedded version/commit and local artifact smoke |
| GHCR version | Both native builds and anonymous pull/verify jobs, published index digest |
| Stable channels and mirror | Container Latest and both relevant Docker Hub runs, including promotion and both architectures |
| User deployment | User's actual running version after pull/recreate; a successful registry publication alone does not establish this |

Record evidence in the stage's plan/retrospective when needed, not as a growing
log in AGENTS.md. Final reports should separate a completed binary release from
any still-pending registry job and link to the release. Update notices on NAS
clients are outside the publication pipeline; see the [Synology guide](../synology.zh-CN.md#后续更新).

## Container images

`Container Release` publishes public GHCR images independently of binary assets.
It follows a successful tag-triggered `Release` via `workflow_run`; this also works
when `GITHUB_TOKEN` created the Release. A manually dispatched binary rehearsal
does not publish images. Package visibility is configured once
by its owner; the workflow does not change visibility. Post-publication verification
uses an empty Docker credential configuration to prove anonymous access.

For an already published stable release, rehearse on `main`:

```sh
gh workflow run container-release.yml --ref main -f tag=v0.1.0 -f publish=false
```

After publication is authorized, use `publish=true` for the same tag. Only stable
`vMAJOR.MINOR.PATCH` tags with a published, non-draft GitHub Release qualify.
The workflow resolves the source commit and checks it against the triggering
Release workflow's SHA when available. It uses current workflow tooling but builds
only the resolved release source; OCI labels record source, version and revision.

1. Build natively on amd64 and arm64; verify platform, binary version and OCI labels.
2. Run actual media, HTTP access/range, persistence and deep integrity acceptance.
3. Push each passing image under a unique candidate tag and collect its digest.
4. Refuse an existing version tag. Combine exactly the two tested digests into the
   version index; this workflow does not publish moving aliases. A subsequent
   Container Latest workflow verifies and promotes the latest stable release to
   `latest`.
5. Pull that index anonymously by digest on both native runners and repeat acceptance.

Check **all** jobs before announcing success. The publish job summary includes
the version reference and index digest; `container-release-manifest` retains the
index JSON as an Actions artifact. Candidate tags back the index and are not for
deployment; do not delete their package versions while a release references them.

If a build fails, no version index is published. If verification fails after the
index was published, inspect the failure and rerun failed jobs; rerunning the
entire publication intentionally refuses to overwrite the version. A necessary
image correction gets a new release version rather than silently replacing an
existing deployment. Binary Releases can succeed while image publication fails:
check both workflows separately. See [Docker deployment](../docker.md) for anonymous
image pulls, NAS configuration and upgrade steps.

## Docker Hub mirror

`Container Docker Hub` follows successful Container Release and Container Latest
runs. It copies the GHCR index and both architectures with Skopeo
`--all --preserve-digests`; it never rebuilds the release. Native amd64/arm64 jobs
pull the version index anonymously from Docker Hub and repeat media/persistence
acceptance before updating `latest`. Versions are visible during acceptance;
`latest` remains unchanged if acceptance fails. Existing version tags with
different content are rejected; moving channels are rechecked before promotion.
GHCR remains usable when Hub publication fails. Inspect both workflows.

One-time setup: create the public Docker Hub repository `jinzhao1994/kist`, then
configure Actions secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` (a Docker Hub
access token with Read & Write permissions, not Delete). Never commit credentials.
Missing credentials fail explicitly; the workflow does not change visibility.

Backfill an existing release or retry a channel after fixing credentials:

```sh
gh workflow run container-dockerhub.yml --ref main -f tag=v0.1.2
gh workflow run container-dockerhub.yml --ref main -f tag=latest
```

Development images are no longer published. Confirm both native acceptance jobs
and final confirmation passed before advertising the Docker Hub tag. No temporary
Docker Hub tags are created. Old Hub `candidate-*` tags may be removed manually
after checking the released tags; do not delete shared manifests by digest or
apply that cleanup to GHCR package versions, which may back platform indices.
Read & Write credentials remain sufficient; automatic deletion is not required.

Release notes must distinguish forward upgrade compatibility, destructive
migrations (if any), and downgrade limits. Do not require a fresh full-library
backup or full blob scrub for routine compatible upgrades. Require a recovery
point for destructive migration or when older-version rollback is needed;
scheduled backups remain independent of application updates.
