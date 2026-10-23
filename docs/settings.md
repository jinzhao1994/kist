# Instance settings

Open **Settings** in the main navigation on either a library or an offline client.
Changes are saved in that instance's `kist.toml`; data synchronization does not
copy settings between instances. **Display settings** remains browser-specific:
language, regional formats, display time zone, reading direction and light/dark
mode. The primary color is an instance setting.

The file is sparse: omitted options use defaults. **Restore default** removes the
explicit option when you save. A typical initialized library needs only its
version, mode, identity, time zone and chosen projection mode. Keep the generated
identity; do not copy a client identity to another independent client.

Settings are grouped into Appearance, Storage, Connection (clients) or Sync
credentials (libraries), and Service. Storage includes client cache age and size
limits, or the library's capture-time fallback zone and projection mode. Service
includes listen address, allowed web origins, upload limits and
map resources. The UI displays current defaults and marks options requiring a
restart. Explicit command-line flags override stored service settings without
rewriting the file; the UI shows their effective values. Projection changes do
not rebuild existing paths: use `kist relink` when necessary.

A minimal custom section can be added to the existing file:

```toml
[appearance]
primary_color = "green"

[server]
listen = "127.0.0.1:8473"
```

Supported colors are gray, green, yellow, pink, blue and orange. Service defaults
are loopback port 8473 for libraries and 8474 for clients, 32 GiB per upload batch
and 1000 files. Client cache defaults are `cache_max_age = "720h"` and
`cache_max_bytes = 10737418240` under `[client]`. Cache limits do not evict pinned
content, pending changes or newly imported originals. Reducing limits applies to
subsequent cache maintenance; **Data sync → Offline content** can prune now.

New-format files use UTC if `library.timezone` is omitted. Initialization writes
the detected IANA zone when available. Changing it affects future interpretation
of timestamps without a zone; existing recorded operations do not change.

Saving validates the whole configuration and atomically replaces the private
file. Unknown keys and invalid values are rejected. An outdated settings page
cannot overwrite a newer configuration: reload and reapply the intended edits.
Simple comments and untouched lines are retained; complex TOML may be reformatted.
Stop old processes before upgrading. First open migrates legacy `client.json`,
`connection.json` and `sync-access.json`, then removes them only after the unified
configuration is durable. Unknown legacy versions are rejected. Back up the
configuration before upgrading; old binaries cannot read version 2.

Treat `kist.toml` as secret: it can contain a client's connection credential and
library credential digests. The API never returns stored secrets. New sync
credentials are shown once; reconnecting requires entering a credential again.
Operational state (pending operations, downloaded catalog, cache inventory and
pinned selections) remains separate from configuration.

Capacity fields accept values such as `32 GB`, `32G`, and `32 GiB`. GB uses powers of 1000; GiB uses powers of 1024. Configuration files still store integer bytes, and editable display values preserve exact limits. Allowed access URLs contain the scheme, host, and optional port used to open kist, without a path. Enter one per line; local-only access can leave this empty.

Map options have their own Maps section. Storage shows local file sizes for originals, archive information, and thumbnail/preview caches, with manual refresh. This includes historical versions and downloaded content, excludes projection copies and temporary data, and does not measure filesystem allocation after compression or block sharing.
