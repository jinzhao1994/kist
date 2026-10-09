#!/bin/sh
# Exercise the actual runtime image using only disposable, named test resources.
set -eu
image=${1:-kist:local}
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
docker run --rm --read-only --tmpfs /tmp --mount "type=volume,src=$volume,dst=/library" --entrypoint sh "$image" -ec '
    test "$(id -u)" != 0
    ffmpeg -v error -f lavfi -i color=c=blue:s=240x160 -frames:v 1 -threads 1 /tmp/sample.png
    ffmpeg -v error -i /tmp/sample.png -frames:v 1 -threads 1 /tmp/thumb.webp
    ffmpeg -v error -f lavfi -i color=c=blue:s=240x160:r=10 -f lavfi -i sine=frequency=440 -t 1 -c:v libx264 -pix_fmt yuv420p -c:a aac -threads 1 /tmp/preview.mp4
    ffprobe -v error -show_entries stream=codec_name -of csv=p=0 /tmp/preview.mp4 | tee /tmp/codecs
    grep -qx h264 /tmp/codecs
    grep -qx aac /tmp/codecs
    test -s /tmp/thumb.webp
    kist --library /library import --box "Docker smoke" /tmp/sample.png
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
docker exec "$name" kist --library /library verify --deep
docker exec "$name" kist --library /library scrub
printf '%s\n' 'Docker smoke passed.'
