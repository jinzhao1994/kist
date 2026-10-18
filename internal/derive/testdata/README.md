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

`cube.stl` is a generated unit cube with 12 triangles, authored for these tests.
It contains no external model data and is covered by the repository license.

`two-pages.docx` is a synthetic OOXML package with two paragraphs separated by an
explicit page break, including Chinese text. `two-pages.doc` is its LibreOffice
Word 97 export. Both were authored for this project and contain no personal data,
macros or external resources. They exercise actual conversion, pagination and
selectable text, including extensionless blob inputs.

`two-slides.pptx` and `two-slides.ppt` are LibreOffice Impress exports of a
project-authored two-slide OpenDocument presentation (16:9, English and Chinese
text). They contain no personal data, macros or external resources. The real
converter tests check both slides, searchable text, landscape thumbnail dimensions
and unchanged originals.
