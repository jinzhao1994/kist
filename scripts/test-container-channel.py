#!/usr/bin/env python3
"""Regression tests for channel eligibility and stale publication guards."""

import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("channel", Path(__file__).with_name("container-channel.py"))
channel = importlib.util.module_from_spec(spec)
spec.loader.exec_module(channel)
SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64
REPO = "owner/kist"


class ChannelTests(unittest.TestCase):
    def test_development_channel_is_rejected_without_external_calls(self):
        with patch.object(channel, "output") as command:
            with self.assertRaises(ValueError):
                channel.source("edge", REPO)
        command.assert_not_called()

    def test_latest_uses_release_validation(self):
        with patch.object(channel, "output", return_value='{"tag_name":"v1.2.3"}'), patch.object(
                channel.release, "release_metadata", return_value={}) as validate:
            channel.source("latest", REPO)
        validate.assert_called_once_with("v1.2.3", REPO)

    def test_old_release_cannot_roll_back_latest(self):
        with patch.object(channel, "source", return_value={"version": "v2.0.0", "revision": SHA}), patch.object(channel, "output") as command:
            channel.promote("latest", REPO, "v1.0.0", SHA, DIGEST)
        command.assert_not_called()

    def test_changed_stable_digest_is_rejected(self):
        with patch.object(channel, "source", return_value={"version": "v1.0.0", "revision": SHA, "image": "ghcr.io/owner/kist"}), patch.object(
                channel, "digest_for", return_value="sha256:" + "c" * 64), patch.object(channel, "output") as command:
            with self.assertRaises(ValueError):
                channel.promote("latest", REPO, "v1.0.0", SHA, DIGEST)
        command.assert_not_called()

    def test_promotes_exact_verified_index(self):
        with patch.object(channel, "source", return_value={"version": "v1.0.0", "revision": SHA, "image": "ghcr.io/owner/kist"}), patch.object(
                channel, "digest_for", return_value=DIGEST), patch.object(channel, "output") as command:
            channel.promote("latest", REPO, "v1.0.0", SHA, DIGEST)
        command.assert_called_once_with("docker", "buildx", "imagetools", "create", "--tag", "ghcr.io/owner/kist:latest", "ghcr.io/owner/kist@" + DIGEST)

    def test_invalid_digest_never_reaches_external_tools(self):
        with patch.object(channel, "source") as source:
            with self.assertRaises(ValueError):
                channel.promote("latest", REPO, "v1.0.0", SHA, "bad")
        source.assert_not_called()


if __name__ == "__main__":
    unittest.main()
