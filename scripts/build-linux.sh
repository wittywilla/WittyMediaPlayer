#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 WittyWilla and contributors
# Builds WittyMediaPlayer for Linux into an AppImage: one file that runs on most desktops with
# nothing installed. Runs on Linux, x86_64 or aarch64, for that machine's processor.
#
#   bash scripts/setup-toolchain-linux.sh    (once)
#   bash scripts/build-linux.sh
#
# The result lands in .build/linux/ , which Git ignores:
#   WittyMediaPlayer-<version>-linux-<arch>.AppDir/      the folder the image is made from
#   WittyMediaPlayer-<version>-linux-<arch>.AppImage     the image, to share
#   source/                                             FFmpeg's source, to go beside it
#
# In a worktree, set WITTYVISUALIZER_DIR to a checkout of WittyVisualizer: the default is one
# beside the repository. APPIMAGETOOL=<program> uses your own appimagetool instead of the
# pinned download in packaging/linux/tools.lock.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
T="${WMP_TOOLCHAIN:-$HOME/.wittymediaplayer-toolchain-linux}"
QT_VERSION=6.10.3
FFMPEG_VERSION=8.1
APP_ID=com.wittywilla.WittyMediaPlayer
LOCK="$HERE/packaging/linux/tools.lock"

[ "$(uname -s)" = Linux ] || { echo "This builds the Linux AppImage: run it on Linux." >&2; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64; IMAGE_ARCH=x86_64; GOARCH=amd64; QT_DIR=gcc_64 ;;
  aarch64|arm64) ARCH=arm64; IMAGE_ARCH=aarch64; GOARCH=arm64; QT_DIR=gcc_arm64 ;;
  *) echo "No Linux build for $(uname -m) yet." >&2; exit 1 ;;
esac
QT="$T/qt/$QT_VERSION/$QT_DIR"
FF="$T/ffmpeg-min"
OUT="$HERE/.build/linux"
VERSION="$(sed -n 's/^project(WittyMediaPlayer VERSION \([0-9.]*\).*/\1/p' "$HERE/CMakeLists.txt")"
NAME="WittyMediaPlayer-$VERSION-linux-$ARCH"
APP="$OUT/$NAME.AppDir"

[ -d "$QT/lib" ] && [ -f "$FF/lib/libavcodec.so" ] || { echo "Run scripts/setup-toolchain-linux.sh first." >&2; exit 1; }
mkdir -p "$OUT/cmake/bin" "$OUT/tools"

# The helpers: no cgo, and no record of where they were built.
go_program() { # <source folder> <out>
  (cd "$HERE/engine/$1" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "$2" .)
}
go_program . "$OUT/cmake/bin/wittymediaplayer-engine"
go_program cmd/wittymediaplayer-updater "$OUT/cmake/bin/wittymediaplayer-updater"

# The window. Its own libraries are found beside it in the image, through AppRun.
WV_ARGS=()
if [ -n "${WITTYVISUALIZER_DIR:-}" ]; then WV_ARGS=(-DWITTYVISUALIZER_DIR="$WITTYVISUALIZER_DIR"); fi
cmake -S "$HERE" -B "$OUT/cmake" -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_PREFIX_PATH="$QT" -DFFMPEG_DIR="$FF" \
  -DCMAKE_EXE_LINKER_FLAGS="-s" -DCMAKE_BUILD_RPATH_USE_ORIGIN=ON "${WV_ARGS[@]}" >/dev/null
cmake --build "$OUT/cmake"

# The AppDir: nothing in it but what's needed to run and what its licences ask for.
rm -rf "$APP"
mkdir -p "$APP/usr/bin/bin" "$APP/usr/lib" "$APP/usr/plugins" "$APP/usr/share/doc/wittymediaplayer/licenses" \
  "$APP/usr/share/applications" "$APP/usr/share/metainfo" "$APP/usr/share/icons/hicolor/256x256/apps" "$APP/usr/share/icons/hicolor/scalable/apps"
cp "$OUT/cmake/WittyMediaPlayer" "$APP/usr/bin/"
cp "$OUT/cmake/bin/wittymediaplayer-engine" "$OUT/cmake/bin/wittymediaplayer-updater" "$APP/usr/bin/bin/"
install -m 0755 "$HERE/packaging/linux/AppRun" "$APP/AppRun"

# Qt: the libraries the window links to, found the way the loader finds them, from Qt's folder
# only. What every desktop has (the C library, OpenGL, X and Wayland's own, fonts) comes from
# the computer, as for every AppImage.
qt_libs() { ldd "$1" | awk '{print $3}' | grep "^$QT/lib/" || true; }
copy_deps() { # <elf>...
  local f lib
  for f in "$@"; do
    for lib in $(qt_libs "$f"); do
      [ -e "$APP/usr/lib/$(basename "$lib")" ] && continue
      cp -L "$lib" "$APP/usr/lib/"
      copy_deps "$lib"
    done
  done
}
copy_deps "$APP/usr/bin/WittyMediaPlayer"
# The plugins a desktop needs: X11 and Wayland windows, their OpenGL, Wayland's shells and
# decorations, and the picture formats Qt doesn't have built in that the app reads (JPEG covers).
for d in platforms xcbglintegrations wayland-decoration-client wayland-graphics-integration-client wayland-shell-integration; do
  [ -d "$QT/plugins/$d" ] && cp -R "$QT/plugins/$d" "$APP/usr/plugins/"
