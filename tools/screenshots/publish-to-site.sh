#!/usr/bin/env bash
# Copy generated Puppeteer PNGs into apps/site/public/preview/ for the landing + gallery.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/tools/screenshots/output"
DEST="$ROOT/apps/site/public/preview"

if [ ! -d "$OUT" ]; then
  echo "missing screenshot output: $OUT" >&2
  exit 1
fi

mkdir -p "$DEST/gallery" "$DEST/avatars"

# Copy every captured PNG. Slice names (_1, _2) are only written when the page
# is taller than the viewport; alias the first slice to the unsliced file too
# so the gallery can request either name.
shopt -s nullglob
copied=0
for png in "$OUT"/*.png; do
  base="$(basename "$png")"
  cp -f "$png" "$DEST/gallery/$base"
  copied=$((copied + 1))
done
if [ "$copied" -eq 0 ]; then
  echo "no PNG files in $OUT" >&2
  exit 1
fi

# Gallery historically asked for *_1.png on long pages. If capture wrote a
# single viewport shot, publish it under the _1 name as well.
for png in "$OUT"/*.png; do
  base="$(basename "$png")"
  case "$base" in
    *_*.png) ;;
    *) continue ;;
  esac
  case "$base" in
    *_1.png|*_2.png|*_3.png) continue ;;
  esac
  alias="${base%.png}_1.png"
  if [ ! -f "$OUT/$alias" ]; then
    cp -f "$png" "$DEST/gallery/$alias"
  fi
done

# The reverse alias. A page taller than the viewport is only ever written as
# name_1.png, name_2.png, …, and the gallery names several pages unsuffixed. A
# page that starts rendering its real content grows past the viewport and stops
# publishing under the name the site asks for.
for png in "$OUT"/*_1.png; do
  base="$(basename "$png")"
  unsliced="${base%_1.png}.png"
  if [ ! -f "$OUT/$unsliced" ]; then
    cp -f "$png" "$DEST/gallery/$unsliced"
  fi
done

cp -f "$OUT/meeting-chat__dark__mobile.png" "$DEST/meeting-phone.png"
cp -f "$OUT/meeting-chat__light__mobile.png" "$DEST/meeting-phone-light.png"
# MacBook lid is 4:3; prefer dark desktop grid (landing crops with object-cover).
cp -f "$OUT/meeting-grid__dark__desktop.png" "$DEST/meeting-desktop.png"

if [ -d "$ROOT/tools/screenshots/avatars" ]; then
  shopt -s nullglob
  for png in "$ROOT/tools/screenshots/avatars"/*.png; do
    cp -f "$png" "$DEST/avatars/"
  done
fi

echo "published $copied screenshots → $DEST"

REQUIRED=(
  meeting-grid__dark__desktop.png
  meeting-chat__dark__mobile.png
  meeting-chat__light__mobile.png
)
for f in "${REQUIRED[@]}"; do
  if [ ! -f "$DEST/gallery/$f" ]; then
    echo "missing required landing shot: $f" >&2
    exit 1
  fi
done

# Everything the landing gallery names has to be on disk, or the page ships
# broken images and nothing says so. Derived from the component rather than
# listed here, so adding a shot to the gallery cannot drift from this check.
GALLERY_SRC="$ROOT/apps/site/src/components/landing/screenshots-gallery.tsx"
missing=0
for f in $(grep -oE '/preview/gallery/[A-Za-z0-9_.-]+\.png' "$GALLERY_SRC" | sed 's|.*/||' | sort -u); do
  if [ ! -f "$DEST/gallery/$f" ]; then
    echo "gallery references a shot that was not captured: $f" >&2
    missing=1
  fi
done
if [ "$missing" -ne 0 ]; then
  exit 1
fi
