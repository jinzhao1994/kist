#!/bin/sh
# Exercise the actual runtime image using only disposable, named test resources.
set -eu
image=${1:-kist:local}
# CI passes an independent expectation, not one inferred from the tested image.
expected_platform=${2:-}
expected_version=${3:-}
if [ -n "$expected_platform" ]; then
    actual_platform=$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$image")
    test "$actual_platform" = "$expected_platform"
    docker run --rm "$image" version --json | python3 -c '
import json, sys
version = json.load(sys.stdin)
assert version["platform"] == sys.argv[1], version
if sys.argv[2]:
    assert version["version"] == sys.argv[2], version
print("Runtime platform verified:", version["platform"])
' "$expected_platform" "$expected_version"
fi
fixture_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../internal/derive/testdata" && pwd)
name="kist-smoke-$$"
volume="$name-library"
cleanup() {
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM
docker volume create "$volume" >/dev/null
docker run --rm -e TZ=Asia/Shanghai --mount "type=volume,src=$volume,dst=/library" "$image" init /library --yes
docker run --rm --mount "type=volume,src=$volume,dst=/library" "$image" --library /library box new 'Docker smoke'
# The image must have working encoders, not merely an ffmpeg executable.
docker run --rm --read-only --tmpfs /tmp --mount "type=volume,src=$volume,dst=/library" --mount "type=bind,src=$fixture_dir,dst=/fixtures,readonly" --entrypoint sh "$image" -ec '
    test "$(id -u)" != 0
    ffmpeg -v error -f lavfi -i color=c=blue:s=240x160 -frames:v 1 -threads 1 /tmp/sample.png
    ffmpeg -v error -i /tmp/sample.png -frames:v 1 -threads 1 /tmp/thumb.webp
    ffmpeg -v error -f lavfi -i color=c=blue:s=240x160:r=10 -f lavfi -i sine=frequency=440 -t 1 -c:v libx264 -pix_fmt yuv420p -c:a aac -threads 1 /tmp/preview.mp4
    ffprobe -v error -show_entries stream=codec_name -of csv=p=0 /tmp/preview.mp4 | tee /tmp/codecs
    grep -qx h264 /tmp/codecs
    grep -qx aac /tmp/codecs
    test -s /tmp/thumb.webp
    ffmpeg -v error -i /tmp/preview.mp4 -c copy /tmp/preview.mov
    pdftoppm -f 1 -l 1 -singlefile -scale-to 512 -jpeg /fixtures/two-pages.pdf /tmp/pdf-page
    test -s /tmp/pdf-page.jpg
    decoder=$(command -v heif-dec || command -v heif-convert)
    "$decoder" /fixtures/tiled.heic /tmp/heif.png
    ffprobe -v error -show_entries stream=width,height -of csv=p=0 /tmp/heif.png | grep -qx "3072,2048"
    kist --library /library import --box "Docker smoke" /tmp/sample.png /tmp/preview.mp4 /tmp/preview.mov /fixtures/tiled.heic /fixtures/two-pages.pdf /fixtures/cube.stl /fixtures/two-pages.doc /fixtures/two-pages.docx
    kist --library /library extract --quiet
'
start_server() {
    docker run -d --name "$name" --cap-drop ALL \
        --security-opt no-new-privileges:true \
        --mount "type=volume,src=$volume,dst=/library" \
        -p 127.0.0.1::8473 "$image" >/dev/null
}
start_server
port=$(docker port "$name" 8473/tcp | sed 's/.*://')
url="http://127.0.0.1:$port"
tries=0
until curl -fsS -H 'Host: localhost:8473' "$url/api/v1/info" >/dev/null; do
    tries=$((tries + 1))
    if [ "$tries" -ge 30 ]; then docker logs "$name"; exit 1; fi
    sleep 1
done
curl -fsS -H 'Host: localhost:8473' "$url/" | grep -q '<title>kist</title>'
curl -fsS -H 'Host: localhost:8473' "$url/api/v1/boxes" | grep -q 'Docker smoke'
code=$(curl -sS -o /dev/null -w '%{http_code}' -H 'Host: localhost:8473' -H 'Origin: https://untrusted.example' "$url/api/v1/info")
test "$code" = 403
python3 "$(dirname -- "$0")/media-smoke.py" "$url" localhost:8473
docker stop "$name" >/dev/null
docker rm "$name" >/dev/null
start_server
port=$(docker port "$name" 8473/tcp | sed 's/.*://')
url="http://127.0.0.1:$port"
tries=0
until curl -fsS -H 'Host: localhost:8473' "$url/api/v1/boxes" | grep -q 'Docker smoke'; do
    tries=$((tries + 1))
    if [ "$tries" -ge 30 ]; then docker logs "$name"; exit 1; fi
    sleep 1
done
curl -fsS -H 'Host: localhost:8473' "$url/api/v1/files" | grep -q sample.png
python3 "$(dirname -- "$0")/media-smoke.py" "$url" localhost:8473
# Inspect the actual cached PDFs, not a separate direct CLI conversion.
docker exec "$name" sh -ec '
    count=0
    for pdf in $(find /library/derived -name office_pdf_v1.pdf); do
        pdfinfo "$pdf" | grep -Eq "^Pages:[[:space:]]+2$"
        pdftotext "$pdf" - | grep -q "Kist Word second page"
        count=$((count + 1))
    done
    test "$count" -eq 2
'
docker exec "$name" kist --library /library verify --deep
docker exec "$name" kist --library /library scrub
printf '%s\n' 'Docker smoke passed.'
