package uninstall

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// The registry, as packaging/windows/WittyMediaPlayer.wxs writes it. Everything there is removed
// by Windows Installer too, but this must find it all if that failed or ever went missing: a test
// fills a fake registry from the .wxs and checks nothing is left.
const (
	// UninstallKey is the entry in Installed apps, with ProductCode in it.
	UninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\WittyMediaPlayer`
	appPathsKey  = `Software\Microsoft\Windows\CurrentVersion\App Paths\WittyMediaPlayer.exe`
	publisherKey = `Software\WittyWilla`
	productKey   = publisherKey + `\WittyMediaPlayer`
	// qtNativeKey is where QSettings keeps settings in the registry. The player uses an INI file
	// and doesn't write here, but it's our name, and a key can't be left by accident this way.
	qtNativeKey     = `Software\WittyMediaPlayer`
	registeredApps  = `Software\RegisteredApplications`
	classesKey      = `Software\Classes`
	applicationsKey = classesKey + `\Applications\WittyMediaPlayer.exe`
	shellFoldersKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders`
	appPathsParent  = `Software\Microsoft\Windows\CurrentVersion\App Paths`
	uninstallParent = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	applications    = classesKey + `\Applications`
	// What Windows itself notes about programs, for this account only.
	fileExtsKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts`
	toastsKey   = `Software\Microsoft\Windows\CurrentVersion\ApplicationAssociationToasts`
	muiCacheKey = `Software\Classes\Local Settings\Software\Microsoft\Windows\Shell\MuiCache`
	exeName     = "WittyMediaPlayer.exe"
)

// FileTypes is what the installer can offer to open, by the type it registers. Each must be
// something the player's File, Open takes, and a test holds this list, the .wxs and mainwindow.cpp
// together. .bin is left out on purpose: it's a CD image only sometimes, and a name many programs
// use for many things.
var FileTypes = map[string][]string{
	"WittyMediaPlayer.Video": {".mkv", ".mp4", ".m4v", ".mov", ".avi", ".webm", ".ts", ".m2ts", ".mts", ".mpg", ".mpeg", ".vob", ".wmv", ".flv", ".ogv", ".3gp"},
	"WittyMediaPlayer.Music": {".mp3", ".m4a", ".aac", ".flac", ".wav", ".ogg", ".oga", ".opus", ".wma", ".ac3", ".dts"},
	"WittyMediaPlayer.Disc":  {".iso", ".img", ".cue", ".toc", ".ccd", ".nrg", ".chd"},
}

// allExts is every type in one list.
func allExts() []string {
	var out []string
	for _, l := range FileTypes {
		out = append(out, l...)
	}
	return out
}

func planRegistry(e *Env, pl *Plan) {
	for _, hive := range []string{"HKCU", "HKLM"} {
		admin := hive == "HKLM"
		for _, k := range []struct{ kind, key string }{
			{"Installed apps entry", UninstallKey},
			{"the program's own registry key", productKey},
			{"Run box entry", appPathsKey},
			{"Open with entry", applicationsKey},
		} {
			pl.addKey(e, k.kind, hive, k.key, GroupProgram, admin)
		}
		for progID := range FileTypes {
			pl.addKey(e, "file type", hive, classesKey+`\`+progID, GroupProgram, admin)
		}
		pl.addKey(e, "settings", hive, qtNativeKey, GroupData, admin)
		if _, ok := e.Sys.RegValue(hive, registeredApps, appName); ok {
			pl.add(Item{Kind: "Default apps entry", Name: hive + `\` + registeredApps + `\` + appName, Group: GroupProgram, Admin: admin, step: stepFiles,
				remove: func() error { return e.Sys.RegDeleteValue(hive, registeredApps, appName) },
				gone:   func() bool { _, ok := e.Sys.RegValue(hive, registeredApps, appName); return !ok }})
		}
		if hasOpenWith(e, hive) {
			pl.add(Item{Kind: "Open with file types", Name: hive + `\` + classesKey + `\<type>\OpenWithProgids`, Group: GroupProgram, Admin: admin, step: stepFiles,
				remove: func() error { return removeOpenWith(e, hive) }})
		}
		// Ours went, so if nothing is left under the publisher's key, that goes as well. In this
		// account's registry the keys that held ours may only have been there for that. The
		// computer's have always been there, for everything else on it.
		if e.Sys.RegKeyExists(hive, productKey) || e.Sys.RegKeyExists(hive, UninstallKey) {
			keys := []string{publisherKey}
			if hive == "HKCU" {
				keys = append(keys, registeredApps, applications, appPathsParent, uninstallParent)
			}
			for _, key := range keys {
				pl.add(Item{Kind: "registry key", Name: hive + `\` + key, Group: GroupProgram, Admin: admin, step: stepEmptyFolders,
					remove: func() error { return pruneKey(e, hive, key) }})
			}
		}
	}
	notes := findWindowsNotes(e, programFolders(e))
	if len(notes) > 0 {
		pl.add(Item{Kind: "what Windows noted about it", Name: fmt.Sprintf("%d registry entries in HKCU", len(notes)), Group: GroupProgram, step: stepFiles,
			remove: func() error {
				var first error
				for _, n := range notes {
					if err := n(); err != nil && first == nil {
						first = err
					}
				}
				return first
			}})
	}
}

// addKey adds a key that's entirely ours, if it's there.
func (pl *Plan) addKey(e *Env, kind, hive, key string, g Group, admin bool) {
	if !e.Sys.RegKeyExists(hive, key) {
		return
	}
	pl.add(Item{Kind: kind, Name: hive + `\` + key, Group: g, Admin: admin, step: stepFiles,
		remove: func() error { return e.Sys.RegDeleteTree(hive, key) },
		gone:   func() bool { return !e.Sys.RegKeyExists(hive, key) }})
}

func hasOpenWith(e *Env, hive string) bool {
	for progID, exts := range FileTypes {
		for _, ext := range exts {
			if _, ok := e.Sys.RegValue(hive, classesKey+`\`+ext+`\OpenWithProgids`, progID); ok {
				return true
			}
		}
	}
	return false
}

// removeOpenWith takes this program out of each file type's Open with list. Other programs in
// the list stay, and so does the file type, unless taking this out left it with nothing at all.
func removeOpenWith(e *Env, hive string) error {
	var errs []error
	for progID, exts := range FileTypes {
		for _, ext := range exts {
			key := classesKey + `\` + ext + `\OpenWithProgids`
			if err := e.Sys.RegDeleteValue(hive, key, progID); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := pruneKey(e, hive, key); err != nil {
				errs = append(errs, err)
			}
			if err := pruneKey(e, hive, classesKey+`\`+ext); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// pruneKey removes a key that has nothing in it. RegValueNames counts a default value too. Only
// what's in the plan's own list of names is ever asked about.
func pruneKey(e *Env, hive, key string) error {
	if !e.Sys.RegKeyExists(hive, key) || len(e.Sys.RegValueNames(hive, key)) > 0 || len(e.Sys.RegSubkeys(hive, key)) > 0 {
		return nil
	}
	return e.Sys.RegDeleteTree(hive, key)
}

// programFolders is everywhere the program may be: the usual two, this copy, and where the
// installer says it put it.
func programFolders(e *Env) []string {
	var out []string
	for _, d := range []string{filepath.Join(e.Getenv("ProgramFiles"), appName), filepath.Join(e.Getenv("LOCALAPPDATA"), "Programs", appName), e.SelfRoot} {
		if d != "" {
			out = append(out, d)
		}
	}
	for _, hive := range []string{"HKCU", "HKLM"} {
		if d, ok := e.Sys.RegValue(hive, UninstallKey, "InstallLocation"); ok && d != "" {
			out = append(out, filepath.Clean(d))
		}
	}
	return out
}

// ours says a progid, as Windows writes it, is this program's.
func ours(progID string) bool {
	if _, ok := FileTypes[progID]; ok {
		return true
	}
	return strings.EqualFold(progID, `Applications\`+exeName)
}

// findWindowsNotes finds what File Explorer wrote about this program once people used it: its
// place in Open with lists, a default someone chose for it, the "new app" notices and the cached
// name of its file. Each is a removal to do. Windows' own history of what was opened isn't ours.
func findWindowsNotes(e *Env, folders []string) []func() error {
	const hive = "HKCU"
	var out []func() error
	delValue := func(key, name string) {
		out = append(out, func() error { return e.Sys.RegDeleteValue(hive, key, name) })
	}
	for _, ext := range allExts() {
		base := fileExtsKey + `\` + ext
		for _, name := range e.Sys.RegValueNames(hive, base+`\OpenWithProgids`) {
			if ours(name) {
				delValue(base+`\OpenWithProgids`, name)
			}
		}
		// A default that points at a program that's gone only makes Windows ask, but it's ours.
		if id, ok := e.Sys.RegValue(hive, base+`\UserChoice`, "ProgId"); ok && ours(id) {
			key := base + `\UserChoice`
			out = append(out, func() error { return e.Sys.RegDeleteTree(hive, key) })
		}
		listKey := base + `\OpenWithList`
		for _, letter := range e.Sys.RegValueNames(hive, listKey) {
			if v, _ := e.Sys.RegValue(hive, listKey, letter); strings.EqualFold(v, exeName) {
				l := letter
				out = append(out, func() error { return removeFromMRU(e, hive, listKey, l) })
			}
		}
	}
	for _, name := range e.Sys.RegValueNames(hive, toastsKey) {
		id, _, found := strings.Cut(name, "_")
		if found && ours(id) {
			delValue(toastsKey, name)
		}
	}
	for _, name := range e.Sys.RegValueNames(hive, muiCacheKey) {
		for _, d := range folders {
			if strings.HasPrefix(strings.ToLower(name), strings.ToLower(d)+`\`) {
				delValue(muiCacheKey, name)
				break
			}
		}
	}
	return out
}

// removeFromMRU takes one lettered entry out of an Open with list, and out of the value that
// says what order the letters go in.
func removeFromMRU(e *Env, hive, key, letter string) error {
	if err := e.Sys.RegDeleteValue(hive, key, letter); err != nil {
		return err
	}
	order, ok := e.Sys.RegValue(hive, key, "MRUList")
	if !ok {
		return nil
	}
	return e.Sys.RegSetValue(hive, key, "MRUList", strings.ReplaceAll(order, letter, ""))
}
