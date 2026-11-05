# Changelog

What changed in WittyMediaPlayer, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

Version 0.1.3. Version 0.1.2 was tagged but never published, because its build failed on GitHub, so everything below
arrives with 0.1.3.

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
