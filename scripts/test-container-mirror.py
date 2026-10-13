#!/usr/bin/env python3
"""Guard mirror immutability, stale channels and registry failures."""

import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("mirror", Path(__file__).with_name("container-mirror.py"))
mirror = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mirror)
DIGEST = "sha256:" + "a" * 64
OTHER = "sha256:" + "b" * 64
META = {"tag": "latest", "version": "v0.1.2", "revision": "c" * 40, "digest": DIGEST, "image": "ghcr.io/owner/kist"}


class MirrorTests(unittest.TestCase):
    def test_target_rejects_other_registries_and_injection(self):
        mirror.validate_target("jinzhao1994/kist")
        for value in ("ghcr.io/user/kist", "user/kist:latest", "user/kist\nother=x", "--evil"):
            with self.assertRaises(ValueError):
                mirror.validate_target(value)

    def test_stale_channel_does_not_publish(self):
        with patch.object(mirror, "resolve", return_value={**META, "digest": OTHER}), patch.object(mirror, "copy_index") as copy:
            mirror.promote(META, "owner/kist", "owner/kist")
            copy.assert_not_called()

    def test_existing_version_cannot_be_replaced(self):
        with patch.object(mirror, "existing_digest", return_value=OTHER), patch.object(mirror, "copy_index") as copy:
            with self.assertRaises(ValueError):
                mirror.publish(META, "owner/kist")
            copy.assert_not_called()

    def test_identical_version_is_retryable(self):
        with patch.object(mirror, "existing_digest", return_value=DIGEST), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=DIGEST):
            mirror.publish(META, "owner/kist")
            copy.assert_not_called()

    def test_latest_publishes_version_before_acceptance(self):
        with patch.object(mirror, "existing_digest", return_value=None), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=DIGEST):
            mirror.publish(META, "owner/kist")
            copy.assert_called_once_with(f"ghcr.io/owner/kist@{DIGEST}", "docker.io/owner/kist:v0.1.2")

    def test_edge_does_not_create_temporary_tags(self):
        metadata = {**META, "tag": "edge", "version": "edge-" + "c" * 40}
        with patch.object(mirror, "existing_digest", return_value=OTHER), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=DIGEST):
            mirror.publish(metadata, "owner/kist")
            copy.assert_called_once_with(f"ghcr.io/owner/kist@{DIGEST}", "docker.io/owner/kist:edge")

    def test_version_and_edge_acceptance_do_not_write_again(self):
        for tag in ("v0.1.2", "edge"):
            metadata = {**META, "tag": tag}
            with patch.object(mirror, "resolve", return_value=metadata), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=DIGEST):
                mirror.promote(metadata, "owner/kist", "owner/kist")
                copy.assert_not_called()

    def test_changed_version_cannot_promote_latest(self):
        with patch.object(mirror, "resolve", return_value=META), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=OTHER):
            with self.assertRaises(ValueError):
                mirror.promote(META, "owner/kist", "owner/kist")
            copy.assert_not_called()

    def test_promotion_uses_accepted_digest(self):
        with patch.object(mirror, "resolve", return_value=META), patch.object(mirror, "existing_digest", return_value=OTHER), patch.object(mirror, "copy_index") as copy, patch.object(mirror.channel, "digest_for", return_value=DIGEST):
            mirror.promote(META, "owner/kist", "owner/kist")
            copy.assert_called_once_with(f"docker.io/owner/kist@{DIGEST}", "docker.io/owner/kist:latest")

    def test_registry_errors_do_not_mean_missing(self):
        for error in ("unauthorized", "429 Too Many Requests", "connection refused", "proxy not found", "ERROR: other/image:tag: not found"):
            with patch.object(mirror.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "", error)):
                with self.assertRaises(RuntimeError):
                    mirror.existing_digest("docker.io/owner/kist:v0.1.2")
        with patch.object(mirror.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "", "manifest unknown")):
            self.assertIsNone(mirror.existing_digest("docker.io/owner/kist:v0.1.2"))

    def test_buildx_missing_tag_response(self):
        reference = "docker.io/owner/kist:v0.1.2"
        with patch.object(mirror.subprocess, "run", return_value=subprocess.CompletedProcess(
                [], 1, "", f"ERROR: {reference}: not found\n")):
            self.assertIsNone(mirror.existing_digest(reference))

    def test_latest_requires_promoted_stable(self):
        with patch.object(mirror.channel, "source", return_value={"image": "ghcr.io/owner/kist", "version": "v0.1.2"}), patch.object(mirror.channel, "digest_for", side_effect=[OTHER, DIGEST]):
            with self.assertRaises(ValueError):
                mirror.resolve("latest", "owner/kist")

    def test_copy_preserves_all_platforms_and_digests(self):
        with patch.object(mirror.subprocess, "run") as run:
            mirror.copy_index(f"ghcr.io/owner/kist@{DIGEST}", "docker.io/owner/kist:v0.1.2")
            run.assert_called_once_with(["skopeo", "copy", "--all", "--preserve-digests", f"docker://ghcr.io/owner/kist@{DIGEST}", "docker://docker.io/owner/kist:v0.1.2"], check=True)


if __name__ == "__main__":
    unittest.main()
