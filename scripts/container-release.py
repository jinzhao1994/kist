#!/usr/bin/env python3
"""Validate release provenance and registry manifests for the container workflow."""

import argparse
import json
from pathlib import Path
import re
import subprocess


def output(*command):
    return subprocess.check_output(command, text=True).strip()


def release_metadata(tag, repository, expected_sha=""):
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("Only stable vMAJOR.MINOR.PATCH releases can publish images")
    release = json.loads(output(
        "gh", "release", "view", tag, "--repo", repository,
        "--json", "tagName,isDraft,isPrerelease",
    ))
    if release["tagName"] != tag or release["isDraft"] or release["isPrerelease"]:
        raise ValueError("A published stable GitHub Release is required")
    revision = output("git", "rev-parse", f"refs/tags/{tag}^{{commit}}")
    if expected_sha and revision != expected_sha:
        raise ValueError("The release tag moved after the triggering workflow ran")
    if not re.fullmatch(r"[a-f0-9]{40}", revision):
        raise ValueError("Invalid source revision")
    return {"version": tag, "revision": revision, "image": f"ghcr.io/{repository.lower()}"}


def check_index(index, expected):
    manifests = index["manifests"]
    actual = {}
    for manifest in manifests:
        platform = manifest["platform"]
        name = f'{platform["os"]}/{platform["architecture"]}'
        if name in actual:
            raise ValueError(f"Duplicate platform: {name}")
        actual[name] = manifest["digest"]
    if actual != expected:
        raise ValueError(f"Unexpected index: {actual}; expected {expected}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    prepare = commands.add_parser("prepare")
    prepare.add_argument("tag")
    prepare.add_argument("repository")
    prepare.add_argument("--expected-sha", default="")
    image = commands.add_parser("check-image")
    image.add_argument("image")
    image.add_argument("version")
    image.add_argument("revision")
    digest = commands.add_parser("digest")
    digest.add_argument("image")
    digest.add_argument("repository")
    index = commands.add_parser("check-index")
    index.add_argument("manifest", type=Path)
    index.add_argument("amd64")
    index.add_argument("arm64")
    args = parser.parse_args()
    if args.command == "prepare":
        for key, value in release_metadata(args.tag, args.repository, args.expected_sha).items():
            print(f"{key}={value}")
    elif args.command == "check-image":
        labels = json.loads(output("docker", "image", "inspect", "--format", "{{json .Config.Labels}}", args.image))
        expected = {"org.opencontainers.image.version": args.version, "org.opencontainers.image.revision": args.revision}
        if any(labels.get(key) != value for key, value in expected.items()):
            raise ValueError(f"Image provenance labels do not match {expected}")
    elif args.command == "digest":
        digests = json.loads(output("docker", "image", "inspect", "--format", "{{json .RepoDigests}}", args.image))
        matches = {entry.split("@", 1)[1] for entry in digests if entry.startswith(args.repository + "@")}
        if len(matches) != 1 or not re.fullmatch(r"sha256:[a-f0-9]{64}", next(iter(matches), "")):
            raise ValueError("Expected exactly one pushed image digest")
        print(matches.pop())
    elif args.command == "check-index":
        check_index(json.loads(args.manifest.read_text()), {"linux/amd64": args.amd64, "linux/arm64": args.arm64})
        print("Published index contains exactly the two tested platform digests")


if __name__ == "__main__":
    main()
