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

## Back up and restore

1. Stop every writer, including `serve`, CLI imports, extraction, and repair.
   Keep them stopped for the entire copy.
2. Copy `kist.toml`, `ops/`, and `blobs/` into a **new** backup directory.
   Retain your previous backup until the new copy is complete.
3. Restore these into a separate directory and verify the restored copy:

```sh
kist --library /absolute/path/to/restored rebuild --yes
kist --library /absolute/path/to/restored relink
kist --library /absolute/path/to/restored verify --deep
kist --library /absolute/path/to/restored scrub
```

All four commands must succeed before switching to the restored library.
`ops/` and `blobs/` are authoritative. `kist.toml` preserves local settings;
`kist.db`, `boxes/` and `derived/` can be regenerated. A blob-only copy can
repair file content but cannot restore names, tags or history.

For Docker, stop the service and any one-off containers that write the volume
before copying it. Back up the whole named volume if that is easier. Do not
use `docker compose down -v`: it deletes the library volume. Mount the restored
library with an owner accessible to the configured container user (default
UID/GID `10001:10001`). Never initialize over a backup or existing library.

## Upgrade or roll back

1. Read the target version's release notes and compatibility requirements.
2. Stop all writers and make a verified backup using the steps above.
3. Retain the previous executable or container image; replace the executable
   with the verified new one. For a source-built Docker deployment, check out
   the desired version and run `docker compose build` while the service is stopped.
4. Run `kist version`, `verify --deep`, and `scrub` against the library using
   the new version, then start `serve` (or `docker compose up -d`).
5. Verify that boxes, a sample original download and history are accessible.

There is no general downgrade guarantee. To roll back safely, stop all writers,
restore the pre-upgrade backup into a separate directory and run the previous
version against it. Retain the newer library separately: writes after the
backup are not present in the restored copy. Do not edit or truncate journals
in an attempt to make a newer library readable by an older binary.

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
