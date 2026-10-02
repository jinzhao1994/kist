# Frozen journal v1 compatibility fixture

Captured on 2026-10-01 from the synthetic `busyLibrary` scenario with the
writer at commit `ddb89f969aa0b091c2b225a0904866399a22aa6f`. No personal files or
configuration are included. The pending verification-lock change does not
change any event or view in this fixture.

The 28 events cover all 15 actions implemented at capture time, including
imports with deduplicated blobs, tag hierarchy changes, box updates, undo,
extraction upgrades and rollback, user metadata tombstones, and timestamps.
`snapshot.txt` records the original writer's materialized state, including
operation indexes. The five tiny blobs contain only synthetic test strings.

Tests must read these committed bytes, not generate them with current writers.
Keep the log and blobs unchanged. Add a separate fixture for new formats or
semantics. An intentional view-schema change may require an explicitly reviewed
expected-snapshot migration; it must not alter the historical meaning of an
event or rewrite the input logs. Keep the previous expectation in Git history
and explain the migration in the design document.

This is the first compatibility baseline, not proof of compatibility with every
unreleased development revision or every possible payload.
