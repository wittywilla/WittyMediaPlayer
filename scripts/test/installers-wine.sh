#!/usr/bin/env bash
# Proves the MSI by really running it, in a throwaway container with Wine: install for one person
# and for everyone, upgrade in place, uninstall with and without /RemoveData, and the files and the
# registry from before and after, to show nothing is left. Wine isn't Windows, so a real Windows has
# the last word, but Windows Installer's tables, the uninstaller and the registry are all exercised.
#
# Wine's Windows Installer doesn't do what real Windows does with MSIINSTALLPERUSER=1: it installs
# for everyone. So "just for me" here says ALLUSERS="", which writes to this account's registry as real
# Windows would, but it stays in Program Files, where real Windows would use %LOCALAPPDATA%\Programs.
# It also can't be given an empty property on a command line from here: "" arrives as two backslashes.
#
#   scripts/test/installers-wine.sh <folder> [screenshots]
#
# The folder holds WittyMediaPlayer-*-windows-amd64.msi, and upgrade/ the same at version 9.9.9.
# Writes its listing to <folder>/results/wine.log. With "screenshots" it also photographs the wizard.
set -uo pipefail

PKGS="$(cd "${1:?folder with the msi}" && pwd)"
SHOTS="${2:-}"
RESULTS="$PKGS/results"; mkdir -p "$RESULTS"
log="$RESULTS/wine.log"
tag=wmp-installer-test-wine

docker build -q -t "$tag" - >"$log" 2>&1 <<'EOF'
FROM debian:trixie-slim
RUN dpkg --add-architecture i386 && apt-get update && apt-get install -y --no-install-recommends \
    wine wine64 wine32:i386 xvfb xauth x11-utils imagemagick xdotool procps file fonts-liberation
EOF
[ $? -eq 0 ] || { echo "FAILED to prepare the image: see $log"; tail -n 20 "$log"; exit 1; }

docker rm -f wmp-inst-wine >/dev/null 2>&1
docker run -i --rm --name wmp-inst-wine --cpus 6 -e SHOTS="$SHOTS" -v "$PKGS:/pkgs" "$tag" bash -s >>"$log" 2>&1 <<'SCRIPT'
set -u
export WINEPREFIX=/root/wine WINEARCH=win64 WINEDEBUG=-all DISPLAY=:99
Xvfb :99 -screen 0 1024x768x24 >/dev/null 2>&1 &
sleep 1
fail() { echo "FAIL: $*"; exit 1; }
say() { echo; echo "##### $*"; }
wineboot -i >/dev/null 2>&1
wineserver -w
msi=$(ls /pkgs/WittyMediaPlayer-*-windows-amd64.msi)
new=$(ls /pkgs/upgrade/WittyMediaPlayer-*-windows-amd64.msi)
C="$WINEPREFIX/drive_c"
USERDIR="$C/users/root"
UN="Uninstall WittyMediaPlayer.exe"
JUST_ME='ALLUSERS=""'

# Every file, and every registry key and value, with what Wine itself changes taken out. It isn't
# filtered for our name: anything left behind shows, whatever it's called.
snapshot() {
  wineserver -w
  (cd "$C" && find . -path ./windows/Installer -prune -o -path ./windows/logs -prune -o -path ./windows/temp -prune \
      -o -path ./users/root/Temp -prune -o -path ./users/root/AppData/Local/Temp -prune -o -path ./users/root/AppData/Roaming/Microsoft/Installer -prune -o -name 'msi.log' -prune -o -name '*.desktop' -prune -o -print | sort)
  echo "--- registry"
  for f in system.reg user.reg; do
    awk -v f="$f" '
      /^\[/ { key=$0; sub(/\] [0-9]+$/, "]", key); print f " " key; next }
      /^"/  { split($0, a, "="); print f " " key " " a[1] }
      /^@=/ { print f " " key " @" }
    ' "$WINEPREFIX/$f"
  done | grep -avF 'user.reg [Software\\Classes]' | grep -avE 'Wine\\\\MenuFiles|Windows\\\\CurrentVersion\\\\Installer\\\\|Software\\\\Classes\\\\Installer\\\\|Software\\\\Microsoft\\\\Installer\\\\|NetworkCards|Volatile|Software\\\\Wine\\\\(Fonts|MSHTML|Debug)|MuiCache|ActiveComputerName|\\\\Windows\\\\CurrentVersion\\\\Run|Perflib|Dynamic Data|DynData|RunOnce|Session Manager\\\\Environment|Explorer\\\\(Advanced|Shell Folders|User Shell Folders)|^user.reg \[Environment\]|^user.reg \[Volatile|\\\\Installer\\\\(Rollback|InProgress)|Software\\\\Microsoft\\\\Cryptography|ComDlg32|Software\\\\Classes\\\\Interface|\\\\Enum\\\\' | sort
}
# A key, by its whole name. Windows Installer's own records mention a component's keys even when
# it wasn't installed, so anything looser finds those.
has_key() { grep -aqiF "[$1]" "$WINEPREFIX/system.reg" "$WINEPREFIX/user.reg"; }
# The value of a registry value, as Wine's files hold it.
# Windows Installer leaves the parents of what it removes; the uninstaller clears those if they're
# empty, and msiexec /x doesn't.
ignore_parents() { grep -avE '^user\.reg \[Software\\\\(Classes\\\\Applications|RegisteredApplications|WittyWilla|Microsoft\\\\Windows\\\\CurrentVersion\\\\(App Paths|Uninstall))\]$'; }
reg_val() { grep -a -A40 -iF "$1]" "$WINEPREFIX/$3" | awk -v n="\"$2\"" 'NR>1 && /^\[/ {exit} index($0, n"=")==1 {print; exit}'; }

