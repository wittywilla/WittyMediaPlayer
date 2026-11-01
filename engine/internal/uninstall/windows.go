package uninstall

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const appName = "WittyMediaPlayer"

// PackageList names the file in a program folder that lists the folder's files. The build writes
// it, and it's how a folder is known to be ours: nothing else in a folder is ever removed.
const PackageList = "package-files.txt"

// packageListHeader is its first line.
const packageListHeader = "# WittyMediaPlayer package files"

// ReadPackageList reads a program folder's list of its files, as slash paths relative to it. It
// fails if the folder has no list, or one that names anything outside the folder.
func ReadPackageList(dir string) ([]string, error) {
	f, err := os.Open(filepath.Join(dir, PackageList))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != packageListHeader {
		return nil, errors.New("it isn't a list of WittyMediaPlayer's files")
	}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.Contains(line, `\`) || !filepath.IsLocal(filepath.FromSlash(line)) {
			return nil, fmt.Errorf("it names %q, outside the folder", line)
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("it's empty")
	}
	return out, nil
}

// planProgramFolder adds what's ours in a folder holding the program, if it can vouch for it.
func planProgramFolder(pl *Plan, dir string, admin bool) {
	if dir == "" || !isDir(dir) || !pl.first(dir) {
		return
	}
	files, err := ReadPackageList(dir)
	if err != nil {
		pl.keep(dir, "it doesn't have WittyMediaPlayer's list of its files in it, so it was left alone")
		return
	}
	ours := map[string]bool{}
	var size int64
	for _, f := range files {
		ours[strings.ToLower(f)] = true
		if st, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(f))); err == nil && st.Mode().IsRegular() {
			size += st.Size()
		}
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err == nil && !ours[strings.ToLower(filepath.ToSlash(rel))] {
			pl.keep(p, "it wasn't made by WittyMediaPlayer")
		}
		return nil
	})
	pl.add(Item{Kind: "program files", Name: dir, Group: GroupProgram, Bytes: size, Admin: admin, step: stepFiles,
		remove: func() error { return removeListed(dir, files) },
		gone:   func() bool { return !exists(dir) }})
}

// removeListed removes the files on a package's list, then whichever of their folders are now
// empty, deepest first, and then the folder itself if nothing is left in it.
func removeListed(dir string, files []string) error {
	var failed []string
	folders := map[string]bool{}
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			failed = append(failed, f)
		}
		for d := path.Dir(f); d != "."; d = path.Dir(d) {
			folders[d] = true
		}
	}
	for len(folders) > 0 {
		deepest := ""
		for d := range folders {
			if len(d) > len(deepest) {
				deepest = d
			}
		}
		delete(folders, deepest)
		_ = os.Remove(filepath.Join(dir, filepath.FromSlash(deepest)))
	}
	if !notEmpty(dir) {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d files were in use or locked, the first being %s", len(failed), failed[0])
	}
	return nil
}

func planWindows(e *Env, pl *Plan) {
	tool := func(n string) string { return filepath.Join(e.Getenv("SystemRoot"), "System32", n) }
	programFiles := filepath.Join(e.Getenv("ProgramFiles"), appName)
	localApp, roaming := e.Getenv("LOCALAPPDATA"), e.Getenv("APPDATA")
	perUser := filepath.Join(localApp, "Programs", appName)

	// What runs. Downloaded modules run from the data folder, and a file in use can't be removed.
	modules := filepath.Join(localApp, appName, "modules")
	seenRun := map[string]bool{}
	for _, dir := range append(programFolders(e), modules) {
		key := strings.ToLower(dir)
		if dir == "" || seenRun[key] || !isDir(dir) {
			continue
		}
		seenRun[key] = true
		pl.add(Item{Kind: "anything running from", Name: dir, Group: GroupProgram, step: stepStop,
			remove: func() error { e.Sys.StopProgramsIn(dir); return nil }})
	}

	// The installer takes out what it put in, first: its database has to agree.
	for _, hive := range []string{"HKLM", "HKCU"} {
		if code, ok := e.Sys.RegValue(hive, UninstallKey, "ProductCode"); ok && strings.HasPrefix(code, "{") {
			pl.add(Item{Kind: "Windows Installer product", Name: code, Group: GroupProgram, Admin: hive == "HKLM", step: stepInstaller,
				remove: func() error { return runMSI(e, tool("msiexec.exe"), code) }})
		}
	}

	planRegistry(e, pl)

	// Shortcuts, in this account and for everyone.
	desktop := filepath.Join(e.Getenv("USERPROFILE"), "Desktop")
	shortcuts := []struct {
		path  string
		admin bool
	}{
		{filepath.Join(roaming, "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk"), false},
		{filepath.Join(e.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk"), true},
		{filepath.Join(desktop, appName+".lnk"), false},
		{filepath.Join(e.Getenv("PUBLIC"), "Desktop", appName+".lnk"), true},
	}
	// A desktop can have been moved, to OneDrive for one.
	if d, ok := e.Sys.RegValue("HKCU", shellFoldersKey, "Desktop"); ok && d != "" && !samePath(d, desktop) {
		shortcuts = append(shortcuts, struct {
			path  string
			admin bool
		}{filepath.Join(d, appName+".lnk"), false})
	}
	for _, s := range shortcuts {
		if exists(s.path) {
			pl.add(fileItem("shortcut", s.path, GroupProgram, s.admin))
		}
	}

	// The program, wherever it is. Where the installer says it put it goes first, as that's right
	// about who can remove it.
	if d, ok := e.Sys.RegValue("HKCU", UninstallKey, "InstallLocation"); ok && d != "" {
		d = filepath.Clean(d)
		planProgramFolder(pl, d, samePath(d, programFiles))
	}
	if d, ok := e.Sys.RegValue("HKLM", UninstallKey, "InstallLocation"); ok && d != "" {
		planProgramFolder(pl, filepath.Clean(d), true)
	}
	planProgramFolder(pl, programFiles, true)
	planProgramFolder(pl, perUser, false)
	planProgramFolder(pl, e.SelfRoot, false)

	// Settings and modules.
	planUserFolders(pl, localApp, roaming, false)
	if d := e.Getenv("WMP_DATA"); d != "" {
		planModulesOverride(pl, d)
	}
	if d := filepath.Join(e.Getenv("TEMP"), ScriptSettingsFolder); e.Getenv("TEMP") != "" && isDir(d) {
		pl.add(fileItem("settings from an automated check", d, GroupData, false))
	}

	// Other accounts on this computer used the player too: an administrator can clear those.
	if !e.IsAdmin || e.Getenv("USERPROFILE") == "" {
		return
	}
	users := filepath.Dir(e.Getenv("USERPROFILE"))
	entries, err := os.ReadDir(users)
	if err != nil {
		return
	}
	for _, u := range entries {
		base := filepath.Join(users, u.Name())
		if !u.IsDir() || samePath(base, e.Getenv("USERPROFILE")) {
			continue
		}
		planUserFolders(pl, filepath.Join(base, "AppData", "Local"), filepath.Join(base, "AppData", "Roaming"), true)
		for _, s := range []string{
			filepath.Join(base, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk"),
			filepath.Join(base, "Desktop", appName+".lnk"),
		} {
			if exists(s) {
				pl.add(fileItem("shortcut", s, GroupProgram, true))
			}
		}
	}
}

// How long to wait for another install to finish, which Windows Installer won't run beside.
var (
	msiBusyTries = 24
	msiBusyWait  = 5 * time.Second
)

var exitStatus = regexp.MustCompile(`exit status (\d+)`)

// runMSI removes the product. Its exit code says more than yes or no.
func runMSI(e *Env, msiexec, code string) error {
	for try := 0; ; try++ {
		_, err := e.Sys.Run(msiexec, "/x", code, "/qn", "/norestart")
		if err == nil {
			return nil
		}
		m := exitStatus.FindStringSubmatch(err.Error())
		if m == nil {
			return err
		}
		switch m[1] {
		case "1605", "1614", "3010", "1641":
			// It wasn't installed after all, or it's gone and a restart will finish it.
			return nil
		case "1618":
			if try < msiBusyTries {
				time.Sleep(msiBusyWait)
				continue
			}
			return errors.New("another installation was running, and didn't finish in time")
		case "1602":
			return errors.New("that was cancelled")
		}
		return fmt.Errorf("Windows Installer stopped with code %s", m[1])
	}
}
