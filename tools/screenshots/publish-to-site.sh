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

GALLERY=(
  meeting-grid__dark__desktop.png
  meeting-grid__light__desktop.png
  meeting-grid__dark__mobile.png
  meeting-grid__light__mobile.png
  meeting-welcome__dark__desktop.png
  meeting-welcome__light__desktop.png
  meeting-welcome__dark__mobile.png
  meeting-welcome__light__mobile.png
  meeting-participants__dark__desktop.png
  meeting-participants__light__desktop.png
  meeting-participants__dark__mobile.png
  meeting-participants__light__mobile.png
  meeting-screenshare__dark__desktop.png
  meeting-screenshare__light__desktop.png
  meeting-info__dark__desktop.png
  meeting-info__light__desktop.png
  meeting-chat__dark__desktop.png
  meeting-chat__light__desktop.png
  meeting-chat__dark__mobile.png
  meeting-chat__light__mobile.png
  landing__dark__desktop.png
  landing__light__desktop.png
  auth-login__dark__desktop.png
  auth-login__light__desktop.png
  dashboard__dark__desktop.png
  dashboard__light__desktop.png
  admin-overview__dark__desktop_1.png
  admin-overview__light__desktop_1.png
  admin-settings-auth__dark__desktop_1.png
  admin-settings-auth__light__desktop_1.png
)

missing=0
for f in "${GALLERY[@]}"; do
  if [ -f "$OUT/$f" ]; then
    cp -f "$OUT/$f" "$DEST/gallery/$f"
  else
    echo "missing gallery shot: $f" >&2
    missing=1
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

echo "published screenshots → $DEST"
if [ "$missing" -ne 0 ]; then
  echo "some gallery files were missing" >&2
  exit 1
fi
