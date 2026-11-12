#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 WittyWilla and contributors
# Gets everything WittyMediaPlayer's Linux build needs, for this user only: no root, nothing
# installed system-wide. Runs on Linux, x86_64 or aarch64.
#
#   bash scripts/setup-toolchain-linux.sh
#
# Needs a C and C++ compiler (gcc, g++), make, CMake, Ninja, NASM (x86_64 only), Python 3,
# curl, xz and Go, from the distribution: on Debian or Ubuntu,
#   sudo apt install build-essential cmake ninja-build nasm python3-venv curl xz-utils golang
# and the libraries Qt's window needs on the build machine,
#   sudo apt install libgl-dev libxkbcommon-dev libxcb-cursor0 libfontconfig1-dev libpulse-dev
# Everything else lands in WMP_TOOLCHAIN (default ~/.wittymediaplayer-toolchain-linux); delete
# that folder to remove it all. The same FFmpeg as the Windows build: the component lists are
# scripts/ffmpeg-components.sh, with no network code and no disc-decryption library.
set -euo pipefail

QT_VERSION=6.10.3
FFMPEG_VERSION=8.1
FFMPEG_SHA256=b072aed6871998cce9b36e7774033105ca29e33632be5b6347f3206898e0756a
ZLIB_VERSION=1.3.1
ZLIB_SHA256=9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23
OGG_VERSION=1.3.5
OGG_SHA256=0eb4b4b9420a0f51db142ba3f9c64b333f826532dc0f48c6410ae51f4799b664
VORBIS_VERSION=1.3.7
VORBIS_SHA256=0e982409a9c3fc82ee06e08205b1355e5c6aa4c36bca58146ef399621b0ce5ab
OPUS_VERSION=1.5.2
OPUS_SHA256=65c1d2f78b9f2fb20082c38cbe47c951ad5839345876e46941612ee87f9a7ce1
LAME_VERSION=3.100
LAME_SHA256=ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e

[ "$(uname -s)" = Linux ] || { echo "This is the Linux toolchain: run it on Linux." >&2; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=x86_64; QT_ARCH=linux_gcc_64; QT_DIR=gcc_64 ;;
  aarch64|arm64) ARCH=aarch64; QT_ARCH=linux_gcc_arm64; QT_DIR=gcc_arm64 ;;
  *) echo "No Linux build for $(uname -m) yet: x86_64 and aarch64 only." >&2; exit 1 ;;
esac
for tool in gcc g++ make cmake ninja python3 curl xz go; do
  command -v "$tool" >/dev/null || { echo "$tool is missing: see the top of this script for what to install." >&2; exit 1; }
done
[ "$ARCH" != x86_64 ] || command -v nasm >/dev/null || { echo "nasm is missing: FFmpeg's fast paths need it." >&2; exit 1; }

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
T="${WMP_TOOLCHAIN:-$HOME/.wittymediaplayer-toolchain-linux}"
mkdir -p "$T/downloads" "$T/src"
JOBS="$(nproc)"

fetch() { # <url> <file> <sha256>
  local out="$T/downloads/$2"
  [ -s "$out" ] || curl -fsSL --retry 3 -o "$out" "$1"
  echo "$3  $out" | sha256sum -c --quiet - || { echo "$2 is not the file this script expects; stopping." >&2; exit 1; }
}

# Qt from Qt's own servers, the same version as the Windows build.
QT="$T/qt/$QT_VERSION/$QT_DIR"
if [ ! -x "$QT/bin/qsb" ] && [ ! -x "$QT/libexec/qsb" ]; then
  [ -x "$T/aqt-venv/bin/aqt" ] || { python3 -m venv "$T/aqt-venv"; "$T/aqt-venv/bin/pip" install -q --disable-pip-version-check aqtinstall; }
  "$T/aqt-venv/bin/aqt" install-qt linux desktop "$QT_VERSION" "$QT_ARCH" -O "$T/qt" -m qtmultimedia qtshadertools
fi

# zlib and what copying CDs encodes with (LAME is LGPL; Opus, Vorbis and Ogg are BSD), as
# static libraries inside FFmpeg's own, which stay replaceable as a whole.
P="$T/ffmpeg-min"
mkdir -p "$P/include" "$P/lib"
untar() { rm -rf "$T/src/$1"; tar -xf "$T/downloads/$2" -C "$T/src"; }
if [ ! -f "$P/lib/libz.a" ]; then
  fetch "https://github.com/madler/zlib/releases/download/v$ZLIB_VERSION/zlib-$ZLIB_VERSION.tar.gz" "zlib-$ZLIB_VERSION.tar.gz" "$ZLIB_SHA256"
  untar "zlib-$ZLIB_VERSION" "zlib-$ZLIB_VERSION.tar.gz"
  (cd "$T/src/zlib-$ZLIB_VERSION" && CFLAGS="-O2 -fPIC" ./configure --static --prefix="$P" >/dev/null && make -s -j"$JOBS" install >/dev/null)
