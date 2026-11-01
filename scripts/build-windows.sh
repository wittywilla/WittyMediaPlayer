#!/usr/bin/env bash
# Builds WittyMediaPlayer for Windows amd64 into a portable folder and a zip.
#
#   bash scripts/setup-toolchain.sh    (once)
#   bash scripts/build-windows.sh
#
# The result lands in .build/ , which Git ignores:
#   WittyMediaPlayer-<version>-windows-amd64/       run WittyMediaPlayer.exe from here
#   WittyMediaPlayer-<version>-windows-amd64.zip    the same folder, to share
#   source/                                         FFmpeg's source, to go beside the zip
#
# packaging/windows/build-msi.ps1 then makes the installer from that folder.
#
# In a worktree, set WITTYVISUALIZER_DIR to a checkout of WittyVisualizer: the default is one
# beside the repository.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
T="${WMP_TOOLCHAIN:-$HOME/.wittymediaplayer-toolchain}"
QT_VERSION=6.10.3
FFMPEG_VERSION=8.1
QT="$T/qt/$QT_VERSION/mingw_64"
FF="$T/ffmpeg-min"
OUT="$HERE/.build"
VERSION="$(sed -n 's/^project(WittyMediaPlayer VERSION \([0-9.]*\).*/\1/p' "$HERE/CMakeLists.txt")"
PKG="$OUT/WittyMediaPlayer-$VERSION-windows-amd64"

[ -x "$QT/bin/qsb.exe" ] && [ -f "$FF/bin/avcodec-62.dll" ] || { echo "Run scripts/setup-toolchain.sh first." >&2; exit 1; }
export PATH="$T/qt/Tools/mingw1310_64/bin:$T/qt/Tools/Ninja:$T/qt/Tools/CMake_64/bin:$QT/bin:$FF/bin:$PATH"

# Puts an icon, a description and a version on Windows programs, which is what Task Manager and a
# file's Properties show. A build tool only: nothing of it ships.
TOOLS="$OUT/tools"
mkdir -p "$OUT/cmake/bin" "$TOOLS"
if ! [ -x "$TOOLS/go-winres.exe" ]; then
  GOWORK=off GOBIN="$(cygpath -w "$(cd "$TOOLS" && pwd)")" go install github.com/tc-hib/go-winres@v0.3.3
fi
ICON="$(cygpath -w "$HERE/resources/WittyMediaPlayer.ico")"

