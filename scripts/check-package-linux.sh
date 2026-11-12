#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 WittyWilla and contributors
# Checks a Linux AppDir against the rules every package keeps (scripts/check-package.sh is the
# Windows one). Stops the build if any fail.
#
#   bash scripts/check-package-linux.sh <AppDir>
set -euo pipefail
APP="$1"
fail=0
say() { echo "check-package-linux: $*" >&2; fail=1; }
elves() { find "$APP/usr/bin" "$APP/usr/lib" "$APP/usr/plugins" -type f \( -name '*.so*' -o -perm -u+x \) 2>/dev/null; }

# Nothing may carry or load a disc-decryption library or a key database.
for pattern in libdvdcss dvdcss libaacs libbdplus libmmbd KEYDB keydb.cfg libbluray libdvdread libdvdnav; do
  while IFS= read -r f; do
    say "$pattern is mentioned in ${f#$APP/}"
  done < <(elves | xargs grep -lai -- "$pattern" 2>/dev/null || true)
done
for f in "$APP"/usr/lib/*; do
  case "$(basename "$f" | tr A-Z a-z)" in
    *dvdcss*|*aacs*|*bdplus*|*bluray*|*dvdread*|*dvdnav*) say "$(basename "$f") must not ship" ;;
  esac
done

# Nothing may reach the network. Qt6Network comes only with Qt's sound library; the player
# itself must not link to it, and Qt's network plugins stay out.
if command -v readelf >/dev/null && readelf -d "$APP/usr/bin/WittyMediaPlayer" | grep -q "libQt6Network"; then
  say "the player must not link to Qt6Network"
fi
for d in tls networkinformation multimedia; do [ ! -e "$APP/usr/plugins/$d" ] || say "Qt's $d plugins must not ship"; done
for f in "$APP"/usr/lib/libav*; do
  if grep -qa -- "http_protocol\|tcp_protocol\|tls_protocol" "$f"; then say "$(basename "$f") has network code"; fi
done
if grep -qa -- "--enable-network" "$APP"/usr/lib/libavformat.so*; then say "FFmpeg was built with its network code"; fi
# Copying CDs needs every encoder the screen offers.
for enc in libmp3lame libopus libvorbis; do
  grep -qa -- "$enc" "$APP"/usr/lib/libavcodec.so* || say "FFmpeg was built without $enc"
done
# FFmpeg must be the LGPL build.
if grep -qa -- "--enable-gpl\|--enable-nonfree" "$APP"/usr/lib/libavcodec.so*; then say "FFmpeg must not be built with GPL or non-free parts"; fi

# The programs: the window, and its two helpers where it looks for them.
for f in AppRun usr/bin/WittyMediaPlayer usr/bin/bin/wittymediaplayer-engine usr/bin/bin/wittymediaplayer-updater; do
  [ -x "$APP/$f" ] || say "$f is missing or can't run"
done
got_bin="$(cd "$APP/usr/bin/bin" && ls -1 | LC_ALL=C sort)"
[ "$got_bin" = "wittymediaplayer-engine
wittymediaplayer-updater" ] || say "usr/bin/bin has these: $got_bin"

# The menu entry, its icon and its description.
APP_ID=com.wittywilla.WittyMediaPlayer
for f in "$APP_ID.desktop" "$APP_ID.png" .DirIcon usr/share/applications/$APP_ID.desktop usr/share/metainfo/$APP_ID.metainfo.xml \
  usr/share/icons/hicolor/256x256/apps/$APP_ID.png usr/share/icons/hicolor/scalable/apps/$APP_ID.svg; do
  [ -e "$APP/$f" ] || say "$f is missing"
done
if grep -q "@VERSION@" "$APP/usr/share/metainfo/$APP_ID.metainfo.xml" 2>/dev/null; then say "the metainfo has no version"; fi

# The licences have to go with it.
D="$APP/usr/share/doc/wittymediaplayer"
for f in LICENSE NOTICE README.md licenses/THIRD-PARTY.txt licenses/LGPL-3.0.txt licenses/GPL-3.0.txt licenses/AGPL-3.0.txt; do
  [ -s "$D/$f" ] || say "$f is missing"
done

[ "$fail" = 0 ] && echo "check-package-linux: all good"
exit "$fail"
