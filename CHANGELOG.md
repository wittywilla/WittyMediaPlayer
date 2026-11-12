# Changelog

What changed in WittyMediaPlayer, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

- **Design 2: the screens are server-driven UI.** The start page (open, drives, recent), Settings (what plays your media, disc options), Disc modules, Look, About, a disc's guide, Why this title, Disc health and WittyRemote are now SDUI documents, the same kind WittyMediaServer sends its apps, made by the engine from the player's own data (`engine/internal/pages`) and drawn by one Qt renderer (`src/sdui`). Nothing about them needs a server. Playback, disc menus, the control bar and the visualizer stay native. **Disc, Disc guide** lists the film, its other cuts, episodes and the other titles; **Disc health** says what couldn't be read and what was mended; **View, WittyRemote** (Ctrl+R) is the disc remote, its keys from the catalogue, the ones a disc doesn't use dimmed and in place. The old Settings window and the message box for Why this title are gone. See DESIGN.md.
- **Themes are the server's.** The fifteen themes of WittyMediaServer (Witty, Midnight, Cinema, Evergreen, True black, Light, Paper, High contrast, Aero Day, Aero Aurora, Aero Meadow, Aero Sunset, Cable blue, Satellite, Teletext) replace the ten dark ones; an earlier choice moves to the closest. **View, Look** has them as cards that wear their own colours, **Import a theme…** takes a ZIP by the server's rules (variables and fonts only, readable text, no way out of the ZIP), and Aero's gloss and glass, Satellite's bevel and the themes' scenes are drawn, with a **Flat** choice. The first time the player starts, the Look is chosen first. Copies are made by `scripts/sync-themes.sh`.

- **Linux:** an AppImage for 64-bit Linux (`scripts/setup-toolchain-linux.sh`, `scripts/build-linux.sh`), with a menu entry, its icon and an AppStream description (`packaging/linux/`), checked by `scripts/check-package-linux.sh` like the Windows package: no network code in its FFmpeg, no disc-decryption library. Both builds now take FFmpeg's parts from one list, `scripts/ffmpeg-components.sh`. Not yet built or run on Linux.
- The window, its dialogs and the About box carry the new icon on every system, not only the Windows program file.
- The test that the player's FFmpeg has no network code says why it is skipped on a developer's FFmpeg (Homebrew's, a distribution's) instead of failing; the release's test run (`scripts/test.sh`) still requires it.
- A new icon in the family's new look: a ring with a play button inside and a sky-blue dot, on the dark tile. It and the installer's pictures are now drawn by WittyMediaServer's genicon, the generator for every logo of the family (`scripts/sync-brand.sh`), which replaces this repository's own `engine/tools/genicon`.
Version 0.1.3. Version 0.1.2 was tagged but never published, because its build failed on GitHub, so everything below
arrives with 0.1.3.

- **The film on a disc that hides it.** The player now uses the disc engines' ranking of a disc's titles in full. **Disc,
  Play main feature** plays the film whole: a film the disc cuts into several titles plays as one, with one seek bar, and
  one its commands put together plays in their order. **Disc, Why this title** says why that title was picked and how sure
  the pick is, with the engine's warnings, where the disc's index was mended and what couldn't be read. A film's other cuts
  are under **Versions** (DVD) or **Editions** (Blu-ray), a TV disc's episodes under **Episodes**, and decoy titles move out
  of the way, under **Titles, More titles**. Nothing is unlocked to work any of this out.
- **Your own pick for a disc.** Choose another cut under **Versions** or **Editions**, or **Play title N as the main
  feature from now on** while a title plays, and **Play main feature** plays that on this disc from then on, until
  **Forget your main feature for this disc**. It is kept in your settings by a fingerprint of the disc's titles, the same
  for the disc, its image and a folder copy; nothing else about the disc is kept.
- On a Blu-ray the status line points to **Disc, Why this playlist**, which is what the menu calls it there, and the
  answer counts what couldn't be read of the whole disc so far, not only of its index.
- The licences that ship now say the Witty disc libraries are under the GNU AGPL 3.0 or later, as they have been since
  the relicence, and `licenses/AGPL-3.0.txt` goes with them. The package check and the installer check look for it.
- A Blu-ray playlist chosen from the Disc menu plays as a title, with chapters and a seek bar, even when none of the
  disc's own titles names it. A Blu-ray whose own program plays nothing starts with the film instead of stopping.
- This needs the disc modules that answer two new calls, `guide` and `choose`: `wittydvdplayer` and `wittyblurayplayer`
  0.3.0, on WittyDVD and WittyBluray 0.4.0. A downloaded disc module from before them still plays discs, without the new menus.
- The release build works on GitHub's Windows machines. Its compiler's `make` crashed there because those machines don't
  name a shell, and now the build names one ([#1](https://github.com/wittywilla/WittyMediaPlayer/issues/1)).
- Buttons that can't be used, such as Play with nothing open, now look dimmed, and the control bar's icons are sharp on
  screens set above 200% ([#6](https://github.com/wittywilla/WittyMediaPlayer/issues/6)).
- Previous and next chapter can only be pressed on a disc. On a file they did nothing
  ([#8](https://github.com/wittywilla/WittyMediaPlayer/issues/8)).
- **A** and **S** say which audio or subtitle track they chose, for a moment over the picture and to a screen reader
  ([#10](https://github.com/wittywilla/WittyMediaPlayer/issues/10)).
- The Subtitles menu says "No subtitles" when there are none, instead of opening empty, and the track menus no longer
  keep a little memory from every time a disc's menu moved ([#12](https://github.com/wittywilla/WittyMediaPlayer/issues/12)).
- The title bar is dark like the rest of the window, even with Windows set to light
  ([#14](https://github.com/wittywilla/WittyMediaPlayer/issues/14)).
- Built on the current disc modules: WittyDVD and WittyBluray 0.3.0, WittyCD and WittyDisc 0.2.0, and the module kit 0.3.0. A downloaded disc module now has to use module API 3, the oldest the kit takes.
- WittyMediaPlayer is now a project of its own, developed and released from this repository.
- **A real Windows installer.** An MSI with a Start menu entry, for just you or for everyone, with an optional desktop
  shortcut, and the file types it should be listed for asked in plain words. A newer one upgrades in place. It has silent
  options for administrators. The zip is still there for a copy that isn't installed.
- **A real uninstaller.** Installed apps now opens `Uninstall WittyMediaPlayer.exe`, which closes the player, asks whether to
  also delete your settings and downloaded modules, removes everything else (program files, shortcuts, registry entries,
  file types, what File Explorer noted about it) and lists what it removed and what it couldn't. It has `/S` and `/RemoveData`
  for scripts. Your media, and anything it didn't make, stay.
- **Clear program names.** `wittymediaplayer.exe` is now `WittyMediaPlayer.exe`, the one to open, and the two helpers moved into
  `bin\`. If you have a shortcut or a script, point it at the new names.
- **Every program has an icon, a description and a version** in its Properties and in Task Manager, and the player has an icon.
- A release now includes the source of FFmpeg and LAME, as the licences page always said it did.
- Looking for your VLC only reads the registry. Before, a player run as an administrator on a computer with no VLC left an
  empty `VideoLAN` key behind.
