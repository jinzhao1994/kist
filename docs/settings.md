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

GPS and AI settings also offer per-field resets, including checkboxes. Resetting
changes the draft until you save. GPS automatic lookup resets to on, pause on
failure to on, and the interval to 2 seconds; AI automatic extraction resets to
off. Explicit settings show a theme-colored **Explicitly set** marker, even when
they equal the current default. Inherited settings show a gray **Following defaults**
marker on hover or keyboard focus, without a reset action; it remains visible on touch devices. Explicit settings offer reset controls on hover or
keyboard focus; these remain visible on touch devices. Version 3
makes a missing GPS automatic flag follow the current default. Older configurations
are upgraded by pinning their previous effective flag, so upgrading never enables
a previously disabled lookup. Resetting only covers files added afterward, without
backfilling existing files.

Settings are grouped into Appearance, Storage, Service, Maps, Connection
(clients), and Access credentials (both libraries and clients). Storage includes client cache age and size
limits, or the library's capture-time fallback zone and projection mode. Service
includes listen address, allowed web origins and upload limits. The UI displays current defaults and marks options requiring a
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
and 1000 files. Client cache defaults are `cache_max_age = "180d"` and
`cache_max_bytes = 10737418240` under `[client]`. Cache limits do not evict pinned
content, pending changes or newly imported originals. Reducing limits applies to
subsequent cache maintenance. The client service prunes eligible content at
startup and every hour, including while offline; the prune CLI and API remain
available for immediate maintenance.

New libraries, omitted `library.timezone` values in new-format files, and restoring
the default use `Asia/Shanghai`. Initialization writes this fixed IANA name instead
of detecting the host timezone. Explicitly saved zones remain unchanged. Changing
it affects future interpretation of timestamps without a zone; existing recorded
operations do not change. Legacy version 1 migration retains its original host-zone
interpretation.

`library.language` controls generated image descriptions, independently of browser language. It defaults to `zh-CN` and accepts BCP 47 tags such as `en` or `zh-Hant`. Changes affect subsequent tasks only; completed descriptions retain their original language.

Saving validates the whole configuration and atomically replaces the private
file. Unknown keys and invalid values are rejected. An outdated settings page
cannot overwrite a newer configuration: reload and reapply the intended edits.
Simple comments and untouched lines are retained; complex TOML may be reformatted.
Stop old processes before upgrading. First open migrates legacy `client.json`,
`connection.json` and `sync-access.json`, then removes them only after the unified
configuration is durable. Unknown legacy versions are rejected. Back up the
configuration before upgrading; binaries supporting version 2 or earlier cannot read version 3. Restore the pre-upgrade backup before downgrading.

Treat `kist.toml` as secret: it can contain a client's connection credential and
library credential digests. The API never returns stored secrets. New sync
credentials are shown once; reconnecting requires entering a credential again.
Operational state (pending operations, downloaded catalog, cache inventory and
pinned selections) remains separate from configuration.

Capacity fields accept values such as `32 GB`, `32G`, and `32 GiB`. GB uses powers of 1000; GiB uses powers of 1024. Configuration files still store integer bytes, and editable display values preserve exact limits. Allowed access URLs contain the scheme, host, and optional port used to open kist, without a path. Enter one per line; local-only access can leave this empty.

Map options have their own Maps section. Storage shows local file sizes for originals, archive information, and thumbnail/preview caches, with manual refresh. This includes historical versions and downloaded content, excludes projection copies and temporary data, and does not measure filesystem allocation after compression or block sharing.

Access credentials support optional notes to distinguish clients or purposes. Editing or clearing a note does not rotate the token; existing credentials remain valid.

Saving applies configuration immediately, except for the listen address, which requires a restart. Map changes refresh the page to load the new resource policy. On Linux/macOS, use `kill -HUP <PID>` after editing the file. Invalid reloads preserve the active service configuration. On Windows, save through the UI or restart after editing the file. Existing upload sessions can finish; projection and time-zone changes do not batch-modify existing files or historical records.