# go_program <source folder> <out> <description> <original file name> <gui|cli>. No cgo, no
# console window, and no record of where it was built.
go_program() {
  local dir="$HERE/engine/$1" out="$2" desc="$3" name="$4" manifest="$5"
  (cd "$dir" && "$TOOLS/go-winres.exe" simply --arch amd64 --out rsrc --manifest "$manifest" --icon "$ICON" \
    --product-name WittyMediaPlayer --product-version "$VERSION" --file-version "$VERSION.0" \
    --file-description "$desc" --original-filename "$name" --copyright "Copyright WittyWilla and contributors" >/dev/null)
  (cd "$dir" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui -X main.Version=$VERSION" -o "$out" .) \
    || { rm -f "$dir"/rsrc_windows_*.syso; return 1; }
  rm -f "$dir"/rsrc_windows_*.syso
}
go_program . "$OUT/cmake/bin/wittymediaplayer-engine.exe" "WittyMediaPlayer: reads discs for the window" wittymediaplayer-engine.exe cli
go_program cmd/wittymediaplayer-updater "$OUT/cmake/bin/wittymediaplayer-updater.exe" "WittyMediaPlayer: downloads updates when you ask" wittymediaplayer-updater.exe cli
# It needs its manifest: Windows takes a program with "install" in its name for an installer, and
# asks for an administrator, unless told otherwise.
go_program cmd/wittymediaplayer-uninstall "$OUT/cmake/Uninstall WittyMediaPlayer.exe" "Uninstall WittyMediaPlayer" "Uninstall WittyMediaPlayer.exe" gui

# The window.
WV_ARGS=()
if [ -n "${WITTYVISUALIZER_DIR:-}" ]; then WV_ARGS=(-DWITTYVISUALIZER_DIR="$(cygpath -m "$WITTYVISUALIZER_DIR")"); fi
cmake -S "$HERE" -B "$OUT/cmake" -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_PREFIX_PATH="$(cygpath -m "$QT")" -DFFMPEG_DIR="$(cygpath -m "$FF")" \
  -DCMAKE_EXE_LINKER_FLAGS="-s" "${WV_ARGS[@]}" >/dev/null
cmake --build "$OUT/cmake"

# The package: nothing in it but what's needed to run and what its licences ask for.
rm -rf "$PKG"
mkdir -p "$PKG/licenses" "$PKG/bin"
# What a person opens stays at the top, with Qt's libraries, which have to be beside it. The helpers
# are in bin.
cp "$OUT/cmake/WittyMediaPlayer.exe" "$OUT/cmake/Uninstall WittyMediaPlayer.exe" "$PKG/"
cp "$OUT/cmake/bin/wittymediaplayer-engine.exe" "$OUT/cmake/bin/wittymediaplayer-updater.exe" "$PKG/bin/"
windeployqt6 --release --no-translations --no-system-d3d-compiler --no-opengl-sw --no-system-dxc-compiler --no-compiler-runtime \
  --skip-plugin-types generic,networkinformation,tls,qmltooling,iconengines --dir "$PKG" "$PKG/WittyMediaPlayer.exe" >"$OUT/windeployqt.log"
# The compiler's runtime libraries.
for dll in libgcc_s_seh-1.dll libstdc++-6.dll libwinpthread-1.dll; do cp "$T/qt/Tools/mingw1310_64/bin/$dll" "$PKG/"; done
# Qt's own FFmpeg media backend isn't used, and it's a general build with network code:
# this player decodes through the libraries built for it.
rm -rf "$PKG/multimedia" "$PKG/tls" "$PKG/networkinformation" "$PKG/generic"
rm -f "$PKG"/av*.dll "$PKG"/sw*.dll
for lib in avcodec-62 avformat-62 avfilter-11 avutil-60 swresample-6 swscale-9; do cp "$FF/bin/$lib.dll" "$PKG/"; done
rm -f "$PKG"/Qt6Svg.dll "$PKG"/dxcompiler.dll "$PKG"/dxil.dll
rm -f "$PKG"/imageformats/q{gif,icns,ico,pdf,svg,tga,tiff,wbmp,webp}*.dll

cp "$HERE/README.md" "$HERE/LICENSE" "$HERE/NOTICE" "$PKG/"
cp "$HERE/licenses/"* "$PKG/licenses/"

# LGPL: FFmpeg's source travels beside the binaries.
mkdir -p "$OUT/source"
cp "$T/downloads/ffmpeg-$FFMPEG_VERSION.tar.xz" "$OUT/source/"
cp "$T/downloads"/lame-*.tar.gz "$OUT/source/"
sed -n '/DEC="/,/--extra-ldflags/p' "$HERE/scripts/setup-toolchain.sh" > "$OUT/source/how-ffmpeg-was-configured.txt"

# The list of what's in the package is how the uninstaller knows the folder is ours, and what in it
# it may remove.
LIST="package-files.txt"
{
  echo "# WittyMediaPlayer package files"
  (cd "$PKG" && { find . -type f -not -name "$LIST" | sed 's|^\./||'; echo "$LIST"; } | LC_ALL=C sort)
} > "$OUT/$LIST"
mv "$OUT/$LIST" "$PKG/$LIST"

bash "$HERE/scripts/check-package.sh" "$PKG"

rm -f "$PKG.zip"
(cd "$OUT" && powershell.exe -NoProfile -Command "Compress-Archive -Path '$(basename "$PKG")' -DestinationPath '$(basename "$PKG").zip'")
du -sh "$PKG" "$PKG.zip"
echo "Built $PKG"
