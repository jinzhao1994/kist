#!/usr/bin/env python3
"""Regression tests for channel eligibility and stale publication guards."""

import importlib.util
import json
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
    def run_record(self, **changes):
        return dict(head_sha=SHA, head_branch="main", event="push", conclusion="success",
                    head_repository={"full_name": REPO}) | changes

    def test_edge_requires_successful_same_repository_main_push(self):
        for changes in [{"head_sha": "c" * 40}, {"head_branch": "feature"},
                        {"event": "pull_request"}, {"conclusion": "failure"},
                        {"head_repository": {"full_name": "fork/kist"}}]:
            with self.subTest(changes=changes), patch.object(channel, "output", side_effect=[
                    SHA, json.dumps({"workflow_runs": [self.run_record(**changes)]})]):
                with self.assertRaises(ValueError):
                    channel.source("edge", REPO)

    def test_edge_resolves_verified_revision(self):
        with patch.object(channel, "output", side_effect=[SHA, json.dumps({"workflow_runs": [self.run_record()]})]):
            result = channel.source("edge", REPO)
        self.assertEqual(result["revision"], SHA)
        self.assertEqual(result["version"], "edge-" + SHA)

    def test_stale_edge_never_queries_ci_or_publishes(self):
        with patch.object(channel, "output", return_value="c" * 40) as command:
            channel.promote("edge", REPO, "edge-" + SHA, SHA, DIGEST)
        self.assertEqual(command.call_count, 1)

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
        with patch.object(channel, "source", return_value={"version": "edge-" + SHA, "revision": SHA, "image": "ghcr.io/owner/kist"}), patch.object(
                channel, "digest_for", return_value=DIGEST), patch.object(channel, "output") as command:
            channel.promote("edge", REPO, "edge-" + SHA, SHA, DIGEST)
        command.assert_called_once_with("docker", "buildx", "imagetools", "create", "--tag", "ghcr.io/owner/kist:edge", "ghcr.io/owner/kist@" + DIGEST)

    def test_invalid_digest_never_reaches_external_tools(self):
        with patch.object(channel, "source") as source:
            with self.assertRaises(ValueError):
                channel.promote("edge", REPO, "edge", SHA, "bad")
        source.assert_not_called()


if __name__ == "__main__":
    unittest.main()
