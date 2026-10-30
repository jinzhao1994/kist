# Installing and operating kist

kist v0.1 has no login. Its default listener is `127.0.0.1:8473`.
Use a trusted network or an authenticated reverse proxy for remote access.
The service account can read and modify every file in its library.

## Install a binary

Download an archive and `SHA256SUMS` from the same
[GitHub Release](https://github.com/jinzhao1994/kist/releases/tag/v0.1.0).
An untagged Release workflow's `dist` artifact is a development build, not a
published version.

| Machine | Archive |
|---|---|
| Apple Silicon Mac | `kist_darwin_arm64.tar.gz` |
| Intel Mac | `kist_darwin_amd64.tar.gz` |
| Linux x86-64 | `kist_linux_amd64.tar.gz` |
| Windows x86-64 | `kist_windows_amd64.zip` |

On Linux, check the downloaded archive against its line in `SHA256SUMS` using
`sha256sum`; on macOS use `shasum -a 256`. On Windows use
`Get-FileHash .\kist_windows_amd64.zip -Algorithm SHA256` in PowerShell.
The hash must match before extraction. These hashes detect corruption; they
are not a separate signature or proof of publisher identity.

Extract the archive. On macOS/Linux, run `./kist version`; on Windows run
`.\kist.exe version`. You can put the binary in a directory on your PATH to use
`kist` directly. No Go, Node.js, SQLite server or C toolchain is required.
Binaries are not code-signed or notarized. Follow your operating system's
normal approval flow only after checking the download source and checksum;
do not disable system-wide security controls.

Create a new library once, then start the server:

```sh
kist init /absolute/path/to/library --yes
kist --library /absolute/path/to/library serve
```

Replace the path with a writable local directory (on Windows, for example,
`C:\Users\you\kist`). Open `http://127.0.0.1:8473`. Stop with Ctrl-C.
`serve` runs in the foreground; it does not install a background service.
Docker Compose provides restart management; native service supervision is
managed by your operating system.

For video previews, install `ffmpeg` and `ffprobe` with H.264/AAC encoding.
HEIC needs libheif's `heif-info` and `heif-dec` or `heif-convert`, with an HEVC
decoder. Missing tools disable those previews, not storage or downloads.
The Docker build includes and tests these dependencies; see README's Docker
instructions. For prebuilt images and NAS deployment, see [Docker deployment](docker.md).

### Thumbnail and preview cache

NAS thumbnails and non-video previews share a separate cache, limited to 32 GB
by default, with no age limit. Settings → Storage accepts a size or duration and
offers **No limit**. Omit a setting to inherit its default; explicitly setting
`"inf"` disables that limit even if the default changes later.

```toml
[previews]
cache_max_age = "inf"
cache_max_bytes = 32000000000 # 32 GB; also accepts "inf"
```

Eviction uses last access time, protects active reads and sync transfers, and
never removes originals, archive metadata or execution history. Evicted outputs
are regenerated when requested. Existing caches get a fresh retention window
when upgraded. A single output larger than the configured quota, or a quota
occupied by active readers, can fail generation with `cache_quota`; retry after
adjusting the limit or after readers finish. Client offline cache limits and video
playback cache limits remain separate.

### Video playback cache

NAS video playback uses HLS with on-demand six-second H.264/AAC segments.
Only requested segments are converted; seeking can generate a later segment
without converting the beginning first. Encoding is limited to one process per
instance, independently of thumbnail generation and metadata extraction.

Settings → Storage exposes the separate video cache limits and a clear action.
The equivalent optional `kist.toml` settings are:

```toml
[video]
cache_max_age = "30d"
cache_max_bytes = 8589934592 # 8 GiB
```

Omitting either value follows its current default. Changes take effect without
restarting. HLS segments live in `transcodes/`; retention is based on last access
and eviction skips actively read or generated segments. Temporary encoding output
is bounded separately to 32 MiB. Clearing this cache leaves originals, thumbnails,
metadata and execution history intact. Existing full MP4 previews and offline
client video preparation remain available through the earlier derived-media API;
the HLS cache clear action does not remove them.

## Back up and restore

1. Stop every writer, including `serve`, CLI imports, extraction, and repair.
   Keep them stopped for the entire copy.
2. Copy the entire instance directory into a **new** backup directory. This
   preserves `runtime.db` (and any remaining WAL/SHM files), local choices and
   execution history, upload staging, and client pending content as well as the
   archive. Retain your previous backup until the new copy is complete.
3. Restore these into a separate directory and verify the restored copy:

```sh
kist --library /absolute/path/to/restored rebuild --yes
kist --library /absolute/path/to/restored relink
kist --library /absolute/path/to/restored verify --deep
kist --library /absolute/path/to/restored scrub
```

All four commands must succeed before switching to the restored library.
`ops/` and `blobs/` are authoritative. `kist.toml` preserves local settings;
`kist.db`, `boxes/` and rendered previews can be regenerated. `runtime.db`
cannot be reconstructed from archive operations. A blob-only copy can
repair file content but cannot restore names, tags or history.

For Docker, stop the service and any one-off containers that write the volume
before copying it. Back up the whole named volume if that is easier. Do not
use `docker compose down -v`: it deletes the library volume. Mount the restored
library with an owner accessible to the configured container user (default
UID/GID `10001:10001`). Never initialize over a backup or existing library.

An archive-only backup of `kist.toml`, `ops/` and `blobs/` remains sufficient to
recover archived content and history, but loses local jobs, model responses,
retention choices and transfer progress. To intentionally start fresh local state,
restore only those three items into a **new directory**, run the archive checks
above, and remove the `runtime_storage` setting from that restored configuration
before starting `serve`. Do not copy legacy runtime JSON into this fresh instance.
Do not clear this setting or delete `runtime.db` to bypass errors in an existing
instance. Client backups must also preserve `pending/` and unconfirmed original
content; an archive-only backup is not a client recovery procedure.

## Upgrade or roll back

1. Read the target version's release notes and compatibility requirements.
2. Stop all writers. Routine compatible upgrades do not require a new full-library
   backup. Keep a regular backup schedule independent of upgrades. Create a
   verified recovery point first if the release requires a destructive migration
   or you need to roll back to a version that cannot read newly written actions.
3. Retain the previous executable or container image; replace the executable
   with the verified new one. For a source-built Docker deployment, check out
   the desired version and run `docker compose build` while the service is stopped.
4. Start `serve` (or recreate the container with `docker compose up -d`) and
   check the version and startup logs. Run `verify --deep` when the release notes
   require it or an integrity warning appears. Full-content `scrub` belongs to
   scheduled integrity checks, not every application upgrade.
5. Verify that boxes, a sample original download and history are accessible.

There is no general downgrade guarantee. To roll back safely, stop all writers,
restore the pre-upgrade backup into a separate directory and run the previous
version against it. Retain the newer library separately: writes after the
backup are not present in the restored copy. Do not edit or truncate journals
in an attempt to make a newer library readable by an older binary.

## Runtime storage upgrade

Stop every old service and CLI writer before first opening the upgraded instance.
Local state is imported into `runtime.db` under the existing subsystem locks; each
subsystem commits its data and import marker together. The old JSON files remain
unchanged as pre-upgrade backups. They are not updated or reused after import.
Malformed legacy state stops that import without marking it complete.

The `runtime_storage = 1` configuration marker prevents older binaries from
opening the migrated instance and prevents silent recreation of a missing runtime
database. Restore a matching local-state backup if the database is missing or
corrupt. `rebuild`, snapshots and `verify --deep` apply to archive views, not this
local database. Rolling back requires the stopped pre-upgrade instance backup.

Extraction attempts retain model text locally, including rejected structured
output (up to 8 MiB per attempt, with truncation and original-size indicators).
Treat instance backups as private. Execution lists omit the text; use
`GET /api/v1/files/{id}/{kind}/attempts` for the latest 100 records and
`?before=<sequence>` for older records, then
`GET /api/v1/files/{id}/{kind}/attempts/{request_id}` for explicit details.
Supported extraction kinds are `ocr`, `desc`, `facts`, `orientation` and `geo`.
History is not automatically pruned or synchronized. Orientation currently records
the provider's parsed orientation result; requests that failed before a result
have no model output. Existing legacy queues cannot supply past model responses.

## Troubleshooting

- **Connection refused:** `serve` is not running or is listening on another
  address. Inspect its startup output and the configured port.
- **403:** verify the actual Host and Origin against the explicit listener and
  `--allow-origin` settings. Include the scheme and port; these checks do not
  replace authentication. Do not globally disable them.
- **Preview blocked:** install the reported media dependency. It is detected
  again automatically. Failed or unsupported previews retain original downloads.
- **Projection warning:** run `relink`; it rebuilds the human-readable `boxes/`
  projection from the library. Do not modify original content via these links.
- **Integrity findings:** keep the library and logs, stop unnecessary writes,
  and inspect `verify --deep` and `scrub`. `repair` needs another correct blob
  copy configured as a remote. `rebuild` repairs derived tables, not missing
  journals or corrupted original bytes.
- **Undo blocked:** inspect the blocking operation. Undo is atomic and strict;
  restoring a batch restores the whole deletion event. v0.1 keeps history and
  original blobs permanently; deleting from the UI does not reclaim disk space.

Commands support `--help`; `--json` returns machine-readable results and errors.

## Folder-size schema upgrade

The folder-size feature adds schema migration 5. Stop all old writers, replace the binary (or recreate the container with the same library mount), and start kist normally. No separate upgrade command is required. Startup holds the library write lock and runs embedded migrations before serving requests. The first startup calculates existing folder totals from SQLite file/blob metadata, without reading original file contents. Large libraries may take longer; logs report the upgrade start and completion.

The column, backfill and migration version commit together. If migration fails, startup stops; fix the reported cause and restart to retry. A failed lock acquisition is also retryable. Do not remove the database or edit its migration version to bypass an error. Retain the normal library backup before upgrading.

Totals count current logical file bytes, including repeated references to identical content, and exclude deleted files, old versions and derived previews. Box totals use their root folder. Triggers maintain totals in the same write transaction, including legacy SQL paths. This does not make downgrades generally supported: old binaries cannot enforce the new schema-version guard and must not be used to rebuild the upgraded database. New binaries reject schemas newer than their embedded migrations. `kist verify --deep` checks totals independently; `kist rebuild` with the new binary restores them from the journal.
