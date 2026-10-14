#!/usr/bin/env python3
"""Resolve and promote verified stable container channel."""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import re

spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("container-release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
output = release.output


def digest_for(reference):
    digest = output("docker", "buildx", "imagetools", "inspect", "--format", "{{.Manifest.Digest}}", reference)
    validate_digest(digest)
    return digest


def validate_digest(digest):
    if not re.fullmatch(r"sha256:[a-f0-9]{64}", digest):
        raise ValueError("Invalid registry digest")


def source(channel, repository):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Invalid repository")
    if channel != "latest":
        raise ValueError("Only the latest stable container channel is published")
    stable = json.loads(output("gh", "api", f"repos/{repository}/releases/latest"))
    return release.release_metadata(stable["tag_name"], repository)


def promote(channel, repository, version, revision, digest):
    validate_digest(digest)
    current = source(channel, repository)
    if (current["version"], current["revision"]) != (version, revision):
        print("Candidate is obsolete; channel unchanged")
        return
    image = current["image"]
    if digest_for(f"{image}:{version}") != digest:
        raise ValueError("Stable image changed after acceptance")
    target = f"{image}:{channel}"
    output("docker", "buildx", "imagetools", "create", "--tag", target, f"{image}@{digest}")
    if digest_for(target) != digest:
        raise ValueError("Channel does not reference the verified index")
    summary = f"Published `{target}` → `{image}@{digest}` ({version}, source `{revision}`).\n"
    print(summary)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as report:
            report.write(summary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare", "promote"])
    parser.add_argument("channel", choices=["latest"])
    parser.add_argument("repository")
    parser.add_argument("candidate", nargs="*")
    args = parser.parse_args()
    if args.command == "promote":
        if len(args.candidate) != 3:
            parser.error("promote requires version, revision and digest")
        promote(args.channel, args.repository, *args.candidate)
    else:
        metadata = source(args.channel, args.repository)
        if args.channel == "latest":
            metadata["digest"] = digest_for(f'{metadata["image"]}:{metadata["version"]}')
        for key, value in metadata.items():
            print(f"{key}={value}")


if __name__ == "__main__":
    main()
