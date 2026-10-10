#!/usr/bin/env python3
"""Keep source-build and registry deployments identical except image selection."""

import json
import os
import subprocess

# Do not let a developer's local .env or exported KIST_* settings alter the check.
env = {key: value for key, value in os.environ.items() if not key.startswith(("KIST_", "COMPOSE_")) and key != "TZ"}


def configuration(path):
    config = json.loads(subprocess.check_output([
        "docker", "compose", "--env-file", os.devnull, "-f", path, "config", "--format", "json",
    ], env=env, text=True))
    service = config["services"]["kist"]
    build = service.pop("build", None)
    image = service.pop("image")
    return config, build, image


source, build, source_image = configuration("compose.yaml")
registry, registry_build, registry_image = configuration("compose.image.yaml")
assert build is not None and source_image == "kist:local"
assert registry_build is None and registry_image.startswith("ghcr.io/jinzhao1994/kist:")
assert source == registry, "Runtime configuration drift between source and registry deployments"
print("Source and registry Compose runtime settings match")
