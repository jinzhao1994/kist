# Laptop client development commands

The development build provides NAS pairing, local status and manual synchronization.
The offline web interface and automatic content-cache management are still being
implemented; these commands alone are not the complete offline workflow.

## Authorize a client

On the NAS, create an independently revocable read/write synchronization credential:

```sh
kist --library /path/to/nas-library sync-access create
kist --library /path/to/nas-library sync-access list
```

Creation displays the secret once. Listing shows only its ID and creation time.
The NAS stores a digest in `sync-access.json`; credentials do not become archive
operations and cannot be restored through archive undo.

Run the NAS server with its usual explicit allowed-host configuration. On the
laptop, supply the secret through standard input, for example from a private
credential file:

```sh
kist client init /path/to/laptop-client --server https://nas.example \
  --token-stdin < /path/to/private-credential.txt
kist client sync /path/to/laptop-client
kist client status /path/to/laptop-client
```

`init` authenticates the NAS and binds the client to its library UUID. It does not
download the catalog; `sync` performs that step. The client directory is separate
from a NAS library and cannot be opened through normal NAS commands. No default
personal library path is used for client commands.

HTTPS is required outside loopback unless `--allow-http` explicitly permits a
plain HTTP connection. The client does not follow redirects. Credentials are
saved in the client's private `connection.json`, with mode 0600 on POSIX systems.
Do not put the credential in the server URL or command-line arguments.

## Rotate a credential or server address

Create a replacement NAS credential, then update the laptop connection:

```sh
kist client connect /path/to/laptop-client --server https://nas.example \
  --token-stdin < /path/to/replacement-credential.txt
kist --library /path/to/nas-library sync-access revoke OLD_CREDENTIAL_ID
```

`connect` verifies the same library UUID before replacing the configuration.
It cannot reassign pending changes to another NAS library. Revocation is read
on each new NAS request, including by an already-running server.

## Inspect and retry synchronization

```sh
kist --json client status /path/to/laptop-client
kist --json client sync /path/to/laptop-client
```

Status works without a network connection. It includes the confirmed catalog
boundary, any partial catalog download, and pending operations. An operation
awaiting a receipt may already have committed on the NAS; it cannot be discarded
until synchronization establishes the outcome.

Run `sync` again after a connection failure. Original uploads resume from durable,
verified offsets. Retrying a committed operation recovers its existing receipt
instead of creating another archive operation. A failed synchronization returns
a nonzero exit status; use `status` to inspect local pending work.

The NAS can continue to change independently while the laptop is offline. Merge
uses operation order and explicit preconditions, not timestamp-based overwrites.

## Manage the content cache

These commands work without contacting the NAS:

```sh
kist client cache status /path/to/laptop-client
kist client cache configure /path/to/laptop-client --max-age 720h --max-bytes 10737418240
kist client cache prune /path/to/laptop-client
kist client cache rebuild /path/to/laptop-client
```

Capacity includes actual cached bytes and the remaining space reserved for partial
downloads. Retention age is measured from the last access. Pruning removes expired
or least recently used eligible content, while skipping active readers, transfers
and explicitly retained selections. Lowering the limit can therefore report
`over_limit` without deleting protected content.

`rebuild` recovers tracking records from known original facts and cache files.
It reports verified, partial, corrupt, missing and in-use objects separately.
Corrupt content stays unavailable until fetched again; skipped active objects
can be checked by a later rebuild. Offline selections, pending operations and
protected local originals are not removed by cache pruning or rebuilding.

These byte totals describe the disposable content cache, not total laptop disk
usage. Accounting and lifecycle management of protected local originals, offline
selection controls and the web interface remain under development.
