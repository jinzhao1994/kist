# Release procedure

Release notes are reviewed in version-named Markdown files in this directory.
Do not create or push a version tag until the maintainer explicitly authorizes
publication. A tag triggers publication of a GitHub Release; workflow dispatch on
`main` only tests and uploads development artifacts.

1. Run `make check`, commit, push, and inspect CI and Visual on that exact SHA.
2. Dispatch `Release` on the same commit's branch. All four artifact smoke jobs
   must pass; inspect the `dist` artifact and its `SHA256SUMS`.
3. Review `docs/releases/vX.Y.Z.md`, installation instructions and known limits.
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

`Container Docker Hub` follows successful Container Release, Container Latest and
Container Edge runs. It copies the GHCR index and both architectures with Skopeo
`--all --preserve-digests`; it never rebuilds the release. Native amd64/arm64 jobs
pull the candidate anonymously from Docker Hub and repeat media/persistence
acceptance before publishing the user-facing tag. Existing version tags with
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

Use `tag=edge` for the development channel. Confirm both native acceptance jobs
and promotion passed before advertising the Docker Hub tag. Candidate tags are
implementation details; deploy version tags or channels.

Release notes must distinguish forward upgrade compatibility, destructive
migrations (if any), and downgrade limits. Do not require a fresh full-library
backup or full blob scrub for routine compatible upgrades. Require a recovery
point for destructive migration or when older-version rollback is needed;
scheduled backups remain independent of application updates.
