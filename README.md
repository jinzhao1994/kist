# kist

**English** · [中文](README.zh-CN.md)

A self-hosted personal archive. The unit is a **box** — one trip, one hospital
visit, one batch of paperwork — and a single global **tag** tree cuts across
boxes for retrieval. It holds photos and video as comfortably as it holds
passports, contracts and lab reports.

One Go binary plus SQLite. Apache-2.0.

> **v0.1.0 is available.** Download the [release archives](https://github.com/jinzhao1994/kist/releases/tag/v0.1.0)
> or deploy with Docker. M0–M7 are complete; see [Milestones](#milestones).
> kist has no built-in login and retains historical originals permanently.

## The idea

Most tools make you file a document in one place and then find it from that
same place. kist separates the two:

| | answers |
|---|---|
| **box** | how did this file come to be? |
| **tag** | what is this file? |
| metadata | what state is it in now? *(after v0.1)* |

A box is where things land — flat, non-nestable, one activity each. A file
belongs to exactly one box. The tag tree is orthogonal to all of them, so
"every insurance document I have ever had" and "that week in Kyoto" are both
one query, and neither one had to be anticipated at filing time.

## Why not one of the existing ones

- **Business changes support strict undo.** The authority is an append-only op log, not
  the database. Undo appends an inverse op, so undos are themselves undoable,
  when later changes do not conflict. Request receipts are not undoable.
  The whole database can be rebuilt from the log.
- **Documents and photos in one archive.** Photo managers assume everything has
  an EXIF date; document managers have no idea what to do with 4000 pictures.
- **Nothing traps your data.** A library is two ordinary directories: content
  lives content-addressed under `blobs/`, and the op log is one JSON-per-line
  file per day under `ops/`. `boxes/` is a human-readable projection you can
  browse with any file manager, and `kist.db` is a cache that can be thrown
  away and rebuilt. Back the archive up by copying a directory; delete kist and
  both your files and their history are still there, in formats you can read
  with `less` and `grep`.

## Design documents

Written in Chinese, and worth reading before the code:

- [`docs/design/2026-08-22-kist-v0.1-design.md`](docs/design/2026-08-22-kist-v0.1-design.md)
  — the authoritative design. It records rejected alternatives alongside every
  decision, on purpose.
- [`docs/design/open-questions.md`](docs/design/open-questions.md) — what is
  still undecided.

## Installation and operations

Release archives contain the executable, license and usage documentation.
See [installation, backup, recovery and upgrades](docs/operations.md) for platform
selection and checksum verification. Download stable builds from [GitHub Releases](https://github.com/jinzhao1994/kist/releases/tag/v0.1.0).
Untagged workflow artifacts are development builds. Docker instructions are below.

## Building from source

Go 1.26.1 or newer and Node.js 24 LTS or 26+ are needed to build from source.
Checks and release packaging also require Python 3.
The resulting binary includes the web assets and needs neither Node.js nor a C toolchain.

```sh
npm --prefix web ci
make check   # frontend checks plus tidy, gofmt, vet, race tests, and build
make build   # -> bin/kist
make dist    # -> dist/, four platform archives + SHA256SUMS (requires Python 3)
```

## Docker (with FFmpeg)

For Synology's web interface, see the [Synology guide (Chinese)](docs/synology.zh-CN.md).

For prebuilt Linux amd64/arm64 images, use `compose.image.yaml`; see the
[NAS installation and upgrade guide](docs/docker.md). Docker Hub and GHCR images are public;
no registry login is needed:

```sh
docker compose -f compose.image.yaml pull
docker compose -f compose.image.yaml run --rm kist init /library --yes
docker compose -f compose.image.yaml up -d
```

Only initialize a new library. The image defaults to `ghcr.io/jinzhao1994/kist:v0.1.0`;
Docker selects the native architecture. For current stable releases from Docker Hub,
set `KIST_IMAGE=jinzhao1994/kist:latest` in `.env`; use `:v0.1.2` to pin a version.
Synology users should prefer Docker Hub for its image update UI integration.

To build from source instead, use the original Compose file:

```sh
docker compose build
docker compose run --rm kist init /library --yes
docker compose up -d
```

Open `http://127.0.0.1:8473`. The image runs as UID/GID `10001:10001` and
includes FFmpeg, ffprobe, libheif tools and timezone data. The default named volume retains
the whole library across container recreation. `docker compose down` keeps it;
`docker compose down -v` deletes it. Do not run initialization again for an
existing library. CLI operations use the same library, for example:

```sh
docker compose exec kist kist --library /library verify --deep
```

Set `TZ=Asia/Shanghai` before initialization to choose the library timezone.
For a different local port, set both `KIST_PORT=18473` and
`KIST_ORIGIN=http://127.0.0.1:18473`. Remote access requires explicitly changing
`KIST_BIND` to the host interface address and setting `KIST_ORIGIN` to the trusted browser origin; use
only a trusted network or an authenticated reverse proxy because kist has no
login. The default exposes the service only on the host's loopback address.

To use an existing library, replace the named volume with an absolute host
path and ensure UID/GID `10001:10001` can access it, or configure `user:` for its
existing owner. Mount the entire library, not just `kist.db` or `blobs/`.
There is no automatic library creation or recursive ownership change.

The image includes the tools for thumbnails, HEIC previews and video transcoding.
CI builds and exercises the image natively on both Linux amd64 and arm64,
including real media processing and persistence across container recreation.
Versioned GHCR images pass the same checks before publication and after pulling. Run `sh scripts/docker-smoke.sh kist:local`
(with Docker, curl and Python 3) to exercise actual media jobs, HTTP ranges,
access checks and persistence across container recreation.

## Using it

```sh
kist init                                  # create a library, interactively
kist box new "Yunnan" --start 2019-08-12   # a box is one trip, one visit, one batch
kist import --box Yunnan ~/Pictures/2019/  # copy files in; Ctrl-C is safe and resumable
kist ls Yunnan                             # what is in there
kist extract                               # read EXIF; makes --sort captured real
kist file show <id>                        # everything known about one file
kist find --name jpg --tag people          # search across boxes
kist find --tag people --not-tag private --sort name --asc
kist file tag people <file-id>             # attach a tag
kist relink                                # rebuild boxes/, the human-readable mirror
```

Business changes can be undone by appending a compensating operation, provided
later changes do not conflict. Request receipts are not undoable:

```sh
kist log            # what has happened to this library
kist undo <op-id>   # take one of them back
```

### Web interface

```sh
kist serve --library ~/kist
```

Open `http://127.0.0.1:8473`. Create a box, choose or drop files into it, then
browse files, inspect metadata, download originals, and manage tags. Filters
combine names, dates, media types and tag AND/OR/NOT conditions. The grid shows
the number of loaded files; use **Load more** to continue through the results.

Use **Move to trash** on a box page to remove the entire box after confirming
its name and total file count. Restoring the deletion restores folders, files,
tag assignments, metadata tombstones and extraction records together. A stale
confirmation is rejected. Once these operations are recorded, older kist
versions cannot read the new actions; do not downgrade that library.

Open **History** in the sidebar, or from a box, to inspect operations and undo
them. **File versions** in file details opens that file's operation history;
**Compare versions** compares its state after two operation numbers, including
metadata, tag paths and whether the file existed. **View box at this point** shows
the box at that operation, with downloads bound to that historical state. These
views are read-only and do not start extraction or preview jobs.

**Recycle bin** lists deletion operations that have not been restored. Restore
applies to the whole operation, including every file in a batch. Undo and restore
append new operations; they never erase history. Conflicting later changes cause
an atomic refusal: inspect the reported blocking operation before deciding what
to undo next. In particular, undo later metadata extraction or suppression before
undoing an import. v0.1 retains historical originals permanently and offers no
empty-bin or retention-period setting. Historical queries replay the log; large
histories may exceed the 30-second request limit and return a retryable error.

**Display settings** separates English/Chinese language, regional number/date
formatting, display timezone and reading direction. These preferences stay in
the browser. Mantine controls support gray, green, yellow, pink, blue, and orange
primary colors, independently from Light, Dark, or Follow system appearance.
Existing skin preferences migrate to the corresponding primary color. System
changes apply automatically in Follow system
mode. The grid loads derived thumbnails, and details generate larger image
previews or H.264/AAC video on demand. Unsupported files keep an original
download link.

Uploads default to at most 1,000 files and a 32 GiB multipart body per batch.
Use `--max-upload-files` and `--max-upload-bytes` to adjust those limits. A batch
must arrive completely before it can commit. If the response is lost, use
**Try again** or **Check result**; refreshing the same tab retains the request
key for checking its receipt. Re-selecting files after explicitly starting a
new operation creates a new request. Metadata extraction runs separately;
its failure does not mean the original files were lost.

There is no login. Remote access requires an explicit listener and trusted
origin, for example on a trusted LAN:

```sh
kist serve --library ~/kist --listen 0.0.0.0:8473 --allow-origin http://192.168.1.20:8473
```

Use your server's actual address. For a reverse proxy, configure its full public
origin instead and protect access at the proxy. Host/Origin checks are not user
authentication. The versioned API schema is at `/api/v1/openapi.json`; writes
require an `Idempotency-Key`, and `/api/v1/requests/{key}` returns the durable
operation receipt. CLI and Web can use the same library; **Refresh** picks up
CLI changes. Repair, rebuild and other maintenance remain CLI operations.

### Scripting

Every command takes `--json` and prints one object, so kist composes with the
rest of your tools:

```sh
kist box new "Sichuan" --json | jq -r .box.id
kist find --tag people --json | jq -r '.files[].id' | xargs kist file tag scanned
kist verify --json | jq -e '.findings | length == 0'   # exit 1 if anything is wrong
```

`find` pages with a cursor: `--json` gives you `.cursor`, and `--cursor` takes
it back. Errors go to stderr with a non-zero exit status; `--json` renders
structured errors with stable codes instead of plain text.

### Checking that nothing has gone wrong

```sh
kist verify          # is anything missing, mismatched or out of place?  (seconds)
kist verify --deep   # also: replay the whole log and compare it with the tables
kist scrub           # read every byte and re-hash it, to catch silent rot
kist repair          # fetch damaged content back from a remote copy
kist rebuild         # throw away the derived tables and replay the log
```

`verify` stats files; `scrub` reads them. That is the whole difference, and it
is why one is cheap enough for cron and the other is not.

`repair` needs somewhere to repair from. A remote is any other copy of the same
content — an external drive, a network share, a second machine — added by hand
to `kist.toml`:

```toml
[[remote]]
name = "backup"
path = "/Volumes/Archive/kist/blobs"
```

The usual way to make one is the command that backs the library up anyway:

```sh
rsync -a ~/kist/blobs/ /Volumes/Archive/kist/blobs/
```

For a complete library backup, stop all writers (including imports and repairs),
keep them stopped throughout the copy, and copy `kist.toml`, `ops/`, and `blobs/`
to a new backup directory. A live copy is not a consistent backup, even when
blobs are copied first. Keep the previous backup until the new copy completes.
On a separate restored copy, run `kist rebuild`, `kist relink`,
`kist verify --deep`, and `kist scrub` before using it. The blob-only copy above
is a repair source, not a backup of the library's history.


kist never writes to a remote, and never trusts one: content fetched from it is
hashed on the way in and discarded if it is not what was asked for. Run
`kist remote list` to check a path today rather than on the day you need it.

## Derived media

Opening a grid or file requests background work: imports do not wait for image
processing. JPEG/PNG/GIF/WebP and other supported raster images use the Go
renderer; HEIC needs `heif-info` and `heif-dec` (or `heif-convert`) with an HEVC
decoder. Video needs `ffmpeg` and `ffprobe` with H.264/AAC encoders. Docker
includes these tools. On Ubuntu, install `ffmpeg libheif-examples
libheif-plugin-libde265`; native deployments may omit them and still archive,
search and download originals.

Missing tools are checked again every 30 seconds; installation takes effect
without restarting the server. Details offer cancel and retry controls.
Jobs survive restart and share outputs for identical content. Stop the server
before removing the disposable `derived/` directory; browsing regenerates
requested outputs. This never changes originals or undo history.

Recipes are `thumb_512_v1` (JPEG, longest edge 512), `preview_2048_v1`
(JPEG, longest edge 2048), and `preview_1080p_v1` (H.264/AAC MP4, up to
1920×1080). Images preserve aspect ratio and orientation, flatten alpha onto
white, and show the first animation frame. Multi-image HEIF collections are
currently unsupported. Limits include 64 million pixels, 256 MiB compressed
still images, six-hour video inputs and 2 GiB output; unsupported or oversized
files remain downloadable. CLI-imported files need `kist extract` or the UI's
**Read metadata** action to classify their media type.

Derived-media jobs use `GET/POST/DELETE /api/v1/files/{id}/derived/{recipe}` and
`GET/HEAD .../content`. POST accepts `{ "retry": true }` to retry a terminal
failure. These disposable jobs do not use `Idempotency-Key` or create archive
operations; ordinary archive writes still require request keys.


## Milestones

| | | |
|---|---|---|
| M0 | repository groundwork | done |
| M1 | kernel: op log, blob store, CLI import and search | done |
| M2a | integrity: `verify`, `scrub`, `repair`, `rebuild` | done |
| M2b | the query contract and its conformance suite | done |
| M3 | metadata extraction, including EXIF | done |
| M4 | HTTP API and embedded web UI | done |
| M5 | derived media: thumbnails, previews, transcoding; Docker with FFmpeg/libheif | done |
| M6 | history, historical boxes, file version comparison, strict undo and recycle bin | done |
| M7 | docs, packaging, v0.1 | done |

## License

Apache-2.0. See [LICENSE](LICENSE).

Visual regression checks run in GitHub Actions by capturing the last approved commit and the current commit in the same Linux environment. Git stores only the approved commit reference; screenshots stay in Actions artifacts. See [reviewing and updating screenshots](docs/testing/visual.md) for reports and the automated baseline acceptance workflow.

## Security and third-party licenses

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
Third-party license texts are included in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
and shipped with release archives and container images. After dependency updates,
run `npm --prefix web ci` and `python3 scripts/third-party-notices.py`; `make check`
verifies the generated notices.

### Saved searches

Use the star beside the search bar to name and save the current file or box search,
including its scope, filters, and sorting. Open it from **Saved searches** in the
sidebar to run it against the current library. The row menu can rename it, replace
its conditions with the current search, or remove it.

Saved searches belong to the library and are shared across browsers and devices.
Their operations are included in history, backups, rebuilds, and strict undo.
Missing referenced tags, boxes, or folders produce a warning; conditions are
preserved rather than silently removed. Older binaries cannot replay the new
saved-search operations once they have been written.