done
mkdir -p "$APP/usr/plugins/imageformats"
cp "$QT/plugins/imageformats/libqjpeg.so" "$APP/usr/plugins/imageformats/"
# Only the X11 and Wayland windows: no VNC, no framebuffer, no off-screen test platform.
find "$APP/usr/plugins/platforms" -type f ! -name 'libqxcb.so' ! -name 'libqwayland*.so' -delete
copy_deps $(find "$APP/usr/plugins" -name '*.so')
# Qt's own FFmpeg media backend isn't used, and it's a general build with network code: this
# player decodes through the libraries built for it. Qt6Network comes only with Qt's sound
# library, which won't load without it; nothing calls it.
rm -f "$APP"/usr/lib/libQt6Svg* "$APP"/usr/lib/libav* "$APP"/usr/lib/libsw*
for lib in libavcodec.so.62 libavformat.so.62 libavfilter.so.11 libavutil.so.60 libswresample.so.6 libswscale.so.9; do
  cp -L "$FF/lib/$lib" "$APP/usr/lib/"
done
printf '[Paths]\nPrefix = ..\nPlugins = plugins\n' > "$APP/usr/bin/qt.conf"

# The menu entry, its icon and the description software centres read, where AppImage tools look.
cp "$HERE/packaging/linux/$APP_ID.desktop" "$APP/$APP_ID.desktop"
cp "$HERE/packaging/linux/$APP_ID.desktop" "$APP/usr/share/applications/"
sed "s/@VERSION@/$VERSION/" "$HERE/packaging/linux/$APP_ID.metainfo.xml" > "$APP/usr/share/metainfo/$APP_ID.metainfo.xml"
cp "$HERE/resources/icon-256.png" "$APP/$APP_ID.png"
cp "$HERE/resources/icon-256.png" "$APP/usr/share/icons/hicolor/256x256/apps/$APP_ID.png"
cp "$HERE/resources/icon.svg" "$APP/usr/share/icons/hicolor/scalable/apps/$APP_ID.svg"
ln -s "$APP_ID.png" "$APP/.DirIcon"
cp "$HERE/README.md" "$HERE/LICENSE" "$HERE/NOTICE" "$APP/usr/share/doc/wittymediaplayer/"
cp "$HERE/licenses/"* "$APP/usr/share/doc/wittymediaplayer/licenses/"

# LGPL: FFmpeg's source travels beside the image.
mkdir -p "$OUT/source"
cp "$T/downloads/ffmpeg-$FFMPEG_VERSION.tar.xz" "$T/downloads"/lame-*.tar.gz "$OUT/source/"
{ cat "$HERE/scripts/ffmpeg-components.sh"; sed -n '/\.\/configure/,/--extra-ldflags/p' "$HERE/scripts/setup-toolchain-linux.sh"; } > "$OUT/source/how-ffmpeg-was-configured.txt"

bash "$HERE/scripts/check-package-linux.sh" "$APP"

# appimagetool and the runtime: exactly the files tools.lock names.
fetch_locked() { # <out> <name>
  local out="$1" name="$2" want url got=""
  read -r want url < <(awk -v n="$name" '$1 == n { print $2, $3 }' "$LOCK") || true
  [ -n "${want:-}" ] && [ -n "${url:-}" ] || { echo "$LOCK names no file for $name" >&2; return 1; }
  if [ -s "$out" ]; then got=$(sha256sum "$out" | cut -d' ' -f1); fi
  [ "$got" = "$want" ] && return 0
  curl -fsSL --retry 3 -o "$out.part" "$url" || { rm -f "$out.part"; echo "download failed: $url" >&2; return 1; }
  got=$(sha256sum "$out.part" | cut -d' ' -f1)
  [ "$got" = "$want" ] || { rm -f "$out.part"; echo "$url does not match $LOCK" >&2; return 1; }
  mv "$out.part" "$out"
}
TOOL="${APPIMAGETOOL:-}"
if [ -z "$TOOL" ]; then
  TOOL="$OUT/tools/appimagetool-$IMAGE_ARCH.AppImage"
  fetch_locked "$TOOL" "appimagetool-$IMAGE_ARCH"
  chmod +x "$TOOL"
fi
RUNTIME="$OUT/tools/runtime-$IMAGE_ARCH"
fetch_locked "$RUNTIME" "runtime-$IMAGE_ARCH"

TARGET="$OUT/$NAME.AppImage"
rm -f "$TARGET"
# Unpacking itself to run means the build machine needs no FUSE.
ARCH="$IMAGE_ARCH" VERSION="$VERSION" APPIMAGE_EXTRACT_AND_RUN=1 "$TOOL" --runtime-file "$RUNTIME" "$APP" "$TARGET"
chmod +x "$TARGET"
du -sh "$APP" "$TARGET"
echo "Built $TARGET"
