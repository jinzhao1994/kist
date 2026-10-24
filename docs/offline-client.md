# Laptop offline client (development build)

[中文](offline-client.zh-CN.md)

The development build provides NAS pairing, a local web interface, selected-content
caching, offline edits, automatic one-way synchronization and manual merging. Functional, fault-recovery and visual
acceptance are complete; this feature has not been released. See the
[acceptance evidence (Chinese)](plans/2026-10-22-offline-client-acceptance.md).

## Authorize a client

On the NAS, create an independently revocable read/write synchronization credential:

```sh
kist --library /path/to/nas-library sync-access create
kist --library /path/to/nas-library sync-access list
```

Creation displays the secret once. Listing shows only its ID and creation time.
The NAS stores a digest in `kist.toml`; credentials do not become archive
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
saved in the client's private `kist.toml`, with mode 0600 on POSIX systems.
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

## Open the laptop web interface

```sh
kist client serve /path/to/laptop-client --listen 127.0.0.1:8474
```

Open `http://127.0.0.1:8474`. The local binary serves the web assets, so the NAS
does not need to be reachable to open the interface. Synchronize the catalog
before leaving, then prepare selected content as described below. An uncached
original is shown as not cached, rather than as a missing NAS file.

Use **Offline client** to inspect pending work, synchronize manually, manage
retained content or update the connection. The NAS remains the only authoritative
archive and may change independently during the trip. Offline searches use the
last complete synchronized catalog, including objects whose originals were not
downloaded; they cannot show NAS edits made after that boundary.

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

Initialization, reconnection and synchronization inherit the NAS library's
timezone for future local commands. Existing pending payloads are not rewritten
when that setting changes. The server must advertise an explicit portable zone
such as `Asia/Shanghai` or `UTC`; a missing zone, `Local`, or an unavailable zone
is rejected. Upgrade an older sync server or set an explicit NAS library timezone
instead of relying on the laptop's machine timezone.

## Manage the content cache

These commands work without contacting the NAS:

