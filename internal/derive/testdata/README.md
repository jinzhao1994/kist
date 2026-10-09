# Synthetic media

`pattern.heic` is a 128×96 generated color test pattern, with real HEVC pixel
content. It was created for this project, not taken from a personal photo:

```sh
ffmpeg -f lavfi -i testsrc=size=128x96 -frames:v 1 -threads 1 source.png
sips -s format heic source.png --out pattern.heic
```

It exercises an actual HEIC decode (including the display dimensions) rather
than only metadata parsing. `tiled.heic` uses the same procedure with `testsrc=size=3072x2048`; it
contains 24 tiles and must decode as one full image, never as one 512×512 tile.
These samples are not a claim to support every HEIC variant.
