# Release procedure

Release notes are reviewed in version-named Markdown files in this directory.
Do not create or push a version tag until the maintainer explicitly authorizes
publication. A tag triggers a public release operation; workflow dispatch on
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
