#!/usr/bin/env python3
"""Copy verified GHCR indices to Docker Hub without rebuilding their contents."""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess

spec = importlib.util.spec_from_file_location("channel", Path(__file__).with_name("container-channel.py"))
channel = importlib.util.module_from_spec(spec)
spec.loader.exec_module(channel)
output = channel.output


def validate_target(target):
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*/[a-z0-9][a-z0-9_.-]*", target):
        raise ValueError("Expected Docker Hub namespace/repository")


def resolve(tag, repository):
    if tag in ("latest", "edge"):
        metadata = channel.source(tag, repository)
    else:
        metadata = channel.release.release_metadata(tag, repository)
    reference = f'{metadata["image"]}:{tag}'
    metadata["digest"] = channel.digest_for(reference)
    metadata["tag"] = tag
    if tag == "latest" and metadata["digest"] != channel.digest_for(f'{metadata["image"]}:{metadata["version"]}'):
        raise ValueError("GHCR latest has not promoted the current stable release")
    return metadata


def copy_index(source, destination):
    subprocess.run(["skopeo", "copy", "--all", "--preserve-digests",
                    f"docker://{source}", f"docker://{destination}"], check=True)


def existing_digest(reference):
    result = subprocess.run(["docker", "buildx", "imagetools", "inspect", "--format",
                             "{{.Manifest.Digest}}", reference], capture_output=True, text=True)
    if result.returncode == 0:
        digest = result.stdout.strip()
        channel.validate_digest(digest)
        return digest
    # Missing credentials, rate limits and network errors must fail closed.
    if re.search(r"manifest unknown|no such manifest", result.stderr, re.I):
        return None
    raise RuntimeError(f"Cannot inspect destination: {result.stderr}")


def promote(metadata, repository, target):
    validate_target(target)
    current = resolve(metadata["tag"], repository)
    if any(current[key] != metadata[key] for key in ("version", "revision", "digest")):
        print("Source moved during acceptance; destination unchanged")
        return
    destination = f'docker.io/{target}:{metadata["tag"]}'
    existing = existing_digest(destination)
    if existing == metadata["digest"]:
        print(f"Already synchronized: {destination}")
        return
    if metadata["tag"] not in ("latest", "edge") and existing is not None:
        raise ValueError("Refusing to replace an existing version tag with different content")
    copy_index(f'docker.io/{target}@{metadata["digest"]}', destination)
    if channel.digest_for(destination) != metadata["digest"]:
        raise ValueError("Destination digest differs from the accepted source")
    summary = f'Published `{destination}` at `{metadata["digest"]}` (source `{metadata["revision"]}`).\n'
    print(summary)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as report:
            report.write(summary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare", "promote"])
    parser.add_argument("repository")
    parser.add_argument("target")
    parser.add_argument("tag")
    parser.add_argument("--metadata", type=Path, default=Path("mirror.json"))
    args = parser.parse_args()
    validate_target(args.target)
    if args.command == "promote":
        metadata = json.loads(args.metadata.read_text())
        if metadata["tag"] != args.tag:
            raise ValueError("Acceptance tag mismatch")
        promote(metadata, args.repository, args.target)
        return
    metadata = resolve(args.tag, args.repository)
    candidate = f'docker.io/{args.target}:candidate-{os.environ["GITHUB_RUN_ID"]}-{os.environ["GITHUB_RUN_ATTEMPT"]}'
    copy_index(f'{metadata["image"]}@{metadata["digest"]}', candidate)
    if channel.digest_for(candidate) != metadata["digest"]:
        raise ValueError("Copied index changed digest")
    args.metadata.write_text(json.dumps(metadata))
    # Skopeo progress goes to stdout; only these fields belong in job outputs.
    with open(os.environ["GITHUB_OUTPUT"], "a") as outputs:
        for key in ("version", "revision", "digest", "tag"):
            outputs.write(f"{key}={metadata[key]}\n")


if __name__ == "__main__":
    main()