fi
if [ ! -f "$P/lib/libmp3lame.a" ] || [ ! -f "$P/lib/libopus.a" ] || [ ! -f "$P/lib/libvorbisenc.a" ]; then
  fetch "https://downloads.xiph.org/releases/ogg/libogg-$OGG_VERSION.tar.gz" "libogg-$OGG_VERSION.tar.gz" "$OGG_SHA256"
  fetch "https://downloads.xiph.org/releases/vorbis/libvorbis-$VORBIS_VERSION.tar.gz" "libvorbis-$VORBIS_VERSION.tar.gz" "$VORBIS_SHA256"
  fetch "https://github.com/xiph/opus/releases/download/v$OPUS_VERSION/opus-$OPUS_VERSION.tar.gz" "opus-$OPUS_VERSION.tar.gz" "$OPUS_SHA256"
  fetch "https://downloads.sourceforge.net/project/lame/lame/$LAME_VERSION/lame-$LAME_VERSION.tar.gz" "lame-$LAME_VERSION.tar.gz" "$LAME_SHA256"
  build_cmake() { # <folder> <extra args>
    local src="$T/src/$1"; shift
    cmake -S "$src" -B "$src/b" -G Ninja -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
      -DCMAKE_INSTALL_PREFIX="$P" -DCMAKE_INSTALL_LIBDIR=lib -DCMAKE_PREFIX_PATH="$P" -DCMAKE_POLICY_VERSION_MINIMUM=3.5 "$@" >"$T/deps-build.log" 2>&1 || { tail -20 "$T/deps-build.log"; exit 1; }
    cmake --build "$src/b" --target install >>"$T/deps-build.log" 2>&1 || { tail -20 "$T/deps-build.log"; exit 1; }
  }
  for a in "libogg-$OGG_VERSION" "libvorbis-$VORBIS_VERSION" "opus-$OPUS_VERSION" "lame-$LAME_VERSION"; do untar "$a" "$a.tar.gz"; done
  build_cmake "libogg-$OGG_VERSION" -DINSTALL_DOCS=OFF
  build_cmake "libvorbis-$VORBIS_VERSION" -DOGG_ROOT="$P"
  build_cmake "opus-$OPUS_VERSION" -DOPUS_BUILD_PROGRAMS=OFF -DOPUS_BUILD_TESTING=OFF
  # LAME: only the library, from its own configure.
  (cd "$T/src/lame-$LAME_VERSION" && CFLAGS="-O2 -fPIC" ./configure --prefix="$P" --disable-shared --enable-static --disable-frontend --disable-decoder >/dev/null \
    && make -s -j"$JOBS" >/dev/null && make -s install >/dev/null) >>"$T/deps-build.log" 2>&1 || { tail -20 "$T/deps-build.log"; exit 1; }
fi

# FFmpeg: only what a player needs, no network code, no libbluray, no libdvdread; LGPL (no
# --enable-gpl, no --enable-nonfree); shared libraries, so anyone can replace them. Hardware
# decoding through VA-API needs libva on the build machine and is left out: decoding is in
# software, which the graphics card still draws.
if [ ! -f "$P/lib/libavcodec.so" ] || [ ! -f "$P/.with-encoders" ]; then
  fetch "https://ffmpeg.org/releases/ffmpeg-$FFMPEG_VERSION.tar.xz" "ffmpeg-$FFMPEG_VERSION.tar.xz" "$FFMPEG_SHA256"
  untar "ffmpeg-$FFMPEG_VERSION" "ffmpeg-$FFMPEG_VERSION.tar.xz"
  . "$HERE/scripts/ffmpeg-components.sh"
  ASM=()
  [ "$ARCH" = x86_64 ] && ASM=(--x86asmexe=nasm)
  (
    cd "$T/src/ffmpeg-$FFMPEG_VERSION"
    PKG_CONFIG_PATH="$P/lib/pkgconfig" ./configure --prefix="$P" --enable-shared --disable-static --enable-version3 "${ASM[@]}" \
      --disable-autodetect --disable-network --disable-programs --disable-doc --disable-debug --disable-avdevice \
      --disable-everything --enable-zlib --enable-pthreads \
      --enable-libmp3lame --enable-libopus --enable-libvorbis \
      --enable-encoder="$ENC" --enable-muxer="$MUX" \
      --enable-decoder="$(list "$DEC")" --enable-demuxer="$(list "$DMX")" --enable-parser="$(list "$PAR")" \
      --enable-bsf="$(list "$BSF")" --enable-protocol=file \
      --enable-filter="$FILTERS" \
      --extra-cflags="-I$P/include" --extra-ldflags="-L$P/lib -Wl,-rpath,\$\$ORIGIN" \
      >"$T/ffmpeg-configure.log" 2>&1 || { tail -20 "$T/ffmpeg-configure.log"; tail -30 ffbuild/config.log; exit 1; }
    make -s -j"$JOBS" >"$T/ffmpeg-build.log" 2>&1 || { tail -30 "$T/ffmpeg-build.log"; exit 1; }
    make -s install >>"$T/ffmpeg-build.log" 2>&1
    touch "$P/.with-encoders"
  )
  grep -i "^License" "$T/ffmpeg-configure.log" || true
fi

echo "Toolchain ready in $T"