```sh
kist client cache status /path/to/laptop-client
kist client cache configure /path/to/laptop-client --max-age 180d --max-bytes 10737418240
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

Rebuilding also recovers accounting when a disposable preview mapping is lost or
malformed. It preserves the bytes and requires a matching full content hash before
marking an unindexed object complete. A missing mapping still means the preview
is unavailable; prepare it again to obtain a valid descriptor and reuse verified
bytes. Unknown metadata versions are rejected rather than overwritten.

These byte totals describe the disposable content cache, not total laptop disk
usage. Preview outputs and conversion workspaces share the cache budget. Protected
local originals are counted separately in client status. The complete offline
workflow and fault-recovery acceptance are still under development.

## Prepare selected content for offline use

Save a named selection using IDs from the synchronized catalog. Files can belong
to different boxes; a fileset uses its folder ID. Repeat either flag as needed:

```sh
kist client offline select /path/to/laptop-client trip --file FILE_UUID --fileset FILESET_FOLDER_UUID
kist client offline prepare /path/to/laptop-client trip
kist --json client offline status /path/to/laptop-client trip
kist client offline list /path/to/laptop-client
kist client offline remove /path/to/laptop-client trip
```

`select` replaces that name's selection and retains its current originals without
downloading them. Filesets include configured members, nested filesets and an
explicit primary, including all Live Photo components and subtitle tracks.
Unrelated files in the same directory are not included automatically. Overlapping
selections share content; removing one does not cancel another's retention.

`prepare` resumes missing original downloads and prepares the thumbnail and detail
previews used by the web interface. It uses local conversion tools when available
and can fetch derived previews from the NAS otherwise. New local
originals awaiting upload are already usable and are not duplicated into cache.
`status` works offline and reports verified availability against the captured
local view and its catalog boundary. It does not assert that the NAS has no newer
changes. Both commands resolve logical targets again; run them after syncing or
editing a selection's members to check or download newly required originals.
Eviction also refreshes logical selections under the editing lock before removing
bytes, so newly selected cached members remain protected without another prepare.
A missing target is an error, not an empty successful selection; its previous
retention is preserved until the selection is corrected or removed.

Readiness requires both originals and required previews to be available. The
status reports their counts separately. Missing conversion tools or an unreachable
NAS can leave preparation incomplete; retry after providing either. Already
prepared content needs no NAS connection. This includes nested fileset members,
while external map backgrounds may still be unavailable offline.
Removing a selection leaves its cached bytes eligible for normal pruning; it does
not delete archive files or unsynchronized originals.


The web client's **Offline client → Cache** tab lists retained selections,
including named selections created with the CLI. Selections created from a file
menu display the current filename. If a target can no longer be resolved, the
selection remains visible and can still be removed. **Check availability**
verifies originals and previews on demand, rather than scanning all content when
the tab opens. **Check and download** resumes the existing selection without
changing its members. **Stop retaining offline** removes only that selection's
protection; other selections may still retain shared content, and archive files
are not deleted.


To prepare several items from the web UI, open a file list and choose **Select
files for offline use** beside the display-mode control. Click file or file set
cards to select them, or choose **Select loaded items**. This checkbox includes
only the currently loaded results, not unrequested search pages. Choose **Keep
offline**, review the selection, and start **Retain and download**. Every selected
item receives its own retention entry before downloads begin. The dialog shows
how many entries have been retained and prepared. Canceling or losing the
connection keeps completed work; **Check and download** resumes by verifying
current dependencies and skips content already ready. These entries can then be
managed independently in the Cache tab.


Manual cache cleanup also retries migration of NAS-confirmed originals that were
still being read during sync. Once those readers close, the bytes become eligible
for the configured cache policy without another network sync. Retained items stay
protected even above the limit; unconfirmed local originals remain outside eviction.

When synchronization stops on a conflicting box cover, choose **Review local
change**. The editor shows the previous and current references and starts with
your local choice. Select an available compared file or **No cover**; unavailable
files cannot be submitted. Save, then synchronize again. A newer NAS edit can
invalidate the reviewed choice and requires another review. The original local
intention stays in history, and retrying an uncertain save sends the same decision.
To use a different file, use its existing **Set as box cover** action after
resolving the blocked change.

For conflicting box, directory or file deletions, the review compares the original
and current scope and links to the current contents. Confirm the current scope
explicitly before saving: it may include files added on the NAS and newer
annotations. Saving queues a replacement; the next sync rechecks its exact scope
and related file set references. A newer NAS change invalidates the review. If
the target is already gone, return and keep the NAS result. The original intent
remains in history; undoing an accepted replacement restores the scope actually
reviewed, including the newer files and annotations.

The **Local operations** tab shows object names and change summaries directly in
the paginated list. Names recorded in the original intent remain unchanged after
later edits or undo. Summaries that resolve current object names are labeled as
such. Open an entry for its original identity, sync receipt, related operations,
and available undo action.

Connection and cache policies are under **Settings**. Synchronization, offline content and local history are under **Data sync**. Persistent configuration is consolidated in this instance’s `kist.toml`; see [Instance settings](settings.md).

## Automatic synchronization while online

While the client service is running, it checks at startup and every 30 seconds; completed local web edits trigger an immediate check. A browser status check that finds one-way pending changes also wakes synchronization immediately. Remote-only changes are downloaded automatically, and local-only changes are uploaded automatically. Independent changes on both sides or recorded conflicts require a manual merge under **Data synchronization**. Closing the browser does not stop the service worker. Older remotes without conditional submission support still require manual uploads.

The sidebar distinguishes **Synchronizing…**, **Up to date**, and **Manual merge required**, and hides zero operation counts. Counts refer to operations, not files. Original files remain cached on demand. File lists refresh when synchronization advances the local catalog.

Cache retention defaults to `180d`. Integer `d` values represent 24-hour days; existing durations such as `720h` remain supported. Explicit settings are preserved.

The client service cleans eligible cache entries at startup and every hour, including while offline. The web UI needs no manual cleanup; the prune API and CLI remain available. Retained, pending and actively used content stays protected.

## Use another client as a relay

A client can use another client of the same library as its upstream. For example,
a phone-role client connects to a laptop, which connects to the authoritative
library. This does not include a native phone application.

Create a separate downstream credential in the laptop's **Settings → Access
credentials**. Initialize the downstream with `client init --server` pointing to
the laptop, or change an existing client's connection in Settings. Use the
laptop's credential, not the authoritative library's credential. The laptop's
service must be reachable under the usual host and transport configuration.

Both clients can exchange temporary operations and originals while the library
is unavailable. **Awaiting root confirmation** means the relay has durable
custody, not that the library has committed the operation. Protected originals
remain outside the evictable cache. Resolve conflicts at the upstream holding
the operation. Once the laptop syncs to the library, sync the downstream again
to receive final confirmation.

Single-direction changes synchronize automatically; changes on both sides require
a manual merge. Changing an address for the same node preserves its temporary
branch. Before switching to another relay or directly to the library, retrieve
all final outcomes from the previous relay. Pending custody, uncertain submissions
and incomplete relay snapshots block node changes. Different libraries, divergent
known history, self-connections and cycles are rejected.
