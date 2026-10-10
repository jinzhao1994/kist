# Docker and NAS deployment

The GHCR image `ghcr.io/jinzhao1994/kist:v0.1.0` supports `linux/amd64` and
`linux/arm64`. Docker chooses the host architecture: amd64 for an x86 NAS,
arm64 for Docker on Apple Silicon. Both images include FFmpeg, ffprobe, libheif
and timezone data, and run as UID/GID `10001:10001`.

## Install a new library

Save the repository's `compose.image.yaml` in a persistent deployment directory.
It is standalone; no source build is needed. Keep this directory and Compose
project name stable because they determine the default named volume's name.

The package is private, matching the repository. Log in with an account granted
read access to the package. Use a classic personal access token with
`read:packages` as the password (and authorize SSO if your organization requires it):

```sh
docker login ghcr.io -u YOUR_GITHUB_USERNAME
```

Enter the token at the password prompt, not in the command or `.env`. A NAS
registry interface can use the same registry, username and token. Package access
must be granted separately if the account does not inherit it from the repository.

For local access, defaults work without an `.env` file. For a trusted LAN, create
`.env` next to the Compose file, substituting your NAS address:

```dotenv
KIST_IMAGE=ghcr.io/jinzhao1994/kist:v0.1.0
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
first. Stop all writers and back up the whole library before upgrading; retain
the backup until verification is complete. Do not run initialization again.

Set `KIST_IMAGE` to the target `vX.Y.Z`, then run from the same deployment directory:

```sh
docker compose -f compose.image.yaml pull
docker compose -f compose.image.yaml up -d
docker compose -f compose.image.yaml exec kist kist --library /library verify --deep
```

Release tags are never overwritten by the publication workflow. To also pin the
registry content, copy the multi-platform index digest from the successful
**Container Release** run's summary and use
`KIST_IMAGE=ghcr.io/jinzhao1994/kist@sha256:...`. Docker still selects the matching
architecture. There is no `latest` alias or automatic upgrade service.

Downgrades are not guaranteed; restore the matching backup if a newer version
has introduced incompatible data or operations. Keep the same volume and project
name across upgrades and never use `down -v` as an upgrade step.

## Build from source

The original `compose.yaml` still builds `kist:local`; use `docker compose build`
and the README's initialization commands. It accepts the same runtime settings.
CI compares both Compose configurations to prevent accidental divergence.
