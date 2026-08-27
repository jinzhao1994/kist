# kist

**English** · [中文](README.zh-CN.md)

A self-hosted personal archive. The unit is a **box** — one trip, one hospital
visit, one batch of paperwork — and a single global **tag** tree cuts across
boxes for retrieval. It holds photos and video as comfortably as it holds
passports, contracts and lab reports.

One Go binary plus SQLite. Apache-2.0.

> **Not usable yet.** The design is settled and written down. You can create a
> library, make boxes, build a tag tree, and undo any of it — every change is
> an entry in an append-only log, and the whole library can be rebuilt from
> that log alone. What you cannot do yet is put files in: `kist import` is
> next. See [Milestones](#milestones).

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
  the tables. Undo appends an inverse op, so undos are themselves undoable, and
  the whole library can be rebuilt from the log.
- **Documents and photos in one archive.** Photo managers assume everything has
  an EXIF date; document managers have no idea what to do with 4000 pictures.
- **Nothing traps your data.** Files are stored content-addressed under
  `blobs/`, and `boxes/` is a human-readable projection you can browse with any
  file manager. Delete kist and the archive is still an ordinary directory
  tree.

## Design documents

Written in Chinese, and worth reading before the code:

- [`docs/design/2026-08-22-kist-v0.1-design.md`](docs/design/2026-08-22-kist-v0.1-design.md)
  — the authoritative design. It records rejected alternatives alongside every
  decision, on purpose.
- [`docs/design/open-questions.md`](docs/design/open-questions.md) — what is
  still undecided.

## Building from source

Go 1.26 or newer. No C toolchain, no other dependencies.

```sh
make check   # everything CI runs: tidy, gofmt, vet, test, build
make build   # -> bin/kist
make dist    # -> dist/, all four release platforms
```

## Using it

```sh
kist init                                  # create a library, interactively
kist box new "Yunnan" --start 2019-08-12   # a box is one trip, one visit, one batch
kist import --box Yunnan ~/Pictures/2019/  # copy files in; Ctrl-C is safe and resumable
kist ls Yunnan                             # what is in there
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

kist never writes to a remote, and never trusts one: content fetched from it is
hashed on the way in and discarded if it is not what was asked for. Run
`kist remote list` to check a path today rather than on the day you need it.

## Milestones

| | | |
|---|---|---|
| M0 | repository groundwork | done |
| M1 | kernel: op log, blob store, CLI import and search | done |
| M2a | integrity: `verify`, `scrub`, `repair`, `rebuild` | done |
| **M2b** | the query contract and its conformance suite | ← here |
| M3 | extraction: EXIF, thumbnails, previews | |
| M4 | web UI | first version worth showing anyone |
| M5 | history UI: browse and undo any past operation | |
| M6 | docs, packaging, v0.1 | |

## License

Apache-2.0. See [LICENSE](LICENSE).
