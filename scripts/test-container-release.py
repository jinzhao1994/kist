#!/usr/bin/env python3
"""Regression checks for publication validation; never contacts a registry."""

import importlib.util
import sys
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location("container_release", __file__.replace("test-container-release.py", "container-release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseValidation(unittest.TestCase):
    def test_rejects_unpublished_or_moved_release(self):
        with patch.object(release, "output", return_value='{"tagName":"v0.1.0","isDraft":true,"isPrerelease":false}'):
            with self.assertRaises(ValueError):
                release.release_metadata("v0.1.0", "owner/repo")
        with patch.object(release, "output", side_effect=[
            '{"tagName":"v0.1.0","isDraft":false,"isPrerelease":false}', "a" * 40,
        ]):
            with self.assertRaises(ValueError):
                release.release_metadata("v0.1.0", "owner/repo", "b" * 40)

    def test_rejects_non_release_input_before_running_commands(self):
        with patch.object(release, "output") as command:
            for tag in ("main", "v0.1.0-rc.1", "v0.1.0\nother", "v１.0.0", "--help"):
                with self.assertRaises(ValueError):
                    release.release_metadata(tag, "owner/repo")
            command.assert_not_called()

    def test_resolves_published_source(self):
        with patch.object(release, "output", side_effect=[
            '{"tagName":"v0.1.0","isDraft":false,"isPrerelease":false}', "a" * 40,
        ]):
            result = release.release_metadata("v0.1.0", "Owner/Repo", "a" * 40)
        self.assertEqual(result, {"version": "v0.1.0", "revision": "a" * 40, "image": "ghcr.io/owner/repo"})

    def test_index_must_match_both_tested_digests(self):
        expected = {"linux/amd64": "sha256:" + "a" * 64, "linux/arm64": "sha256:" + "b" * 64}
        entries = [{"platform": {"os": "linux", "architecture": name.split("/")[1]}, "digest": digest} for name, digest in expected.items()]
        release.check_index({"manifests": entries}, expected)
        with self.assertRaises(ValueError):
            release.check_index({"manifests": entries[:1]}, expected)
        with self.assertRaises(ValueError):
            release.check_index({"manifests": entries + entries[:1]}, expected)
        altered = [dict(entry, digest="sha256:" + "c" * 64) for entry in entries]
        with self.assertRaises(ValueError):
            release.check_index({"manifests": altered}, expected)


if __name__ == "__main__":
    unittest.main()
