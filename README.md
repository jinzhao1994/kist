# kist

**English** · [中文](README.zh-CN.md)

A self-hosted personal archive. The unit is a **box** — one trip, one hospital
visit, one batch of paperwork — and a single global **tag** tree cuts across
boxes for retrieval. It holds photos and video as comfortably as it holds
passports, contracts and lab reports.

One Go binary plus SQLite. Apache-2.0.

> **In development.** The storage kernel, import, integrity tools, queries and
> metadata extraction are implemented. M4's HTTP API and embedded English/Chinese
> Web UI have passed browser and end-to-end acceptance testing. Derived media is
> under development in M5. See [Milestones](#milestones).

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

- **Every operation is undoable.** The authority is an append-only op log, not
  the database. Undo appends an inverse op, so undos are themselves undoable,
  and the whole database can be rebuilt from the log.
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

## Building from source

Go 1.26 or newer and Node.js 24 or newer are needed to build from source.
The resulting binary includes the web assets and needs neither Node.js nor a C toolchain.

```sh
npm --prefix web ci
make check   # frontend checks plus tidy, gofmt, vet, race tests, and build
make build   # -> bin/kist
make dist    # -> dist/, all four release platforms
```

## Docker (with FFmpeg)

Build and initialize a new, persistent library explicitly:

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
the port binding and setting `KIST_ORIGIN` to the trusted browser origin; use
only a trusted network or an authenticated reverse proxy because kist has no
login. The default exposes the service only on the host's loopback address.

To use an existing library, replace the named volume with an absolute host
path and ensure UID/GID `10001:10001` can access it, or configure `user:` for its
existing owner. Mount the entire library, not just `kist.db` or `blobs/`.
There is no automatic library creation or recursive ownership change.

FFmpeg is bundled for M5 development; this packaging step does not yet add
thumbnail or transcoding features to the UI. A registry image is not published
yet. Run `sh scripts/docker-smoke.sh kist:local` to check the built image's
encoders, web app, access checks and persistent library.

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

Everything is undoable, because every write is an entry in an append-only log:

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

**Display settings** separates English/Chinese language, regional number/date
formatting, display timezone and reading direction. These preferences stay in
the browser. Choose Old Paper, Moonlit Moss, Golden Fields, Cherry Rain, Ocean Whispers, or
Apricot Dusk independently from Light, Dark,
or Follow system appearance; system changes apply automatically in Follow system
mode. Unsupported previews, including HEIC, have a download fallback;
thumbnail generation belongs to M5.

Uploads default to at most 1,000 files and a 2 GiB multipart body per batch.
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
it back. Errors stay as plain text on stderr with a non-zero exit status.

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

## Milestones

| | | |
|---|---|---|
| M0 | repository groundwork | done |
| M1 | kernel: op log, blob store, CLI import and search | done |
| M2a | integrity: `verify`, `scrub`, `repair`, `rebuild` | done |
| M2b | the query contract and its conformance suite | done |
| M3 | metadata extraction, including EXIF | done |
| M4 | HTTP API and embedded web UI | done |
| **M5** | derived media: thumbnails, previews, transcoding; Docker with FFmpeg | in progress |
| M6 | history UI: browse and undo any past operation | |
| M7 | docs, packaging, v0.1 | |

## License

Apache-2.0. See [LICENSE](LICENSE).
