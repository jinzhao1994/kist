#!/usr/bin/env python3
"""Exercise a shipped binary, without Go, Node, or media tools installed."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import zipfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("platform")
    parser.add_argument("--version", default="dev")
    parser.add_argument("--commit", help="expected source commit")
    args = parser.parse_args()
    binary = args.binary.resolve()
    checksums = binary.parent / "SHA256SUMS"
    if binary.name.endswith((".tar.gz", ".zip")) and not checksums.exists():
        raise RuntimeError("Release archive is missing SHA256SUMS")
    if checksums.exists():
        entries = dict(line.split(maxsplit=1)[::-1] for line in checksums.read_text().splitlines())
        assert hashlib.sha256(binary.read_bytes()).hexdigest() == entries[binary.name]
    binary.chmod(binary.stat().st_mode | 0o111)

    # A separate cwd proves the UI is embedded rather than read from the checkout.
    with tempfile.TemporaryDirectory(prefix="kist-release-") as temporary:
        work = Path(temporary)
        if binary.name.endswith((".tar.gz", ".zip")):
            executable = "kist.exe" if args.platform.startswith("windows/") else "kist"
            expected = {executable, "LICENSE", "README.md", "README.zh-CN.md", "operations.md"}
            if binary.suffix == ".zip":
                with zipfile.ZipFile(binary) as archive:
                    assert set(archive.namelist()) == expected
                    content = archive.read(executable)
            else:
                with tarfile.open(binary) as archive:
                    assert set(archive.getnames()) == expected
                    assert all(member.isfile() for member in archive.getmembers())
                    content = archive.extractfile(executable).read()
            binary = work / executable
            binary.write_bytes(content)
            binary.chmod(0o755)
        library = work / "library"

        def cli(*command):
            result = subprocess.run(
                [str(binary), "--library", str(library), "--json", *command],
                cwd=work, capture_output=True, text=True, encoding="utf-8", timeout=60,
            )
            if result.returncode:
                raise RuntimeError(f"{command}: {result.stdout}\n{result.stderr}")
            return json.loads(result.stdout)

        version = cli("version")
        assert version["platform"] == args.platform, version
        assert version["version"] == args.version, version
        if args.commit:
            assert version["commit"] == args.commit[:12], version
            assert version["dirty"] is False, version
        print(json.dumps(version), flush=True)
        cli("init", str(library), "--yes")
        cli("box", "new", "Release smoke")
        original = b"kist release artifact smoke\n"
        source = work / "source.txt"
        source.write_bytes(original)
        cli("import", "--box", "Release smoke", str(source))
        files = cli("find")["files"]
        assert len(files) == 1, files
        file_id = files[0]["id"]
        imported = cli("log")["ops"][0]["id"]
        deleted = cli("undo", str(imported))["undone_by"]
        assert not cli("find")["files"]
        cli("undo", str(deleted))
        cli("rebuild", "--yes")
        cli("relink")
        cli("verify", "--deep")
        cli("scrub")
        assert cli("find")["files"][0]["id"] == file_id

        # Restore only authoritative data and settings, as the operations guide says.
        restored = work / "restored"
        restored.mkdir()
        shutil.copy2(library / "kist.toml", restored / "kist.toml")
        for directory in ("ops", "blobs"):
            shutil.copytree(library / directory, restored / directory)
        library = restored
        cli("rebuild", "--yes")
        cli("relink")
        cli("verify", "--deep")
        cli("scrub")
        assert cli("find")["files"][0]["id"] == file_id

        # serve requires a nonzero port. Reserve an available local port until launch.
        with socket.socket() as reservation:
            reservation.bind(("127.0.0.1", 0))
            port = reservation.getsockname()[1]
        base = f"http://127.0.0.1:{port}"
        # Do not inherit machine proxy settings for this local-only test.
        http = urllib.request.build_opener(urllib.request.ProxyHandler({}))

        def get(path, headers=None):
            request = urllib.request.Request(base + path, headers=headers or {})
            with http.open(request, timeout=5) as response:
                return response.status, response.read()

        with (work / "serve.log").open("w", encoding="utf-8") as log:
            server = subprocess.Popen(
                [str(binary), "--library", str(library), "serve", "--listen", f"127.0.0.1:{port}"],
                cwd=work, stdout=log, stderr=log,
            )
            try:
                for _ in range(100):
                    if server.poll() is not None:
                        raise RuntimeError("server exited before readiness")
                    try:
                        get("/api/v1/info")
                        break
                    except (urllib.error.URLError, TimeoutError):
                        time.sleep(0.1)
                else:
                    raise RuntimeError("server readiness timed out")
                html = get("/")[1].decode()
                assert "<title>kist</title>" in html
                assets = re.findall(r'(?:src|href)="([^" ]+\.(?:js|css))"', html)
                assert any(asset.endswith(".js") for asset in assets), html
                assert any(asset.endswith(".css") for asset in assets), html
                for asset in assets:
                    status, body = get(asset)
                    assert status == 200 and body and not body.startswith(b"<!doctype")
                assert get(f"/api/v1/files/{file_id}/content")[1] == original
                assert get(f"/api/v1/files/{file_id}/content?at={imported}")[1] == original
                status, body = get(f"/api/v1/files/{file_id}/content", {"Range": "bytes=0-3"})
                assert status == 206 and body == original[:4]
                try:
                    get("/api/v1/info", {"Origin": "https://untrusted.example"})
                    raise AssertionError("untrusted origin accepted")
                except urllib.error.HTTPError as error:
                    assert error.code == 403
            finally:
                server.terminate()
                try:
                    server.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    server.kill()
                    server.wait(timeout=5)
                print((work / "serve.log").read_text(encoding="utf-8"), flush=True)
    print("Release artifact smoke passed.", flush=True)


if __name__ == "__main__":
    main()
