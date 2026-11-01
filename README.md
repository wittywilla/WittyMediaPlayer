<h1 align="center">WittyMediaPlayer</h1>

<p align="center">
  <b>Video, audio, DVDs and Blu-rays, on your own computer.</b><br>
  Real disc menus, straight from a drive, a disc image or a folder. No server, no account, and nothing leaves your computer.
</p>

## What it does

- **Plays ordinary files:** MKV, MP4, AVI, MPEG, TS, WebM, WMV, MP3, AAC, FLAC, Ogg, WAV and more, with
  audio and subtitle track choice. Subtitle files beside a video (`Movie.srt`, `Movie.en.srt`) are found by themselves.
- **Plays DVDs and Blu-rays with their real menus:** arrows, Enter and the mouse work the disc's own
  buttons, with title, chapter and angle controls, using the same engines as
  [WittyMediaServer](https://github.com/wittywilla/WittyMediaServer).
- **Plays from a disc drive directly,** as well as from `.iso` and `.img` images and `VIDEO_TS` or `BDMV` folders.
- **Makes disc images:** copies a disc to one `.iso` or `.img` file to keep, play later, or add to a
  WittyMediaServer library.
- **Plays and copies audio CDs,** from a drive or an image (`.cue`, `.toc`, `.ccd`, `.nrg`, `.chd`, `.bin`), on a CD screen
  with its own glossy look and a visualizer: tick the tracks and copy them to FLAC, ALAC, WAV, MP3, AAC, Opus or Ogg
  Vorbis, with careful reading, tags and cover art, or make a `.cue` and `.bin` image of the disc.
- **Shows music as light:** [WittyVisualizer](https://github.com/wittywilla/WittyVisualizer) brings seventeen effects to audio files, full
  screen, with a flash limiter and a calm mode. It's off until you turn it on (V), and it costs nothing until then.
- **Says no plainly:** a copy-protected disc gets a clear message. Nothing here unlocks copy protection.
- **Runs well on small machines:** the picture is drawn by the graphics card, and hardware decoding is used where
  the card has it.
- **Can use the VLC libraries:** if you have [VLC](https://www.videolan.org/vlc/) installed, **File, Settings** can play
  through its libraries instead of the built-in FFmpeg, for formats the built-in choice can't play, and for surround
  sound. Thank you to VideoLAN and the VLC contributors, who make them. They aren't included here and aren't needed.
- **Updates its disc modules one at a time:** WittyDVD and WittyBluray can each take a newer signed version without a new
  player, from **File, Settings**. A version that keeps failing is taken back out by itself, and the built-in copy always works.
- **Keeps to itself:** no telemetry, and nothing leaves your computer unless you press a button that says so. **Look for
  updates** in Settings talks only to GitHub. **Look up names** on the CD screen sends only the disc's track layout to
  MusicBrainz, and fetches its cover from the Cover Art Archive. Its FFmpeg and its disc helper are built with no network code, and only a
  small updater program has any. (`Qt6Network.dll` is in the folder only because Qt's sound library won't load without it.
  Nothing calls it.)

## Install it

It needs Windows 10 version 1809 or newer, 64-bit. Download **`WittyMediaPlayer-<version>-windows-amd64.msi`** from
[Releases](https://github.com/wittywilla/WittyMediaPlayer/releases) and open it. The installer asks three things:

- **Who it's for.** Just you, which needs no administrator, or everyone on this computer, which goes in Program Files.
- **A shortcut on the desktop,** if you want one. It's always in the Start menu.
- **Which kinds of file to list it for,** under Open with: video, music and disc images. Nothing else changes: whatever
  opens those files now still opens them, until you choose WittyMediaPlayer in Windows Settings, under Apps, Default apps.

Windows may say it protected your PC from an unrecognised app, because the installer isn't signed yet. Choose **More
info**, then **Run anyway**.

A newer installer upgrades the one you have, keeping your settings and your answers.

### Without installing

**`WittyMediaPlayer-<version>-windows-amd64.zip`** is the same program as a folder: unzip it anywhere and run
`WittyMediaPlayer.exe`. Its settings are in the same place as an installed copy's.

### For administrators

```bash
msiexec /i WittyMediaPlayer-<version>-windows-amd64.msi /qn MSIINSTALLPERUSER=1
```

```bash
msiexec /i WittyMediaPlayer-<version>-windows-amd64.msi /qn MSIINSTALLPERUSER="" DESKTOPSHORTCUT=1 ASSOCDISC=0
```

| Property | Means | If you don't say |
|---|---|---|
| `MSIINSTALLPERUSER` | `1` just for this account; `""` for everyone (run it as an administrator) | just for this account |
| `DESKTOPSHORTCUT` | `1` puts a shortcut on the desktop | none |
| `ASSOCVIDEO`, `ASSOCMUSIC`, `ASSOCDISC` | `0` leaves that kind of file off the Open with list | listed |

An upgrade keeps the last answers to these, whatever a new command line says.

## Remove it

Use **Settings, Apps, Installed apps**, as for anything else, and choose WittyMediaPlayer. It tells you what it's about to
do, closes the player if it's open, and asks one question: whether to also delete your settings and the disc modules it
downloaded. Then it lists what it removed and anything it couldn't, and keeps that list in your temp folder.

A portable copy has the same `Uninstall WittyMediaPlayer.exe` in its folder.

| It removes | Where |
|---|---|
| The program | `%LOCALAPPDATA%\Programs\WittyMediaPlayer`, `C:\Program Files\WittyMediaPlayer` or the folder you unzipped: only the files in its `package-files.txt` |
| Shortcuts | The Start menu and the desktop |
| Its registry entries | Installed apps, Default apps, Open with, its file types, its own key, and what File Explorer noted about it (the place in Open with lists, a default you chose, its name) |
| If you say yes: your settings | `%APPDATA%\WittyMediaPlayer` |
| If you say yes: downloaded modules | `%LOCALAPPDATA%\WittyMediaPlayer\modules` |

The player keeps no logs and no caches of its own. Your media is never touched, and neither are the CDs you copied, the disc
images you made, or any file in those folders that the player didn't make: those are left and listed. As an administrator it
also clears the settings and shortcuts of other accounts on the computer, but it can't reach what File Explorer noted in
their registries. Windows' own history of what was opened, such as recent files, isn't touched.

For scripts:

```bash
"Uninstall WittyMediaPlayer.exe" /S
```

```bash
"Uninstall WittyMediaPlayer.exe" /S /RemoveData --report C:\Temp\removed.txt
```

```bash
"Uninstall WittyMediaPlayer.exe" --dry-run
```

`/S` asks nothing and keeps your settings; `/RemoveData` is a full clean-up. It returns at once and works in the
background, and the file named by `--report` appears when it has finished. `msiexec /x` also works, and keeps your settings.

## Use it

Open something with **File, Open**, by dropping it on the window, from the list of drives on the start page, from a
file's **Open with** menu, or from a command line:

```bash
WittyMediaPlayer.exe "C:\Movies\film.mkv"
```

```bash
WittyMediaPlayer.exe "C:\Discs\film.iso"
```

```bash
WittyMediaPlayer.exe D:
```

### What's in the folder

| Program | What it's for |
|---|---|
| `WittyMediaPlayer.exe` | The player. This is the one you open |
| `Uninstall WittyMediaPlayer.exe` | Removes it |
| `bin\wittymediaplayer-engine.exe` | Reads discs for the player, and makes disc images from a command line. No network code |
| `bin\wittymediaplayer-updater.exe` | Downloads updates to the disc modules and looks up CD names, only when you press a button that says so |

The rest are the libraries the player is built on, which Windows needs beside it. `licenses\` says what they are and
where their source is.

### Keyboard

| Key | Does |
|---|---|
| Space or K | Play or pause |
| Left, Right | Back and forward 10 seconds |
| Up, Down | Volume |
| F, Esc | Full screen, and out of it |
| A, S | Next audio track, next subtitle track |
| N, P | Next and previous chapter |
| 1 to 9 | Go to that chapter on a disc; that far through a file |
| M (files) | Mute |
| V | The visualizer, for music |

While the visualizer is on the whole screen or has the focus, Left and Right change the effect, R is random order, S a favourite, and + and - its strength.

On a disc, in its menus:

| Key | Does |
|---|---|
| Arrow keys, Enter | Move along the disc's buttons, and press one |
| M | The disc's menu, or its pop-up menu during a Blu-ray film |
| T | Title menu |
| Backspace | Go back up a menu |

Tab reaches every control, and screen readers can read them, including a disc's menu buttons.

### Making a disc image

Put the disc in a drive, then choose **File, Make disc image** or the button beside the drive on the start page.
To use the image in WittyMediaServer, save it in (or move it to) a folder one of your disc libraries watches, and
scan the library. The same copy runs from a command line, and never overwrites anything:

```bash
bin\wittymediaplayer-engine.exe --image D: "C:\Discs\film.iso"
```

Discs protected with CSS or AACS are refused before anything is written. Audio CDs are imaged from the CD screen.

### Audio CDs

Open a CD from the start page or **File, Play from drive**, or open an image of one. The CD screen shows the tracks with
a tick beside each, and waits for you to press play.

| Key | What it does |
|---|---|
| Space or K | Play or pause |
| N and P | Next and previous track |
| S | Stop |
| Left and Right | Back and forward 10 seconds |
| Up and Down | Volume |
| V | Turn the visuals on, then the next simple style |
| E | Switch between the simple visualizer and WittyVisualizer's effects |
| Enter on a track | Play it |
| F2 on a track | Change its name |

**Copying:** tick the tracks, choose a format, a quality and a folder, then press **Copy ticked tracks**. Each row shows its
own progress. A file is written under a temporary name and only takes its real name when it's finished, so a stopped copy
leaves nothing behind, and nothing is ever overwritten: a second copy gets `(2)` in its name. A short report goes in the
same folder, with each track's checksum and how its reading went.

**Names:** the box takes `{albumartist}`, `{artist}`, `{album}`, `{title}`, `{track}`, `{tracks}`, `{disc}` and `{year}`, and a
slash makes a folder. The default is `{albumartist}/{album}/{track} {title}`.

**Careful reading:** with **Check every read twice** on, each track is read twice, and read again wherever the two disagree,
until two reads match. A spot that never reads becomes silence, and that track's row and the report say how much.
**Drive offset** corrects a drive that reads a few samples early or late. Leave it at 0 unless you know your drive's figure.
An image reads the same every time, so it needs neither.

A disc that can't be read the ordinary way gets a plain message. WittyMediaPlayer has no way around copy protection on a CD
any more than on a DVD.

**The look:** the box at the top right switches between the Classic look and your app theme.

**The visuals:** they're off until you turn them on, and the first time, you're told about flashing light and can choose
calm mode. Bars, Scope, Rings and Plasma are drawn on the processor, so they run anywhere. **More effects** brings in
WittyVisualizer's seventeen effects, drawn by your graphics card, full screen if you like. Everything goes through a flash
limiter that no style can get around.

### Trying a real CD

This was built on a laptop with no disc drive, so a real disc is yours to try:

1. Put in an ordinary music CD. On the start page, its row should say **audio CD**. Press **Open CD**.
2. The tracks should be listed with the right lengths. Press play and listen, then try next, previous and seeking.
3. Press **Look up names**. Album, artist, track names and cover should fill in, or it says MusicBrainz doesn't know the disc.
4. Tick two short tracks, choose FLAC and a folder, and press **Copy ticked tracks**. Both should finish as "Done, checked".
   Play the files in the player, and read the report beside them.
5. Do the same with MP3. Check the tags and the cover show in another program.
6. Press **Make image**, then open the `.cue` it wrote. It should show the same tracks and sound the same.
7. Press **Stop copying** part way through a copy. The unfinished file should be gone.
8. If you have a scratched disc, try it: the row should say how many sectors were bad, rather than stopping or hiding it.
9. Take the disc out while it plays. It should say the disc was taken out.

## What doesn't work yet

- Blu-ray menus written in Java don't run. The disc plays its titles instead, and the window says so.
- Sound is stereo with the built-in playback. Surround tracks are mixed down.
- Windows only for now, so there's no installer or uninstaller for macOS or Linux. Nothing in the design stops other
  systems: see [DESIGN.md](DESIGN.md).

## Build it

You need Git for Windows, Python 3 and Go, and a clone of
[WittyVisualizer](https://github.com/wittywilla/WittyVisualizer) beside this one. Everything else is fetched for
your user only, with no administrator rights and no accounts. Run these in Git Bash from the repository's top folder:

```bash
bash scripts/setup-toolchain.sh
```

```bash
bash scripts/build-windows.sh
```

The portable folder and zip land in `.build/`. The installer is made from that folder, with the
[WiX Toolset](https://wixtoolset.org) 6 on the PATH or at `WIX_EXE` (the top of the script says how to get it). Run this in
PowerShell, with the version from `CMakeLists.txt`:

```powershell
packaging\windows\build-msi.ps1 -Version <version> -PackageDir .build\WittyMediaPlayer-<version>-windows-amd64 -OutDir .build
```

To run the tests:

```bash
bash scripts/test.sh
```

They need no one at the screen and use Qt's offscreen platform. Set `WITTY_TEST_DVD` and `WITTY_TEST_BD` to
disc images to include real discs; those are only ever read.

To test the installer, neither of these installs anything on the computer you run it on:

- `scripts\test\check-msi.ps1` reads the MSI's tables and unpacks it to a folder of your choosing, to check what's in it.
- `scripts/test/installers-wine.sh` really installs, upgrades and removes it, for one person and for everyone, in a throwaway
  Docker container with Wine, and compares every file and registry entry from before and after.

`engine/internal/uninstall` has tests that read the player's source and the installer's: they fail if the player starts
writing somewhere the uninstaller doesn't know, or the installer writes something it wouldn't remove.

## Licence

**Apache-2.0**, the same as WittyMediaServer. © 2026 WittyWilla and contributors. If you redistribute it or build
on it, keep the [NOTICE](NOTICE) attribution, *"WittyMediaPlayer, developed by WittyWilla"*.

Qt and FFmpeg ship as separate libraries under the LGPL, so you can replace them. See
[licenses/THIRD-PARTY.txt](licenses/THIRD-PARTY.txt).

### Credits

- **[VLC and libVLC](https://www.videolan.org/vlc/libvlc.html)**, by VideoLAN and the VLC contributors, under the LGPL 2.1
  or later, when you choose them in Settings. WittyMediaPlayer loads them from your own VLC and doesn't ship them.
  It isn't part of VLC or made by VideoLAN.
- **[Qt](https://www.qt.io)**, by The Qt Company and contributors, for the window.
- **[FFmpeg](https://ffmpeg.org)**, by the FFmpeg developers, for built-in decoding.
- **WittyDVD**, **WittyBluray**, **WittyDisc** and **WittyModule**, by WittyWilla, for discs.

## Legal

WittyMediaPlayer is an independent open-source project. It isn't associated with, sponsored by or
endorsed by the owners of the DVD, Blu-ray or Compact Disc formats, any studio or label, or any other
player project. Your files and what you open are yours: no media comes with it, encrypted discs are
refused, nothing is decrypted, and you're responsible for having the rights to what you open or rip. Any
third-party names here only describe what it works with, and belong to their owners. It's provided
“as is”, with no warranty; see [LICENSE](LICENSE). Local law applies to how you use it. This isn't legal
advice.
