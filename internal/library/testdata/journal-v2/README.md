# Frozen journal v2 request fixture

Captured with the M4 change introducing journal v2 on 2026-10-01. The two
synthetic events record a requested tag creation and a successful no-change
receipt. Both request keys, fingerprints and their op IDs are authoritative.

The expected snapshot includes the request index. Open, rebuild and deep
verification must preserve these results and the source log bytes. This fixture
contains no file content. Add fixtures for future formats instead of rewriting
this one; see the v1 fixture README for the compatibility policy.
