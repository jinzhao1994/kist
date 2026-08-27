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

## Milestones

| | | |
|---|---|---|
| **M0** | repository groundwork | ← here |
| M1 | kernel: op log, blob store, CLI import and search | |
| M2 | integrity: `verify`, `scrub`, `rebuild`, second blob backend | |
| M3 | extraction: EXIF, thumbnails, previews | |
| M4 | web UI | first version worth showing anyone |
| M5 | history UI: browse and undo any past operation | |
| M6 | docs, packaging, v0.1 | |

## License

Apache-2.0. See [LICENSE](LICENSE).
