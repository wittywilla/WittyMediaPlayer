# Changelog

What changed in WittyMediaPlayer, newest first. What changed before it had a repository of its own is in the
[WittyMediaServer changelog](https://github.com/wittywilla/WittyMediaServer/blob/master/CHANGELOG.md).

## Unreleased

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
