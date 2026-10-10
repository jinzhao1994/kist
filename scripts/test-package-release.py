#!/usr/bin/env python3
"""Verify that distributable archives retain project and dependency notices."""

import hashlib
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile


class PackageReleaseTest(unittest.TestCase):
    def test_notices_are_in_both_archive_formats_and_checksums(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scripts").mkdir()
            shutil.copy(Path(__file__).with_name("package-release.py"), root / "scripts/package-release.py")
            (root / "dist").mkdir()
            (root / "docs").mkdir()
            documents = ["LICENSE", "THIRD_PARTY_NOTICES.md", "SECURITY.md", "README.md", "README.zh-CN.md", "docs/operations.md"]
            for document in documents:
                (root / document).write_text(f"notice: {document}\n", encoding="utf-8")
            for name in ["kist_linux_amd64", "kist_windows_amd64.exe"]:
                (root / "dist" / name).write_bytes(b"synthetic executable")
            subprocess.run([sys.executable, str(root / "scripts/package-release.py")], check=True)
            with tarfile.open(root / "dist/kist_linux_amd64.tar.gz") as archive:
                self.assertEqual(set(archive.getnames()), {"kist", *documents})
                self.assertIn(b"THIRD_PARTY_NOTICES", archive.extractfile("THIRD_PARTY_NOTICES.md").read())
            with zipfile.ZipFile(root / "dist/kist_windows_amd64.zip") as archive:
                self.assertEqual(set(archive.namelist()), {"kist.exe", *documents})
                self.assertIn(b"THIRD_PARTY_NOTICES", archive.read("THIRD_PARTY_NOTICES.md"))
            for row in (root / "dist/SHA256SUMS").read_text().splitlines():
                digest, name = row.split("  ")
                self.assertEqual(digest, hashlib.sha256((root / "dist" / name).read_bytes()).hexdigest())


if __name__ == "__main__":
    unittest.main()