run_msi() {
  wine msiexec "$@" /l*v 'C:\msi.log'
  code=$?
  wineserver -w
  return $code
}
run_uninstaller() { # waits for the worker it starts in the background
  wine "$(winepath -w "$root/$UN")" "$@"
  echo "uninstaller exit code: $?"
  wineserver -w; sleep 6; wineserver -w
}
# Fails if anything differs from the first snapshot.
same_as_before() {
  snapshot > /tmp/now.txt
  if ! diff -u /tmp/before.txt /tmp/now.txt > /tmp/d.txt; then
    grep -E '^[+-][^+-]' /tmp/d.txt
    fail "$1"
  fi
  echo "NOTHING LEFT ($1): $(wc -l < /tmp/now.txt) paths and registry entries, before and after the same"
}

say "snapshot before"
snapshot > /tmp/before.txt
wc -l /tmp/before.txt

if [ -n "${SHOTS:-}" ]; then
  say "photographing the wizard"
  mkdir -p /pkgs/results/wizard
  wine msiexec /i "$msi" >/dev/null 2>&1 &
  sleep 25
  shoot() { import -window root "/pkgs/results/wizard/$1.png" 2>/dev/null; }
  shoot 1-welcome
  xdotool key Return; sleep 3; shoot 2-who-is-it-for
  xdotool key Return; sleep 3; shoot 3-choices
  xdotool key Return; sleep 3; shoot 4-ready
  pkill -f msiexec
  wineserver -k; sleep 2
  snapshot > /tmp/after-shots.txt
  diff /tmp/before.txt /tmp/after-shots.txt | grep -E '^[<>]' | head -20
fi

say "install just for me, silently, with a desktop shortcut and no music: $msi"
run_msi /i "$msi" /qn "$JUST_ME" DESKTOPSHORTCUT=1 ASSOCMUSIC=0 || fail "install exit $code"
snapshot > /tmp/installed.txt
diff /tmp/before.txt /tmp/installed.txt | grep '^>' | grep -v 'OpenWithProgids\|SupportedTypes\|FileAssociations\|Software\\\\Classes\\\\\.[a-z0-9]*\]$\|Qt6\|\.dll$' | head -70
dir=$(find "$C" -type f -name WittyMediaPlayer.exe | head -1)
[ -n "$dir" ] || fail "the app wasn't installed"
root=$(dirname "$dir")
echo "installed to: $root"
for f in "$UN" package-files.txt bin/wittymediaplayer-engine.exe bin/wittymediaplayer-updater.exe Qt6Core.dll avcodec-62.dll platforms/qwindows.dll licenses/THIRD-PARTY.txt; do
  [ -f "$root/$f" ] || fail "$f is missing"
