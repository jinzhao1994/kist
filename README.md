# kist

**English** · [中文](README.zh-CN.md)

A self-hosted personal archive. The unit is a **box** — one trip, one hospital
visit, one batch of paperwork — and a single global **tag** tree cuts across
boxes for retrieval. It holds photos and video as comfortably as it holds
passports, contracts and lab reports.

One Go binary plus SQLite. Apache-2.0.

> Download the [latest release archives](https://github.com/jinzhao1994/kist/releases/latest)
> or deploy with Docker. M0–M7 are complete; see [Milestones](#milestones).
> kist has no built-in login and retains historical originals permanently.

Use a local client for selected-content caching, offline edits and synchronization
with an authoritative library. Clients can also relay changes for other clients.
See the [offline and synchronization guide](docs/offline-client.md) and
[instance settings](docs/settings.md). These features are available from v0.1.7.

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
includes FFmpeg, ffprobe, libheif tools, Poppler and timezone data. The default named volume retains
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

**Box covers and sizes.** Use a file's context menu to set it as its box's
cover. Medium and large box cards preserve the complete preview; the box menu
can remove the association without deleting the file. Covers follow file IDs,
survive trash/restore, and participate in undo and rebuild. Folder and box sizes
are maintained logical byte totals, excluding recycled files. File cards and
lists expose tag actions and display assigned tags.

Instance-wide appearance, storage, connection and service options are managed in **Settings** and stored in `kist.toml`. See [Instance settings](docs/settings.md).

**Development version: image OCR.** Run kist and your browser on the same computer. In Settings → AI connection, continue with ChatGPT and save a default model (GPT-6 Luna is offered when available). JPEG, PNG and static WebP images can be queued individually from the OCR metadata group. Automatic OCR is off by default and processes only images imported after it is enabled. The backfill button explicitly queues missing results, skipping completed and already queued images. The serial backend queue continues after the browser closes and can be paused. Failed or interrupted jobs require explicit retry and queue resumption; there is no unbounded automatic retry. Images selected by these modes are sent to OpenAI using your ChatGPT plan. Successful text is saved as file metadata and can be copied or undone from operation history. Images are limited to 20 MiB and text to 256 KiB, without truncation. Changing the model does not rerun completed extraction. OCR text is not yet searchable.

Image descriptions are also stored as metadata (`desc:text`), independently of OCR. Set the library language (default `zh-CN`) and enable automatic descriptions for new images, or explicitly backfill existing images. Descriptions use the shared AI model and proxy; changing the model or language does not regenerate completed results. JPEG, PNG and static WebP are supported.

AI settings accept one HTTP/HTTPS/SOCKS5 proxy address; blank inherits process proxy environment variables. This routes backend authorization, model listing, OCR and image descriptions, while browser sign-in uses the browser's network. Model, automatic mode and proxy settings live in the instance-private `[ai_options]` section of `kist.toml`, outside archive synchronization. Treat proxy passwords as secrets, like connection credentials. This version requires loopback access; NAS login and client-side inference are not included.

**Display settings** separates English/Chinese language, regional number/date
formatting, display timezone and reading direction. These preferences stay in
the browser. Mantine controls support gray, green, yellow, pink, blue, and orange
primary colors, independently from Light, Dark, or Follow system appearance.
Primary color follows the instance configuration; existing browser light/dark preferences remain. System
changes apply automatically in Follow system
mode. The grid loads derived thumbnails, and details generate larger image
previews or H.264/AAC video on demand. Unsupported files keep an original
download link.

Uploads default to at most 1,000 files and 32 GiB per batch. Adjust these limits
with `--max-upload-files` and `--max-upload-bytes`. A batch is archived only after
all files pass verification. New sessions read, hash and upload together in a
worker: each source block is read once, with at most one block prefetched while
the previous request is in flight. Blocks of at most 8 MiB have independent
SHA-256 checksums; the whole-file digest is sealed at completion. The server
checks blocks, import input and final blobs before committing. The upload summary
keeps elapsed time, effective transfer speed and stage timings for the latest
attempt. Overlapping stages should not be added together.

**Pause** keeps confirmed progress; **Resume** queries the server and sends only
missing data. After refreshing or reopening the page, return to the same box and
folder and select the entire original batch again. The browser rehashes the
confirmed prefix and compares it with the server's staged bytes before continuing;
the remaining bytes are hashed while uploading. Matching names, sizes and
timestamps alone is insufficient. Older sessions retain their whole-file
preflight verification. Recovery also
works after a server restart. **Check result** can recover a committed receipt
without the originals. **Cancel** discards staging, but cannot undo an already
committed import. Metadata extraction is separate; its failure does not mean the
original files were lost.

Sessions are discardable data under `uploads/` in the library volume, not archive
history. Keep space for staging and the resulting blobs. At most eight retained
sessions reserve a combined declared size of twice the per-batch byte limit;
this is not a guarantee of free disk space. Idle sessions expire after seven
days. Successful blocks and explicit resume renew them; viewing progress does
not. Expired or canceled staging is cleaned on creation, cancellation and an
hourly sweep. Browser recovery requires site storage to be available and the
same browser profile and site address. A suspended page or locked phone may stop
uploading; continuous mobile background uploading is not guaranteed. You do not
need to watch the page, but keep it open and the device awake for uninterrupted
transfers. Resume after a suspension instead of starting over.

Refresh old pages after upgrading. The checked multipart endpoint remains
available for compatible older clients and servers, and retains its all-or-
nothing batch and receipt semantics; retrying that fallback resends the batch.
Clients never silently fall back to uploads without original-file checksums.

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

### File sets

In a box, open a folder's menu and choose **Configure file set**. Choose Live Photo, video with subtitles, or document pages, assign member roles, and save. Page order is editable; the primary file defaults to the photo, video, or first page (including nested sets), and can be overridden. The combination keeps the folder's name.

Search treats a combination as its primary file. Enable **Include files within sets** to find other members independently. Tags and metadata remain on the original files; changing the primary changes which annotations represent the combination. Use the preview's primary-file button to inspect or edit them, or open any member and follow its link back to the combination.

Projection can retain directories or flatten members with a shared, collision-safe prefix. Original blobs are unchanged. Cancelling a set keeps its folder and files. Configuration changes and whole-folder deletion support strict undo. Subtitle playback supports UTF-8 SRT and WebVTT up to 4 MiB; other subtitle files remain available as originals. Existing folders are configured manually; import does not automatically pair files.

## Derived media

Opening a grid or file requests background work: imports do not wait for image
processing. JPEG/PNG/GIF/WebP and other supported raster images use the Go
renderer; HEIC needs `heif-info` and `heif-dec` (or `heif-convert`) with an HEVC
decoder. PDF first-page thumbnails need Poppler (`pdftoppm`).
DOC/DOCX previews need LibreOffice Writer (`libreoffice` or `soffice` on PATH).
PPT/PPTX previews need LibreOffice Impress. Both need Poppler for thumbnails.
Docker includes Writer, Impress and Noto CJK, Carlito and
Caladea fonts; native installations should provide fonts for their documents.
Video (including MOV) needs `ffmpeg` and `ffprobe` with H.264/AAC encoders. Docker
includes these tools. On Ubuntu, install `poppler-utils ffmpeg libheif-examples
libheif-plugin-libde265 libreoffice-writer libreoffice-impress fonts-noto-cjk fonts-crosextra-carlito
fonts-crosextra-caladea`; native deployments may omit them and still archive,
search and download originals.

Existing MOV files classified by the older media extractor need `kist extract
--extractor media` or the UI metadata extraction action; no re-upload is needed.
PDF grids show first-page thumbnails. Details use PDF.js for paging, zooming,
search and text selection. Open a new tab to use the browser's native PDF reader,
including desktop Chrome's OCR where available. Recognized text is not written
back to the archive.

DOC/DOCX and PPT/PPTX grids show a converted first page or slide; details read a derived PDF with the
same reader. Once ready, the new-tab action opens that PDF, while download always
returns the original Office file. Existing files work without re-import or metadata
extraction. Conversion is local and cached by content and recipe. Complex layouts
and substituted fonts may differ from Microsoft Office. Slides are static: animations, transitions,
embedded audio/video and interactive elements are not played. Inputs and generated PDFs
are limited to 128 MiB, conversion to 120 seconds and scratch files to 512 MiB.
Macro-enabled DOCX/PPTX and external resource relationships are unsupported; ordinary
hyperlinks are retained. Conversion disables macros and Writer automatic link updates.


Missing tools are checked again every 30 seconds; installation takes effect
without restarting the server. Details offer cancel and retry controls.
Jobs survive restart and share outputs for identical content. Stop the server
before removing the disposable `derived/` directory; browsing regenerates
requested outputs. This never changes originals or undo history.

STL files (ASCII and binary) have static model thumbnails and detail previews,
rendered in pure Go without additional tools. Existing `.stl` files can request
previews directly; the backend validates the bytes. Limits are 32 MiB, 200,000
triangles, a bounded raster workload and 90 seconds. Previews use a fixed view
and uniform material; they do not provide interactive rotation or validate
printability. Recipes are `stl_thumb_512_v1` and `stl_preview_2048_v1`.

GPX tracks, routes and waypoints have map thumbnails and zoomable detail maps.
OpenFreeMap Liberty supplies roads and place names; Mapzen AWS Terrain Tiles add
hillshade. Neither requires an account or API key. GPX is parsed locally (32 MiB,
100,000 points maximum); coordinates and original files are never uploaded to
map providers, but tile requests reveal the viewed area. Existing `.gpx` files
work without reimporting. NAS stores `gpx_map_v1.geojson`; the browser renders
512×384 thumbnails, releases WebGL contexts, and retains at most 32 successful
snapshots per page. Reloading redraws maps using normal browser tile caching.
Unavailable backgrounds leave a route outline and a retry/download option.
This is not an offline map package.

Map providers are replaceable at startup:

```sh
kist serve --map-style https://maps.example/style.json \
  --map-resource-origin https://tiles.example \
  --map-terrain-tiles 'https://elevation.example/terrarium/{z}/{x}/{y}.png' \
  --map-terrain-attribution 'Elevation provider attribution'
```

The style URL accepts any compatible MapLibre style, including raster sources.
Repeat `--map-resource-origin` for additional tile, glyph or sprite origins;
style/elevation URL origins are allowed automatically. Same-origin absolute
paths are also supported. Elevation tiles must use Terrarium encoding at 256 px
with source maxzoom 15; `--map-terrain-tiles ''` disables elevation. External
resources must permit CORS; use HTTPS resources for HTTPS deployments. Restart
and refresh after changes. Parsed GPX data remains valid, while snapshot keys
include the provider configuration. Keep correct source attribution in the
style and elevation options.


Video thumbnails prefer embedded cover art, then a valid QuickTime poster time.
Otherwise, up to 100 small frames are sampled across the video; near-black frames
are excluded and a representative color histogram selects the result. Sampling
has a 60-second budget and uses fewer frames for short clips. This does not
identify the best scene semantically.

After upgrading, refresh the browser. Visible videos request `video_thumb_512_v2`,
which generates a new cached result even when an older thumbnail exists. Images
and PDFs keep `thumb_512_v1` because their rendering algorithm is unchanged;
uncropped display is a browser layout change. No re-upload, rebuild or wholesale
cache deletion is needed. Old cache files remain on disk without automatic cleanup.

Recipes are `video_thumb_512_v2` (video cover JPEG, longest edge 512), `thumb_512_v1` (JPEG, longest edge 512), `preview_2048_v1`
(JPEG, longest edge 2048), and `preview_1080p_v1` (H.264/AAC MP4, up to
1920×1080). Images preserve aspect ratio and orientation, flatten alpha onto
white, and show the first animation frame. Multi-image HEIF collections are
currently unsupported. Limits include 64 million pixels, 256 MiB compressed
still images and PDFs, six-hour video inputs and 2 GiB output; unsupported or oversized
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
