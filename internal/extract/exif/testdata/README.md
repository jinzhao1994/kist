# Fixtures

`synthetic.heic` — 755 bytes, a 64×64 gradient with a hand-built EXIF block.
Nobody's photograph. It carries `Make=kist`, `Model=synthetic fixture`, a
capture time with a zone offset, an ISO, a rational aperture and one GPS
coordinate.

**Why it is generated rather than downloaded or contributed.**

A real photograph is the obvious fixture and it is the wrong one, twice over.
Its EXIF carries the photographer's GPS coordinates, their device serial and
the exact moment they were somewhere — and a public repository is permanent.
And the convenient candidates carry licenses of their own: the one iPhone HEIC
within reach is CC BY-SA 4.0, which does not belong in an Apache-2.0 tree.

The sample-file sites say "free for testing" without naming a license, which
is not something to build a repository on.

**How to regenerate it** (needs ImageMagick and libheif; both are development
tools, not build dependencies):

    magick -size 64x64 gradient:steelblue-white -depth 8 -quality 80 JPEG:e.jpg
    KIST_FIXTURE_DIR=. go test ./internal/extract/exif/ -run GenerateHEICFixture
    heif-enc -q 60 withexif.jpg -o synthetic.heic

`GenerateHEICFixture` lives in `genfixture_test.go` and splices the EXIF block
that `fixture_test.go` builds into a JPEG that actually has image data, which
is what libheif needs in order to carry the metadata across.

**What this fixture is for, and what it is not.** It exists to hold down one
claim: EXIF is readable out of a HEIC container in pure Go, so an iPhone photo
gets its capture time in M3 without ffmpeg (spec 5.3). Everything about tag
mapping and typing is tested against JPEGs built in `fixture_test.go`, where a
case can say exactly which tag it is about — that is the better place for it,
and no binary is needed there at all.
