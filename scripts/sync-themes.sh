#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 WittyWilla and contributors
#
# Copies WittyMediaServer's design tokens and its fifteen built-in themes into resources/, where the
# player draws them from. The server's themes/ folder is the source: never edit the copies.
#
#   scripts/sync-themes.sh            write them
#   scripts/sync-themes.sh --check    change nothing; fail if any copy differs
#
# WITTY_SERVER is the WittyMediaServer checkout (default: beside this repository).
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
server=${WITTY_SERVER:-$here/../WittyMediaServer}
check=0
[ "${1:-}" = "--check" ] && check=1
[ -f "$server/themes/tokens.json" ] || { echo "WittyMediaServer isn't at $server: set WITTY_SERVER to its checkout." >&2; exit 1; }
bad=0
sync() { # source destination
  if [ $check = 1 ]; then
    cmp -s "$1" "$2" || { echo "out of date: $2" >&2; bad=1; }
  else
    mkdir -p "$(dirname "$2")"; cp "$1" "$2"
  fi
}
sync "$server/themes/tokens.json" "$here/resources/tokens.json"
for id in default midnight cinema evergreen oled light paper contrast aeroday aeroaurora aeromeadow aerosunset cable satellite teletext; do
  sync "$server/themes/$id/theme.json" "$here/resources/themes/$id.json"
done
exit $bad