done
[ ! -e "$root/wittymediaplayer-engine.exe" ] || fail "the engine is at the top"
find "$C" -iname 'WittyMediaPlayer.lnk' | sed 's/^/shortcut: /'
find "$C" -path '*Start Menu*' -iname 'WittyMediaPlayer.lnk' | grep -q . || fail "no Start menu entry"
find "$C" -ipath '*desktop*' -iname 'WittyMediaPlayer.lnk' | grep -q . || fail "no desktop shortcut"
ARP='Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\WittyMediaPlayer'
VIDEO='Software\\Classes\\WittyMediaPlayer.Video'
MUSIC='Software\\Classes\\WittyMediaPlayer.Music'
DISC='Software\\Classes\\WittyMediaPlayer.Disc'
grep -aqF "[$ARP]" "$WINEPREFIX/user.reg" || fail "no Installed apps entry for this account"
reg_val "$ARP" ProductCode user.reg | grep -q '{' || fail "no product code in the entry"
reg_val "$ARP" UninstallString user.reg
reg_val "$ARP" QuietUninstallString user.reg
has_key "$VIDEO" || fail "video files weren't registered"
has_key "$DISC" || fail "disc images weren't registered"
has_key 'Software\\Classes\\.mkv\\OpenWithProgids' || fail ".mkv has no Open with entry"
has_key "$MUSIC" && fail "music files were registered after saying no"
has_key 'Software\\Classes\\.flac\\OpenWithProgids' && fail ".flac has an Open with entry after saying no"
grep -aF -A3 '[Software\\Classes\\.mkv]' "$WINEPREFIX/user.reg" | grep -a '^@' && fail "it took over what opens .mkv"

say "the helpers run from bin"
wine "$root/bin/wittymediaplayer-engine.exe" --version
wine "$root/bin/wittymediaplayer-engine.exe" --drives; echo "(--drives exit $?)"
wine "$(winepath -w "$root/$UN")" --dry-run --report 'C:\dry.txt'; cat "$C/dry.txt"; rm -f "$C/dry.txt"
wineserver -w

say "the window under Wine"
# Worth trying, and not the point of this test: Qt's graphics under Wine vary. It says so if not.
(QT_QPA_PLATFORM=windows timeout 60 wine "$root/WittyMediaPlayer.exe" --no-audio-device --script 'wait 8;windowgrab C:\window.png;quit'; echo "window exit code: $?") 2>&1 | tail -3
if [ -f "$C/window.png" ]; then cp "$C/window.png" /pkgs/results/window.png; echo "window grabbed"; rm -f "$C/window.png"; else echo "NOTE: no picture of the window under Wine"; fi
wineserver -k; sleep 2
# An automated check keeps its settings to itself: it's the uninstaller's to tidy, tested below.
ls "$USERDIR/AppData/Local/Temp/wittymediaplayer-script-settings" 2>/dev/null | sed 's/^/script settings: /'

say "upgrade in place: $new"
grep -a '^"Chose' "$WINEPREFIX/user.reg"
echo mine > "$root/my-own-file.txt"
was=$(reg_val "$ARP" ProductCode user.reg)
run_msi /i "$new" /qn "$JUST_ME" || fail "upgrade exit $code"
grep -a 'Product: WittyMediaPlayer' "$C/msi.log" | tail -n 3
is=$(reg_val "$ARP" ProductCode user.reg)
echo "product code before $was after $is"
[ "$was" != "$is" ] || fail "the product didn't change"
reg_val "$ARP" DisplayVersion user.reg | tee /tmp/v
grep -q 9.9.9 /tmp/v || fail "Installed apps still shows the old version"
wine "$root/bin/wittymediaplayer-engine.exe" --version
[ -f "$root/my-own-file.txt" ] || fail "the upgrade took a file it didn't make"
[ "$(find "$C" -type f -name WittyMediaPlayer.exe | wc -l)" = 1 ] || fail "there are two copies now"
find "$C" -ipath '*desktop*' -iname 'WittyMediaPlayer.lnk' | grep -q . || fail "the upgrade forgot the desktop shortcut"
grep -a '^"Chose' "$WINEPREFIX/user.reg"
has_key "$MUSIC" && fail "the upgrade forgot that music files were left out"
has_key "$VIDEO" || fail "the upgrade dropped the video files"
wineserver -w

say "what a player that's been used leaves"
settings="$USERDIR/AppData/Roaming/WittyMediaPlayer"
local_data="$USERDIR/AppData/Local/WittyMediaPlayer"
mkdir -p "$settings" "$local_data/modules/wittydvd/versions/0.2.0"
printf '[General]\nvolume=0.5\n' > "$settings/WittyMediaPlayer.ini"
echo '{}' > "$local_data/modules/wittydvd/state.json"
echo x > "$local_data/modules/wittydvd/versions/0.2.0/helper.exe"
echo notes > "$local_data/my-own-notes.txt"
mkdir -p "$USERDIR/Videos" "$USERDIR/Music/A Made-Up Album"
echo film > "$USERDIR/Videos/A Made-Up Film.mkv"
echo song > "$USERDIR/Music/A Made-Up Album/01 A Made-Up Song.flac"

