#!/usr/bin/env python3
"""Package make dist binaries with documentation and SHA-256 checksums."""

import hashlib
from pathlib import Path
import re
import tarfile
import zipfile


def main():
    root = Path(__file__).resolve().parent.parent
    dist = root / "dist"
    binaries = sorted(p for p in dist.iterdir() if re.fullmatch(r"kist_(darwin|linux|windows)_[a-z0-9]+(?:\.exe)?", p.name))
    if not binaries:
        raise SystemExit("No release binaries in dist; run make dist first")
    documents = [root / name for name in ("LICENSE", "THIRD_PARTY_NOTICES.md", "SECURITY.md", "README.md", "README.zh-CN.md", "docs/operations.md", "docs/offline-client.md", "docs/offline-client.zh-CN.md", "docs/settings.md", "docs/settings.zh-CN.md")]
    artifacts = []
    for binary in binaries:
        windows = binary.suffix == ".exe"
        name = "kist.exe" if windows else "kist"
        members = [(binary, name)] + [(path, path.relative_to(root).as_posix()) for path in documents]
        archive = dist / (binary.stem + (".zip" if windows else ".tar.gz"))
        if windows:
            with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as output:
                for path, member in members:
                    output.write(path, member)
        else:
            with tarfile.open(archive, "w:gz") as output:
                for path, member in members:
                    info = output.gettarinfo(str(path), arcname=member)
                    info.uid = info.gid = 0
                    info.uname = info.gname = ""
                    info.mode = 0o755 if path == binary else 0o644
                    with path.open("rb") as content:
                        output.addfile(info, content)
        artifacts.append(archive)
    # Raw binaries remain available locally; only archives are distributed.
    (dist / "SHA256SUMS").write_text("".join(
        f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n"
        for path in artifacts
    ), encoding="utf-8")


if __name__ == "__main__":
    main()
