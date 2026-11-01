package uninstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeSys answers like a machine and writes down what it's asked. A program name it has an answer
// for exists; everything else fails, as a missing program does.
type fakeSys struct {
	answers map[string]func() error
	calls   []string
	stopped []string
	keys    map[string]bool              // HIVE\key
	values  map[string]map[string]string // HIVE\key -> name -> value
}

func newFake() *fakeSys {
	return &fakeSys{answers: map[string]func() error{}, keys: map[string]bool{}, values: map[string]map[string]string{}}
}

func (f *fakeSys) Run(name string, args ...string) (string, error) {
	call := filepath.Base(name) + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	for k, v := range f.answers {
		if strings.HasPrefix(call, k) {
			return "", v()
		}
	}
	return "", errors.New("exit status 1")
}
func (f *fakeSys) StopProgramsIn(dir string) { f.stopped = append(f.stopped, dir) }

// setKey makes a key and every key above it, as the registry has them.
func (f *fakeSys) setKey(h, k string) {
	parts := strings.Split(k, `\`)
	for i := range parts {
		f.keys[h+`\`+strings.Join(parts[:i+1], `\`)] = true
	}
}
func (f *fakeSys) set(h, k, n, v string) {
	f.setKey(h, k)
	if f.values[h+`\`+k] == nil {
		f.values[h+`\`+k] = map[string]string{}
	}
	f.values[h+`\`+k][n] = v
}
func (f *fakeSys) RegKeyExists(h, k string) bool { return f.keys[h+`\`+k] }
func (f *fakeSys) RegValue(h, k, n string) (string, bool) {
	v, ok := f.values[h+`\`+k][n]
	return v, ok
}
func (f *fakeSys) RegValueNames(h, k string) []string {
	var out []string
	for n := range f.values[h+`\`+k] {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
func (f *fakeSys) RegSubkeys(h, k string) []string {
	var out []string
	prefix := h + `\` + k + `\`
	for key := range f.keys {
		if strings.HasPrefix(key, prefix) && !strings.Contains(key[len(prefix):], `\`) {
			out = append(out, key[len(prefix):])
		}
	}
	return out
}
func (f *fakeSys) RegSetValue(h, k, n, v string) error { f.set(h, k, n, v); return nil }
func (f *fakeSys) RegDeleteTree(h, k string) error {
	full := h + `\` + k
	for key := range f.keys {
		if key == full || strings.HasPrefix(key, full+`\`) {
			delete(f.keys, key)
			delete(f.values, key)
		}
	}
	return nil
}
func (f *fakeSys) RegDeleteValue(h, k, n string) error {
	delete(f.values[h+`\`+k], n)
	return nil
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gone(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		t.Errorf("%s is still there", p)
	}
}

func there(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); err != nil {
		t.Errorf("%s was taken", p)
	}
}

type machine struct {
	root                                  string
	programFiles, perUser, local, roaming string
	home                                  string
	vars                                  map[string]string
	sys                                   *fakeSys
	env                                   *Env
}

func newMachine(t *testing.T) *machine {
	t.Helper()
	root := t.TempDir()
	m := &machine{root: root, sys: newFake()}
	m.home = filepath.Join(root, "Users", "me")
	m.vars = map[string]string{
		"SystemRoot": filepath.Join(root, "Windows"), "ProgramFiles": filepath.Join(root, "Program Files"), "ProgramData": filepath.Join(root, "ProgramData"),
		"LOCALAPPDATA": filepath.Join(m.home, "AppData", "Local"), "APPDATA": filepath.Join(m.home, "AppData", "Roaming"),
		"USERPROFILE": m.home, "PUBLIC": filepath.Join(root, "Users", "Public"), "TEMP": filepath.Join(m.home, "Temp"),
	}
	m.programFiles = filepath.Join(m.vars["ProgramFiles"], appName)
	m.perUser = filepath.Join(m.vars["LOCALAPPDATA"], "Programs", appName)
	m.local, m.roaming = filepath.Join(m.vars["LOCALAPPDATA"], appName), filepath.Join(m.vars["APPDATA"], appName)
	m.env = &Env{Getenv: func(k string) string { return m.vars[k] }, Sys: m.sys}
	return m
}

var packageFiles = []string{"WittyMediaPlayer.exe", "Uninstall WittyMediaPlayer.exe", PackageList, "bin/wittymediaplayer-engine.exe", "bin/wittymediaplayer-updater.exe", "Qt6Core.dll", "avcodec-62.dll", "platforms/qwindows.dll", "licenses/LGPL-3.0.txt", "LICENSE", "NOTICE", "README.md"}

// fillPackage writes a program folder as the build makes it.
func fillPackage(t *testing.T, dir string) {
	t.Helper()
	for _, f := range packageFiles {
		write(t, filepath.Join(dir, filepath.FromSlash(f)), "x")
	}
	write(t, filepath.Join(dir, PackageList), packageListHeader+"\n"+strings.Join(packageFiles, "\n")+"\n")
}

// fillData writes what a player that has been used leaves.
func (m *machine) fillData(t *testing.T) {
	t.Helper()
	write(t, filepath.Join(m.roaming, "WittyMediaPlayer.ini"), "[General]\nvolume=1\n")
	write(t, filepath.Join(m.local, "modules", "wittydvd", "state.json"), "{}")
	write(t, filepath.Join(m.local, "modules", "wittydvd", "versions", "0.2.0", "helper.exe"), "x")
	write(t, filepath.Join(m.vars["TEMP"], ScriptSettingsFolder, appName, appName+".ini"), "x")
}

// fillRegistry writes what the installer writes, with the file types it was asked for.
func (m *machine) fillRegistry(h string) {
	s := m.sys
	s.set(h, UninstallKey, "ProductCode", "{11111111-2222-3333-4444-555555555555}")
	s.set(h, UninstallKey, "DisplayName", appName)
	s.set(h, productKey, "ChoseDesktop", "1-")
	s.set(h, productKey+`\Capabilities\FileAssociations`, ".mkv", "WittyMediaPlayer.Video")
	s.set(h, registeredApps, appName, productKey+`\Capabilities`)
	s.set(h, appPathsKey, "", "x")
	s.set(h, applicationsKey+`\shell\open\command`, "", "x")
	s.set(h, classesKey+`\WittyMediaPlayer.Video\shell\open\command`, "", "x")
	s.set(h, classesKey+`\.mkv\OpenWithProgids`, "WittyMediaPlayer.Video", "")
	s.set(h, classesKey+`\.flac\OpenWithProgids`, "WittyMediaPlayer.Music", "")
	s.set(h, classesKey+`\.flac\OpenWithProgids`, "SomePlayer.flac", "")
}

// msiRemoves makes msiexec succeed, doing what a real one does.
func (m *machine) msiRemoves(t *testing.T, h, dir string) {
	t.Helper()
	m.sys.answers["msiexec.exe /x"] = func() error {
		for _, f := range packageFiles {
			_ = os.Remove(filepath.Join(dir, filepath.FromSlash(f)))
		}
		_ = m.sys.RegDeleteTree(h, UninstallKey)
		return nil
	}
}

func TestFullRemovalPerUser(t *testing.T) {
	m := newMachine(t)
	fillPackage(t, m.perUser)
	m.fillData(t)
	m.fillRegistry("HKCU")
	m.msiRemoves(t, "HKCU", m.perUser)
	film := filepath.Join(m.home, "Videos", "A Made-Up Film.mkv")
	copied := filepath.Join(m.home, "Music", "A Made-Up Album", "01.flac")
	notes := filepath.Join(m.local, "my-notes.txt")
	mine := filepath.Join(m.perUser, "my-own-file.txt")
	write(t, film, "x")
	write(t, copied, "x")
	write(t, notes, "x")
	write(t, mine, "x")
	start := filepath.Join(m.vars["APPDATA"], "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk")
	desk := filepath.Join(m.home, "Desktop", appName+".lnk")
	write(t, start, "x")
	write(t, desk, "x")

	pl := MakePlan(m.env)
	if pl.NeedsAdmin(Options{RemoveData: true}) {
		t.Error("a per-user install shouldn't need an administrator")
	}
	r := Execute(pl, Options{RemoveData: true})
	if !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	gone(t, filepath.Join(m.perUser, "WittyMediaPlayer.exe"))
	gone(t, filepath.Join(m.perUser, "bin"))
	gone(t, filepath.Join(m.perUser, "platforms"))
	gone(t, filepath.Join(m.roaming))
	gone(t, filepath.Join(m.local, "modules"))
	gone(t, filepath.Join(m.vars["TEMP"], ScriptSettingsFolder))
	gone(t, start)
	gone(t, desk)
	there(t, film)
	there(t, copied)
	there(t, notes)
	there(t, mine)
	for _, k := range []string{UninstallKey, productKey, publisherKey, appPathsKey, applicationsKey, classesKey + `\WittyMediaPlayer.Video`, classesKey + `\.mkv`} {
		if m.sys.RegKeyExists("HKCU", k) {
			t.Errorf("%s is still in the registry", k)
		}
	}
	if _, ok := m.sys.RegValue("HKCU", registeredApps, appName); ok {
		t.Error("it's still in Default apps")
	}
	// The next program's place in the list stays, and so does the type.
	if _, ok := m.sys.RegValue("HKCU", classesKey+`\.flac\OpenWithProgids`, "SomePlayer.flac"); !ok {
		t.Error("another program's Open with entry was taken")
	}
	if _, ok := m.sys.RegValue("HKCU", classesKey+`\.flac\OpenWithProgids`, "WittyMediaPlayer.Music"); ok {
		t.Error("its own Open with entry stayed")
	}
	report := r.Report()
	for _, want := range []string{mine, notes} {
		if !strings.Contains(report, want) {
			t.Errorf("the report doesn't say %s was left:\n%s", want, report)
		}
	}
	if !strings.Contains(strings.Join(m.sys.stopped, "\n"), m.perUser) {
		t.Error("it didn't close what was running")
	}
}

func TestKeepingDataKeepsIt(t *testing.T) {
	m := newMachine(t)
	fillPackage(t, m.perUser)
	m.fillData(t)
	m.fillRegistry("HKCU")
	m.msiRemoves(t, "HKCU", m.perUser)
	r := Execute(MakePlan(m.env), Options{})
	if !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	gone(t, m.perUser)
	there(t, filepath.Join(m.roaming, "WittyMediaPlayer.ini"))
	there(t, filepath.Join(m.local, "modules", "wittydvd", "state.json"))
	if !strings.Contains(r.Report(), "you chose to keep it") {
		t.Errorf("it doesn't say what it kept:\n%s", r.Report())
	}
}

func TestEveryoneNeedsAnAdministrator(t *testing.T) {
	m := newMachine(t)
	fillPackage(t, m.programFiles)
	m.fillRegistry("HKLM")
	m.msiRemoves(t, "HKLM", m.programFiles)
	pl := MakePlan(m.env)
	if !pl.NeedsAdmin(Options{}) {
		t.Fatal("a for-everyone install should need an administrator")
	}
	r := Execute(pl, Options{RemoveData: true})
	if !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	gone(t, m.programFiles)
	if m.sys.RegKeyExists("HKLM", UninstallKey) || m.sys.RegKeyExists("HKLM", productKey) {
		t.Error("registry keys stayed")
	}
}

func TestWhereTheInstallerSaidItPutIt(t *testing.T) {
	m := newMachine(t)
	dir := filepath.Join(m.root, "Apps", appName)
	fillPackage(t, dir)
	m.fillRegistry("HKLM")
	m.sys.set("HKLM", UninstallKey, "InstallLocation", dir+string(filepath.Separator))
	m.sys.answers["msiexec.exe /x"] = func() error { return errors.New("exit status 1603") }
	pl := MakePlan(m.env)
	if !pl.NeedsAdmin(Options{}) {
		t.Error("it's for everyone, so it needs an administrator")
	}
	r := Execute(pl, Options{})
	if !strings.Contains(strings.Join(m.sys.stopped, "\n"), dir) {
		t.Error("it didn't close what was running there")
	}
	if _, err := os.Lstat(filepath.Join(dir, "WittyMediaPlayer.exe")); !os.IsNotExist(err) {
		t.Errorf("it stayed, after Windows Installer failed:\n%s", r.Report())
	}
}

func TestPortableCopyGoesWithoutAnInstaller(t *testing.T) {
	m := newMachine(t)
	dir := filepath.Join(m.root, "Tools", "WittyMediaPlayer-0.1.1-windows-amd64")
	fillPackage(t, dir)
	m.fillData(t)
	m.env.SelfRoot = dir
	r := Execute(MakePlan(m.env), Options{RemoveData: true})
	if !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	gone(t, dir)
	if len(m.sys.calls) != 0 {
		t.Errorf("it ran %v for a copy no installer put there", m.sys.calls)
	}
}

func TestAFolderWithoutItsListIsLeftAlone(t *testing.T) {
	m := newMachine(t)
	for _, f := range []string{"WittyMediaPlayer.exe", "bin/wittymediaplayer-engine.exe", "photos/me.jpg"} {
		write(t, filepath.Join(m.perUser, filepath.FromSlash(f)), "x")
	}
	r := Execute(MakePlan(m.env), Options{RemoveData: true})
	there(t, filepath.Join(m.perUser, "photos", "me.jpg"))
	there(t, filepath.Join(m.perUser, "WittyMediaPlayer.exe"))
	if !strings.Contains(r.Report(), "left alone") {
		t.Errorf("it doesn't say so:\n%s", r.Report())
	}
}

func TestAListCantReachOutOfItsFolder(t *testing.T) {
	m := newMachine(t)
	precious := filepath.Join(m.home, "Documents", "precious.txt")
	write(t, precious, "x")
	for _, bad := range []string{`../../../Documents/precious.txt`, `..\..\..\Documents\precious.txt`, precious} {
		write(t, filepath.Join(m.perUser, PackageList), packageListHeader+"\n"+bad+"\n")
		if _, err := ReadPackageList(m.perUser); err == nil {
			t.Errorf("a list naming %s was accepted", bad)
		}
		Execute(MakePlan(m.env), Options{RemoveData: true})
		there(t, precious)
	}
	write(t, filepath.Join(m.perUser, PackageList), "no header\nWittyMediaPlayer.exe\n")
	if _, err := ReadPackageList(m.perUser); err == nil {
		t.Error("a list with no header was accepted")
	}
}

func TestOtherAccountsNeedAnAdministrator(t *testing.T) {
	m := newMachine(t)
	other := filepath.Join(m.root, "Users", "them")
	ini := filepath.Join(other, "AppData", "Roaming", appName, "WittyMediaPlayer.ini")
	mods := filepath.Join(other, "AppData", "Local", appName, "modules", "wittydvd", "state.json")
	film := filepath.Join(other, "Videos", "A Made-Up Film.mkv")
	lnk := filepath.Join(other, "Desktop", appName+".lnk")
	write(t, ini, "x")
	write(t, mods, "x")
	write(t, film, "x")
	write(t, lnk, "x")

	Execute(MakePlan(m.env), Options{RemoveData: true})
	there(t, ini)

	m.env.IsAdmin = true
	if r := Execute(MakePlan(m.env), Options{RemoveData: true}); !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	gone(t, ini)
	gone(t, mods)
	gone(t, lnk)
	there(t, film)
}

func TestWindowsInstallerAnswers(t *testing.T) {
	old := msiBusyWait
	msiBusyWait = 0
	defer func() { msiBusyWait = old }()
	for _, c := range []struct {
		code    string
		wantErr bool
	}{{"1605", false}, {"3010", false}, {"1641", false}, {"1603", true}, {"1602", true}} {
		m := newMachine(t)
		m.fillRegistry("HKCU")
		m.sys.answers["msiexec.exe /x"] = func() error { return errors.New("exit status " + c.code) }
		r := Execute(MakePlan(m.env), Options{})
		if failed := !r.OK(); failed != c.wantErr {
			t.Errorf("code %s: failed=%v\n%s", c.code, failed, r.Report())
		}
	}
	// Busy with another install: it waits for that.
	m := newMachine(t)
	m.fillRegistry("HKCU")
	tries := 0
	m.sys.answers["msiexec.exe /x"] = func() error {
		tries++
		if tries < 3 {
			return errors.New("exit status 1618")
		}
		return nil
	}
	if r := Execute(MakePlan(m.env), Options{}); !r.OK() || tries != 3 {
		t.Errorf("tries=%d\n%s", tries, r.Report())
	}
}

// What File Explorer writes about a program once it's used.
func TestWindowsOwnNotesAreCleared(t *testing.T) {
	m := newMachine(t)
	fillPackage(t, m.perUser)
	s := m.sys
	ext := fileExtsKey + `\.mkv`
	s.set("HKCU", ext+`\OpenWithProgids`, "WittyMediaPlayer.Video", "")
	s.set("HKCU", ext+`\OpenWithProgids`, "SomePlayer.mkv", "")
	s.set("HKCU", ext+`\UserChoice`, "ProgId", "WittyMediaPlayer.Video")
	s.set("HKCU", fileExtsKey+`\.mp4\UserChoice`, "ProgId", "SomePlayer.mp4")
	s.set("HKCU", fileExtsKey+`\.iso\UserChoice`, "ProgId", `Applications\WittyMediaPlayer.exe`)
	s.set("HKCU", ext+`\OpenWithList`, "a", "WittyMediaPlayer.exe")
	s.set("HKCU", ext+`\OpenWithList`, "b", "someplayer.exe")
	s.set("HKCU", ext+`\OpenWithList`, "MRUList", "ab")
	s.set("HKCU", toastsKey, "WittyMediaPlayer.Video_.mkv", "0")
	s.set("HKCU", toastsKey, "SomePlayer.mkv_.mkv", "0")
	s.set("HKCU", muiCacheKey, m.perUser+`\WittyMediaPlayer.exe.FriendlyAppName`, "WittyMediaPlayer")
	s.set("HKCU", muiCacheKey, m.root+`\Other\WittyMediaPlayer.exe.FriendlyAppName`, "Somebody else's")

	if r := Execute(MakePlan(m.env), Options{}); !r.OK() {
		t.Fatalf("it failed: %s", r.Report())
	}
	has := func(key, name string) bool { _, ok := s.RegValue("HKCU", key, name); return ok }
	for _, c := range []struct {
		key, name string
		want      bool
	}{
		{ext + `\OpenWithProgids`, "WittyMediaPlayer.Video", false},
		{ext + `\OpenWithProgids`, "SomePlayer.mkv", true},
		{ext + `\UserChoice`, "ProgId", false},
		{fileExtsKey + `\.mp4\UserChoice`, "ProgId", true},
		{fileExtsKey + `\.iso\UserChoice`, "ProgId", false},
		{ext + `\OpenWithList`, "a", false},
		{ext + `\OpenWithList`, "b", true},
		{toastsKey, "WittyMediaPlayer.Video_.mkv", false},
		{toastsKey, "SomePlayer.mkv_.mkv", true},
		{muiCacheKey, m.perUser + `\WittyMediaPlayer.exe.FriendlyAppName`, false},
		{muiCacheKey, m.root + `\Other\WittyMediaPlayer.exe.FriendlyAppName`, true},
	} {
		if got := has(c.key, c.name); got != c.want {
			t.Errorf("%s\\%s: there=%v, want %v", c.key, c.name, got, c.want)
		}
	}
	if v, _ := s.RegValue("HKCU", ext+`\OpenWithList`, "MRUList"); v != "b" {
		t.Errorf("the order of the list is %q", v)
	}
}

func TestAMovedDesktopIsFound(t *testing.T) {
	m := newMachine(t)
	moved := filepath.Join(m.home, "OneDrive", "Desktop")
	m.sys.set("HKCU", shellFoldersKey, "Desktop", moved)
	lnk := filepath.Join(moved, appName+".lnk")
	write(t, lnk, "x")
	if r := Execute(MakePlan(m.env), Options{}); !r.OK() {
		t.Fatal(r.Report())
	}
	gone(t, lnk)
}

func TestModulesFolderChosenByTheEnvironment(t *testing.T) {
	m := newMachine(t)
	ours := filepath.Join(m.root, "Mine")
	write(t, filepath.Join(ours, "modules", "wittybluray", "state.json"), "x")
	write(t, filepath.Join(ours, "keep.txt"), "x")
	m.vars["WMP_DATA"] = ours
	Execute(MakePlan(m.env), Options{RemoveData: true})
	gone(t, filepath.Join(ours, "modules"))
	there(t, filepath.Join(ours, "keep.txt"))

	notOurs := filepath.Join(m.root, "Theirs")
	write(t, filepath.Join(notOurs, "modules", "somethingelse", "x"), "x")
	m.vars["WMP_DATA"] = notOurs
	Execute(MakePlan(m.env), Options{RemoveData: true})
	there(t, filepath.Join(notOurs, "modules", "somethingelse", "x"))
}

func TestAFailureDoesntStopTheRest(t *testing.T) {
	m := newMachine(t)
	fillPackage(t, m.perUser)
	m.fillData(t)
	m.fillRegistry("HKCU")
	m.sys.answers["msiexec.exe /x"] = func() error { return errors.New("exit status 1603") }
	r := Execute(MakePlan(m.env), Options{RemoveData: true})
	if r.OK() {
		t.Fatal("it should say something failed")
	}
	if !strings.Contains(r.Report(), "Couldn't be removed") || !strings.Contains(r.Report(), "code 1603") {
		t.Errorf("the report doesn't say what failed:\n%s", r.Report())
	}
	gone(t, filepath.Join(m.perUser, "WittyMediaPlayer.exe"))
	gone(t, filepath.Join(m.roaming, "WittyMediaPlayer.ini"))
}

func TestFlags(t *testing.T) {
	for _, c := range []struct {
		argv []string
		yes  bool
		data bool
	}{
		{[]string{"/S"}, true, false},
		{[]string{"/S", "/RemoveData"}, true, true},
		{[]string{"--uninstall", "--yes", "--remove-data"}, true, true},
		{[]string{"-s", "/removedata"}, true, true},
		{nil, false, false},
	} {
		a, err := ParseArgs(c.argv)
		if err != nil || a.Yes != c.yes || a.RemoveData != c.data {
			t.Errorf("%v: %+v, %v", c.argv, a, err)
		}
	}
	for _, bad := range [][]string{{"/RemoveData", "/KeepData"}, {"--nonsense"}, {"--report"}, {"--parent", "x"}} {
		if _, err := ParseArgs(bad); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	a, err := ParseArgs([]string{"/S", "/RemoveData", "--report", `C:\a b.txt`, "--program-folder", `C:\p q`, "--user-sid", "S-1-5", "--parent", "7", "--unattended"})
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseArgs(a.Flags())
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(back.RemoveData, back.Report, back.ProgramFolder, back.UserSID, back.Parent, back.Unattended) != fmt.Sprint(true, `C:\a b.txt`, `C:\p q`, "S-1-5", 7, true) {
		t.Errorf("the worker's flags lost something: %+v", back)
	}
}

func TestNothingFound(t *testing.T) {
	m := newMachine(t)
	if pl := MakePlan(m.env); len(pl.Items) != 0 {
		t.Errorf("found %v on an empty machine", pl.Items)
	}
	if !strings.Contains(DryRun(MakePlan(m.env)), "Nothing of WittyMediaPlayer") {
		t.Error("a dry run doesn't say so")
	}
}