say "uninstall silently, keeping settings"
run_uninstaller /S
[ ! -e "$root/WittyMediaPlayer.exe" ] || fail "the program stayed"
[ ! -e "$root/bin" ] || fail "bin stayed"
[ -f "$settings/WittyMediaPlayer.ini" ] || fail "a keep-data uninstall took the settings"
[ -f "$local_data/modules/wittydvd/state.json" ] || fail "a keep-data uninstall took the modules"
[ -f "$root/my-own-file.txt" ] || fail "took a file it didn't make"
has_key "$ARP" && fail "the Installed apps entry stayed"
has_key "$VIDEO" && fail "the file types stayed"
find "$C" -iname 'WittyMediaPlayer.lnk' | grep . && fail "a shortcut stayed"
ls "$USERDIR/AppData/Local/Temp" | grep -i 'WittyMediaPlayer-uninstall' | sed 's/^/report: /'
cat "$USERDIR"/AppData/Local/Temp/WittyMediaPlayer-uninstall.txt
rm -f "$USERDIR"/AppData/Local/Temp/WittyMediaPlayer-uninstall.txt
rm -f "$root/my-own-file.txt"; rmdir "$root" 2>/dev/null

say "install again, for full removal"
run_msi /i "$msi" /qn "$JUST_ME" DESKTOPSHORTCUT=1 || fail "reinstall exit $code"
root=$(dirname "$(find "$C" -type f -name WittyMediaPlayer.exe | head -1)")
has_key "$MUSIC" || fail "a fresh install should offer music files again"

say "uninstall silently, removing everything"
run_uninstaller /S /RemoveData --report 'C:\report.txt'
echo "--- what it says it did"; cat "$C/report.txt" || fail "no report"
grep -q 'my-own-notes.txt' "$C/report.txt" || fail "it doesn't say it left the file it didn't make"
[ ! -e "$settings/WittyMediaPlayer.ini" ] || fail "the settings stayed"
[ ! -e "$settings" ] || fail "the settings folder stayed"
[ ! -e "$local_data/modules" ] || fail "the modules stayed"
[ -f "$local_data/my-own-notes.txt" ] || fail "took a file it didn't make"
[ -f "$USERDIR/Videos/A Made-Up Film.mkv" ] || fail "A FILM WAS DELETED"
[ -f "$USERDIR/Music/A Made-Up Album/01 A Made-Up Song.flac" ] || fail "A COPIED CD WAS DELETED"
ls -d "$USERDIR"/AppData/Local/Temp/wittymediaplayer-uninstall-* 2>/dev/null && fail "the worker's copy was left in the temp folder"
rm -f "$local_data/my-own-notes.txt" "$USERDIR/Videos/A Made-Up Film.mkv" "$USERDIR/Music/A Made-Up Album/01 A Made-Up Song.flac" "$C/report.txt"
rmdir "$local_data" "$USERDIR/Music/A Made-Up Album"
rm -rf "$USERDIR/AppData/Local/Temp/wittymediaplayer-script-settings"
same_as_before "after a full removal, just for me"

say "msiexec /x works too, and keeps settings"
run_msi /i "$msi" /qn "$JUST_ME" || fail "install exit $code"
root=$(dirname "$(find "$C" -type f -name WittyMediaPlayer.exe | head -1)")
mkdir -p "$settings"; echo x > "$settings/WittyMediaPlayer.ini"
run_msi /x "$msi" /qn "$JUST_ME" || fail "msiexec /x exit $code"
[ ! -e "$root/WittyMediaPlayer.exe" ] || fail "msiexec /x left the program"
[ -f "$settings/WittyMediaPlayer.ini" ] || fail "msiexec /x took the settings"
rm -rf "$settings"
snapshot | ignore_parents > /tmp/now.txt
ignore_parents < /tmp/before.txt > /tmp/was.txt
if ! diff -u /tmp/was.txt /tmp/now.txt > /tmp/d.txt; then grep -E '^[+-][^+-]' /tmp/d.txt; fail "msiexec /x left something"; fi
echo "NOTHING LEFT (after msiexec /x) but the empty parent keys Windows Installer leaves"
# So the next check starts where the first did: an uninstaller's run clears these.
wine reg delete 'HKCU\Software\Classes\Applications' /f >/dev/null 2>&1
wine reg delete 'HKCU\Software\RegisteredApplications' /f >/dev/null 2>&1
wine reg delete 'HKCU\Software\WittyWilla' /f >/dev/null 2>&1
wine reg delete 'HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths' /f >/dev/null 2>&1
wine reg delete 'HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall' /f >/dev/null 2>&1
wineserver -w

