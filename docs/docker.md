# Docker and NAS deployment

Public images on Docker Hub (`jinzhao1994/kist`) and GHCR
(`ghcr.io/jinzhao1994/kist`) support `linux/amd64` and
`linux/arm64`. Docker chooses the host architecture: amd64 for an x86 NAS,
arm64 for Docker on Apple Silicon. Both images include FFmpeg, ffprobe, libheif
and timezone data, and run as UID/GID `10001:10001`.

For DSM web-only setup with a dedicated user and a shared-folder bind mount,
see the [Synology guide (Chinese)](synology.zh-CN.md).

## Install a new library

Save the repository's `compose.image.yaml` in a persistent deployment directory.
It is standalone; no source build is needed. Keep this directory and Compose
project name stable because they determine the default named volume's name.

Both registries allow anonymous pulls; no publishing token or `docker login` is
needed. For Synology, prefer Docker Hub for integration with its update UI. The
Compose file retains its existing GHCR default; set `KIST_IMAGE` in `.env` to
`jinzhao1994/kist:latest` for current stable releases or `jinzhao1994/kist:v0.1.2`
for a fixed version. Do not overwrite an existing `.env`; edit only this setting.

For local access, the listener defaults work without extra settings. For a trusted LAN, create
`.env` next to the Compose file, substituting your NAS address:

```dotenv
KIST_IMAGE=jinzhao1994/kist:latest
KIST_BIND=192.168.1.10
KIST_PORT=8473
KIST_ORIGIN=http://192.168.1.10:8473
TZ=Asia/Shanghai
```

`KIST_BIND` is a host interface address; `KIST_ORIGIN` is the exact browser origin
(scheme, host and port, without a path). Defaults bind only `127.0.0.1`.
kist has no login: use a trusted LAN or an authenticated reverse proxy.
Do not expose it directly to the internet.

```sh
docker compose -f compose.image.yaml pull
# New libraries only; choose TZ before this command.
docker compose -f compose.image.yaml run --rm kist init /library --yes
docker compose -f compose.image.yaml up -d
docker compose -f compose.image.yaml logs --tail=50 kist
```

Open the configured origin. Check the library with:

```sh
docker compose -f compose.image.yaml exec kist kist --library /library verify --deep
```

The whole library lives in the named `library` volume. Container replacement and
`down` retain it; `down -v` deletes it. For an existing library, replace
`library:/library` with its absolute host path and ensure UID/GID `10001:10001`
can access it, or set `user:` to its existing owner. Do not initialize it again.
Mount the entire library, not just its database or blobs. Startup does not
create a missing library or change ownership recursively.

## Upgrade and pinning

Read the target version's release notes and [backup/upgrade procedure](operations.md)
first. Routine compatible upgrades do not require a fresh full-library backup.
Keep scheduled backups independently; create a recovery point before destructive
migrations or when older-version rollback is needed. Stop all writers for the
upgrade. Do not run initialization again.

Set `KIST_IMAGE` to the target `vX.Y.Z`, then run from the same deployment directory:

```sh
docker compose -f compose.image.yaml pull
docker compose -f compose.image.yaml up -d
```

Check the running version, startup logs, and access to existing files. Run
`verify --deep` if release notes require it or integrity warnings appear. Full
blob `scrub` is a scheduled maintenance check, not a routine upgrade step.

Release tags are never overwritten by the publication workflow. To also pin the
registry content, copy the multi-platform index digest from the successful
**Container Release** run's summary and use
`KIST_IMAGE=ghcr.io/jinzhao1994/kist@sha256:...`. Docker still selects the matching
architecture.

| Tag | Updated when | Intended use |
| --- | --- | --- |
| `vX.Y.Z` | Once, for that stable release | Reproducible NAS deployments |
| `latest` | GitHub's latest stable release image passes native acceptance on both architectures | Following stable releases |

GHCR images are published at `ghcr.io/jinzhao1994/kist`, with Docker Hub copies
at `jinzhao1994/kist`. Channel updates reuse the
verified multi-platform index digest. Old queued runs recheck the current source
before promotion, so backfilling an older release cannot roll back `latest`.
The **Container Latest** workflow summary records the source revision and digest.
It also supports manual dispatch on main for initial publication or recovery.
Daily pushes to main run checks without publishing development images. The old
`edge` tag is no longer updated; use a stable release or build locally.

To opt in, set `KIST_IMAGE=ghcr.io/jinzhao1994/kist:latest` and use the same
pull/recreate commands above. Running containers do not update themselves;
there is no automatic upgrade service.

Downgrades are not guaranteed; restore the matching backup if a newer version
has introduced incompatible data or operations. Keep the same volume and project
name across upgrades and never use `down -v` as an upgrade step.

## Build from source

The original `compose.yaml` still builds `kist:local`; use `docker compose build`
and the README's initialization commands. It accepts the same runtime settings.
CI compares both Compose configurations to prevent accidental divergence.

## Docker Hub

The **Container Docker Hub** workflow mirrors versions and `latest` to
`docker.io/jinzhao1994/kist` without rebuilding; successful copies have the same
index digest as GHCR. Check that workflow completed for the desired tag before
switching `KIST_IMAGE` to `jinzhao1994/kist:latest` or a fixed version. Keep the
volume, UID/GID, ports and origins unchanged. Public pulls need no publishing token.

Synology may not show update notifications for GHCR images. Docker Hub offers
better integration with its image update UI. A missing update button does not
prove that GHCR is current; changing the project YAML to a new explicit version
is also supported. See the [Synology guide](synology.zh-CN.md).
