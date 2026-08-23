#!/bin/sh
# Build a standalone x86_64 AppImage (GTK 4 + libadwaita + embedded bedrud-host).
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ARCH=${ARCH:-x86_64}
DIST="$ROOT/dist"
APPDIR="$DIST/AppDir"
TOOLS="$ROOT/.appimage-tools"
OUT="$DIST/Bedrud_Host-${ARCH}.AppImage"

BIN="$ROOT/bedrud-gui"
if [ ! -x "$BIN" ]; then
	echo "missing $BIN — run make build first" >&2
	exit 1
fi

mkdir -p "$TOOLS" "$DIST"
export APPIMAGE_EXTRACT_AND_RUN=1

fetch() {
	url=$1
	dest=$2
	if [ -x "$dest" ]; then
		return 0
	fi
	echo "==> download $(basename "$dest")"
	curl -fsSL -o "$dest" "$url"
	chmod +x "$dest"
}

fetch "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${ARCH}.AppImage" \
	"$TOOLS/linuxdeploy-${ARCH}.AppImage"
fetch "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh" \
	"$TOOLS/linuxdeploy-plugin-gtk.sh"
fetch "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage" \
	"$TOOLS/appimagetool-${ARCH}.AppImage"

rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/scalable/apps"

cp -f "$BIN" "$APPDIR/usr/bin/bedrud-gui"
cp -f "$ROOT/packaging/org.bedrud.HostGui.desktop" "$APPDIR/usr/share/applications/"
cp -f "$ROOT/packaging/org.bedrud.HostGui.svg" "$APPDIR/usr/share/icons/hicolor/scalable/apps/"
cp -f "$ROOT/packaging/org.bedrud.HostGui.svg" "$APPDIR/org.bedrud.HostGui.svg"
cp -f "$ROOT/packaging/org.bedrud.HostGui.desktop" "$APPDIR/"

# Adwaita symbolic icons the UI names (standalone, no host theme required).
ICON_DST="$APPDIR/usr/share/icons/hicolor/scalable/actions"
mkdir -p "$ICON_DST" "$APPDIR/usr/share/icons/hicolor/scalable/places" \
	"$APPDIR/usr/share/icons/hicolor/scalable/status" \
	"$APPDIR/usr/share/icons/hicolor/scalable/categories"
copy_icon() {
	name=$1
	dest_dir=$2
	for base in /usr/share/icons/Adwaita/symbolic /usr/share/icons/Adwaita; do
		f=$(find "$base" -name "${name}.svg" 2>/dev/null | head -n1 || true)
		if [ -n "${f:-}" ]; then
			cp -f "$f" "$dest_dir/${name}.svg"
			return 0
		fi
	done
	echo "note: icon $name not found on this system" >&2
}
copy_icon list-add-symbolic "$ICON_DST"
copy_icon view-refresh-symbolic "$ICON_DST"
copy_icon go-previous-symbolic "$ICON_DST"
copy_icon edit-copy-symbolic "$ICON_DST"
copy_icon folder-open-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/places"
copy_icon preferences-system-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/categories"
copy_icon network-server-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/status"
copy_icon dialog-error-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/status"
copy_icon system-run-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/categories"
copy_icon emblem-ok-symbolic "$APPDIR/usr/share/icons/hicolor/scalable/status"

export PATH="$TOOLS:$PATH"
export LINUXDEPLOY="$TOOLS/linuxdeploy-${ARCH}.AppImage"
export DEPLOY_GTK_VERSION=4
export OUTPUT="$OUT"
export VERSION=${VERSION:-0.1.0}

echo "==> linuxdeploy + gtk plugin"
"$TOOLS/linuxdeploy-${ARCH}.AppImage" \
	--appdir "$APPDIR" \
	--executable "$APPDIR/usr/bin/bedrud-gui" \
	--desktop-file "$APPDIR/usr/share/applications/org.bedrud.HostGui.desktop" \
	--icon-file "$APPDIR/usr/share/icons/hicolor/scalable/apps/org.bedrud.HostGui.svg" \
	--plugin gtk

# AppRun helper: prefer bundled GSettings schemas / icons
if [ ! -f "$APPDIR/AppRun" ]; then
	printf '%s\n' '#!/bin/sh' 'HERE=$(dirname "$(readlink -f "$0")")' 'exec "$HERE/usr/bin/bedrud-gui" "$@"' >"$APPDIR/AppRun"
	chmod +x "$APPDIR/AppRun"
fi

echo "==> appimagetool"
"$TOOLS/appimagetool-${ARCH}.AppImage" "$APPDIR" "$OUT"
chmod +x "$OUT"
echo "==> $OUT"
ls -lh "$OUT"
