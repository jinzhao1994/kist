# Security policy

## Supported versions

Security fixes target the latest stable release. Upgrade to the latest patch
release before reporting an issue; older releases may not receive backports.

## Report a vulnerability privately

Use [GitHub private vulnerability reporting](https://github.com/jinzhao1994/kist/security/advisories/new).
Do not open a public issue or pull request containing exploit details, credentials,
archive files, or personal metadata. If private reporting is unavailable, contact
`jinzhao1994@126.com` with a minimal description and arrange a private exchange.

Include the kist version, deployment method, affected endpoint or command,
reproduction steps using synthetic data, and expected impact. Never send your
real library, tokens, or unredacted operation logs. We will investigate and
coordinate disclosure; no fixed response deadline is promised.

## Deployment boundary

kist currently has no user authentication. Its intended boundary is a single
user on a local machine or a trusted network. Remote access must be protected
by an authenticated reverse proxy or an equivalent access-control layer.
Host and Origin checks do not replace authentication. Do not expose kist directly
to the public internet. See [deployment guidance](docs/docker.md).

## Contributing safely

Keep local settings in ignored `.env` files and use synthetic test fixtures.
Run the secret scan before publishing changes. A leaked credential must be
revoked even if a later commit removes it; deleting a file does not erase history.
