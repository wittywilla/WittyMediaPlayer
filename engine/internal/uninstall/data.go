package uninstall

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SettingsEntries is everything the player creates in %APPDATA%\WittyMediaPlayer, which is where
// Qt keeps an INI file for QSettings(IniFormat, UserScope, "WittyMediaPlayer", "WittyMediaPlayer").
// Only what's on these lists is removed. A test reads the source and fails if the player starts
// writing somewhere new.
var SettingsEntries = map[string]string{
	"WittyMediaPlayer.ini":      "settings",
	"WittyMediaPlayer.ini.lock": "settings",
}

// settingsTemp is the copy Qt writes before putting a new settings file in place.
var settingsTemp = regexp.MustCompile(`(?i)^WittyMediaPlayer\.ini\.[A-Za-z0-9]{6}$`)

// LocalEntries is everything the player creates in %LOCALAPPDATA%\WittyMediaPlayer. See
// modhost.DefaultDir.
var LocalEntries = map[string]string{
	"modules": "disc modules it downloaded",
}

// ModuleIDs are the modules the player can download. A folder of modules the player was told to
// use by WMP_DATA is only taken for ours if it holds nothing else.
var ModuleIDs = map[string]bool{"wittydvd": true, "wittybluray": true}

// ScriptSettingsFolder is where an automated check (--script) keeps its settings, in the temp
// folder. See main.cpp.
const ScriptSettingsFolder = "wittymediaplayer-script-settings"

// planUserFolders adds what's ours in one account's folders.
func planUserFolders(pl *Plan, localApp, roaming string, admin bool) {
	planKnown(pl, filepath.Join(roaming, appName), SettingsEntries, settingsTemp, admin)
	planKnown(pl, filepath.Join(localApp, appName), LocalEntries, nil, admin)
}

// planKnown adds the known names in a folder of ours, and reports anything else in it as left.
func planKnown(pl *Plan, dir string, known map[string]string, also *regexp.Regexp, admin bool) {
	entries, err := os.ReadDir(dir)
	if err != nil || !pl.first(dir) {
		return
	}
	for _, ent := range entries {
		name, p := ent.Name(), filepath.Join(dir, ent.Name())
		what, ours := lookup(known, name)
		if !ours && also != nil && also.MatchString(name) {
			what, ours = "settings", true
		}
		if !ours {
			pl.keep(p, "it wasn't made by WittyMediaPlayer")
			continue
		}
		pl.add(fileItem(what, p, GroupData, admin))
	}
	pl.add(emptyFolderItem(dir, GroupData, admin))
}

func lookup(known map[string]string, name string) (string, bool) {
	for k, v := range known {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// planModulesOverride is for a player that was told to keep its modules elsewhere by WMP_DATA. The
// folder itself was someone's choice, so it stays.
func planModulesOverride(pl *Plan, dataDir string) {
	dir := filepath.Join(dataDir, "modules")
	entries, err := os.ReadDir(dir)
	if err != nil || !pl.first(dir) {
		return
	}
	for _, ent := range entries {
		if !ModuleIDs[strings.ToLower(ent.Name())] {
			pl.keep(dir, "it has more in it than WittyMediaPlayer's modules")
			return
		}
	}
	pl.add(fileItem("disc modules it downloaded", dir, GroupData, false))
}