say "for everyone"
run_msi /i "$msi" /qn 'MSIINSTALLPERUSER=""' DESKTOPSHORTCUT=1 || fail "machine install exit $code"
root="$C/Program Files/WittyMediaPlayer"
[ -f "$root/WittyMediaPlayer.exe" ] || fail "not in Program Files: $(find "$C" -name WittyMediaPlayer.exe)"
[ -f "$root/bin/wittymediaplayer-engine.exe" ] || fail "no helpers in Program Files"
grep -aqF "[$ARP]" "$WINEPREFIX/system.reg" || fail "the Installed apps entry isn't for the machine"
grep -aqF "[$VIDEO]" "$WINEPREFIX/system.reg" || fail "file types aren't for the machine"
find "$C" -iname 'WittyMediaPlayer.lnk' | sed 's/^/shortcut: /'

say "an upgrade of it stays for everyone"
run_msi /i "$new" /qn || fail "machine upgrade exit $code"
[ -f "$root/WittyMediaPlayer.exe" ] || fail "the program moved"
[ "$(find "$C" -type f -name WittyMediaPlayer.exe | wc -l)" = 1 ] || fail "the upgrade made a second copy: $(find "$C" -name WittyMediaPlayer.exe)"
reg_val "$ARP" DisplayVersion system.reg

say "uninstall it, removing everything"
run_uninstaller /S /RemoveData --report 'C:\report.txt'
cat "$C/report.txt" || fail "no report"; rm -f "$C/report.txt"
[ ! -e "$root/WittyMediaPlayer.exe" ] || fail "the program stayed in Program Files"
same_as_before "after a full removal, for everyone"

say "a portable copy, from the zip's files"
portable="$C/Tools/WittyMediaPlayer-portable"
mkdir -p "$C/Tools"
cp -r /pkgs/portable "$portable"
echo mine > "$portable/my-own-file.txt"
mkdir -p "$settings"; echo x > "$settings/WittyMediaPlayer.ini"
root="$portable"
run_uninstaller /S /RemoveData --report 'C:\report.txt'
cat "$C/report.txt" || fail "no report"; rm -f "$C/report.txt"
[ ! -e "$portable/WittyMediaPlayer.exe" ] || fail "the portable program stayed"
[ -f "$portable/my-own-file.txt" ] || fail "took a file it didn't make from the portable folder"
[ ! -e "$settings" ] || fail "the settings stayed"
rm -rf "$C/Tools"
same_as_before "after removing a portable copy"

if [ -n "${SHOTS:-}" ]; then
  say "uninstall as a person does: the questions, the answers, the result"
  run_msi /i "$msi" /qn "$JUST_ME" || fail "install exit $code"
  root=$(dirname "$(find "$C" -type f -name WittyMediaPlayer.exe | head -1)")
  mkdir -p "$settings"; echo x > "$settings/WittyMediaPlayer.ini"
  shoot() { import -window root "/pkgs/results/wizard/$1.png" 2>/dev/null; }
  wine "$(winepath -w "$root/$UN")" >/dev/null 2>&1 &
  sleep 8; shoot 5-uninstall-asks
  xdotool key Return; sleep 4; shoot 6-uninstall-asks-about-settings
  xdotool key Return; sleep 12; shoot 7-uninstall-says-what-it-did
  xdotool key Return; sleep 3
  wineserver -w
  [ ! -e "$root/WittyMediaPlayer.exe" ] || fail "answering yes left the program"
  [ ! -e "$settings/WittyMediaPlayer.ini" ] || fail "answering yes left the settings"
  rm -f "$USERDIR"/AppData/Local/Temp/WittyMediaPlayer-uninstall.txt
  same_as_before "after a person's uninstall"
fi
echo ALLPASSED
SCRIPT
if tail -n 1 "$log" | grep -q ALLPASSED; then
  echo "passed: $log"
else
  echo "FAILED: $log"
  tail -n 40 "$log"
  exit 1
fi
