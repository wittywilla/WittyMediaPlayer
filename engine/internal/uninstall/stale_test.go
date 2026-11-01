package uninstall

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// These tests fail when the player starts writing somewhere the uninstaller doesn't know, or the
// installer writes something it won't clean up. If one fails, add the new place to data.go or
// registry.go, and say so in the question the uninstaller asks.

// repo is the repository's top folder.
func repo(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p, "CMakeLists.txt")); err != nil {
		t.Skip("the repository isn't here")
	}
	return p
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sources walks a folder for files with these endings, as paths relative to it.
func sources(t *testing.T, root string, ends ...string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".build" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, e := range ends {
			if strings.HasSuffix(p, e) {
				rel, _ := filepath.Rel(root, p)
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Everywhere the window's code asks Qt for a place to write, or a settings store.
func TestThePlayerWritesOnlyWhereTheUninstallerLooks(t *testing.T) {
	root := repo(t)
	// Asked for, and known. A person's own Music folder is only where a copied CD is suggested to go.
	knownLocations := map[string]string{"MusicLocation": "only suggests where to copy a CD, in a person's own folder"}
	location := regexp.MustCompile(`QStandardPaths::([A-Z]\w+Location)`)
	tempUse := regexp.MustCompile(`QTemporary\w+|QDir::temp\(\)|QDir::tempPath\(\)`)
	settings := regexp.MustCompile(`\bQSettings\s*(\w+\s*)?[({]`)
	dirs := []string{filepath.Join(root, "src")}
	// The visualizer's Qt code is built into the player, from a checkout beside this one.
	if v := filepath.Join(root, "..", "WittyVisualizer", "qt"); isDir(v) {
		dirs = append(dirs, v)
	}
	for _, dir := range dirs {
		for _, f := range sources(t, dir, ".cpp", ".h") {
			if strings.HasPrefix(f, "tests/") {
				continue
			}
			text := read(t, filepath.Join(dir, f))
			for _, line := range strings.Split(text, "\n") {
				if m := location.FindStringSubmatch(line); m != nil && knownLocations[m[1]] == "" {
					t.Errorf("%s writes to %s, which the uninstaller doesn't know: %s", f, m[1], strings.TrimSpace(line))
				}
				if tempUse.MatchString(line) && !(f == "main.cpp" && strings.Contains(line, ScriptSettingsFolder)) {
					t.Errorf("%s uses the temp folder, which the uninstaller doesn't know: %s", f, strings.TrimSpace(line))
				}
				if settings.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
					switch {
					case strings.Contains(line, "IniFormat, QSettings::UserScope") && strings.Contains(line, `"WittyMediaPlayer"), QStringLiteral("WittyMediaPlayer")`):
						// %APPDATA%\WittyMediaPlayer\WittyMediaPlayer.ini: SettingsEntries.
					case strings.Contains(line, "QSettings *"):
						// Given one, by what's above.
					default:
						t.Errorf("%s opens a settings store the uninstaller doesn't know: %s", f, strings.TrimSpace(line))
					}
				}
			}
		}
	}
	main := read(t, filepath.Join(root, "src", "main.cpp"))
	if !strings.Contains(main, `setOrganizationName(QStringLiteral("WittyMediaPlayer"))`) || !strings.Contains(main, `setApplicationName(QStringLiteral("WittyMediaPlayer"))`) {
		t.Error("the app's name changed, and with it where its settings go")
	}
	if _, ok := SettingsEntries["WittyMediaPlayer.ini"]; !ok {
		t.Error("the settings file is off the list")
	}
}

// Everywhere the helpers' code asks for a place of its own.
func TestTheHelpersWriteOnlyWhereTheUninstallerLooks(t *testing.T) {
	root := filepath.Join(repo(t), "engine")
	known := map[string]bool{
		"internal/modhost/modhost.go\tos.UserCacheDir": true, // %LOCALAPPDATA%\WittyMediaPlayer\modules: LocalEntries
	}
	calls := regexp.MustCompile(`os\.(UserCacheDir|UserConfigDir|UserHomeDir|TempDir)\(\)|os\.(MkdirTemp|CreateTemp)\(""`)
	for _, f := range sources(t, root, ".go") {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "internal/uninstall/") || strings.HasPrefix(f, "tools/") {
			continue
		}
		for _, m := range calls.FindAllString(read(t, filepath.Join(root, f)), -1) {
			call := strings.TrimRight(strings.SplitN(m, "(", 2)[0], "(")
			if !known[f+"\t"+call] {
				t.Errorf("%s calls %s: tell the uninstaller about that place", f, call)
			}
		}
	}
	host := read(t, filepath.Join(root, "internal", "modhost", "modhost.go"))
	if !strings.Contains(host, `filepath.Join(d, "WittyMediaPlayer", "modules")`) || !strings.Contains(host, `filepath.Join(d, "modules")`) {
		t.Error("modules moved, and the uninstaller still looks in the old place")
	}
	// Each module folder it would take from a WMP_DATA folder is a real module, and the other way.
	for _, id := range []string{"wittydvd", "wittybluray"} {
		if !ModuleIDs[id] {
			t.Errorf("%s isn't known", id)
		}
	}
}

// wxsRegistry reads what the installer's source writes to the registry, with its lists unrolled.
func wxsRegistry(t *testing.T, path string) (values [][2]string, keys []string) {
	t.Helper()
	text := expandWxs(t, read(t, path))
	dec := xml.NewDecoder(strings.NewReader(text))
	var stack []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return values, keys
		}
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			attr := func(n string) string {
				for _, a := range el.Attr {
					if a.Name.Local == n {
						return a.Value
					}
				}
				return ""
			}
			switch el.Name.Local {
			case "RegistryKey":
				stack = append(stack, attr("Key"))
				keys = append(keys, attr("Key"))
			case "RegistryValue":
				key := attr("Key")
				if key == "" && len(stack) > 0 {
					key = stack[len(stack)-1]
				} else if len(stack) > 0 {
					key = stack[len(stack)-1] + `\` + key
				}
				values = append(values, [2]string{key, attr("Name")})
			}
		case xml.EndElement:
			if el.Name.Local == "RegistryKey" {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

var (
	wxsDefine  = regexp.MustCompile(`<\?define\s+(\w+)\s*=\s*"?([^"?]*?)"?\s*\?>`)
	wxsForeach = regexp.MustCompile(`(?s)<\?foreach\s+(\w+)\s+in\s+\$\(var\.(\w+)\)\s*\?>(.*?)<\?endforeach\s*\?>`)
	wxsIf      = regexp.MustCompile(`<\?(if|endif|else)[^?]*\?>`)
)

func wxsDefines(text string) map[string]string {
	out := map[string]string{}
	for _, m := range wxsDefine.FindAllStringSubmatch(text, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func expandWxs(t *testing.T, text string) string {
	t.Helper()
	defs := wxsDefines(text)
	text = wxsForeach.ReplaceAllStringFunc(text, func(s string) string {
		m := wxsForeach.FindStringSubmatch(s)
		var b strings.Builder
		list, ok := defs[m[2]]
		if !ok {
			t.Errorf("a list isn't defined: %s", m[2])
		}
		for _, v := range strings.Split(list, ";") {
			b.WriteString(strings.ReplaceAll(m[3], "$(var."+m[1]+")", v))
		}
		return b.String()
	})
	text = wxsDefine.ReplaceAllString(text, "")
	return wxsIf.ReplaceAllString(text, "")
}

// Fills a registry from the .wxs and checks that nothing it wrote is left after removing.
func TestEverythingTheInstallerWritesIsRemoved(t *testing.T) {
	values, keys := wxsRegistry(t, filepath.Join(repo(t), "packaging", "windows", "WittyMediaPlayer.wxs"))
	if len(values) < 40 {
		t.Fatalf("that's not much of an installer: %d registry values", len(values))
	}
	for _, hive := range []string{"HKCU", "HKLM"} {
		m := newMachine(t)
		// Windows has these whether or not the player is installed.
		m.sys.set(hive, registeredApps, "SomeOtherPlayer", "x")
		m.sys.set(hive, classesKey+`\.mkv\OpenWithProgids`, "SomeOtherPlayer.mkv", "")
		for _, k := range keys {
			m.sys.setKey(hive, k)
		}
		for _, v := range values {
			m.sys.set(hive, v[0], v[1], "x")
		}
		m.sys.set(hive, UninstallKey, "ProductCode", "{11111111-2222-3333-4444-555555555555}")
		// It's the worst case: Windows Installer failed, and this has to find everything itself.
		m.sys.answers["msiexec.exe /x"] = func() error { return fmt.Errorf("exit status 1603") }
		Execute(MakePlan(m.env), Options{RemoveData: true})
		for _, v := range values {
			if _, left := m.sys.RegValue(hive, v[0], v[1]); left {
				t.Errorf("%s\\%s\\%s is left in the registry", hive, v[0], v[1])
			}
		}
		for key := range m.sys.keys {
			if strings.Contains(strings.ToLower(key), "witty") {
				t.Errorf("%s is left in the registry", key)
			}
		}
		if _, ok := m.sys.RegValue(hive, registeredApps, "SomeOtherPlayer"); !ok {
			t.Error("another program's Default apps entry went too")
		}
		if _, ok := m.sys.RegValue(hive, classesKey+`\.mkv\OpenWithProgids`, "SomeOtherPlayer.mkv"); !ok {
			t.Error("another program's Open with entry went too")
		}
	}
}

// The installer, this package and the player's File, Open agree on what it opens.
func TestFileTypesAgree(t *testing.T) {
	root := repo(t)
	defs := wxsDefines(read(t, filepath.Join(root, "packaging", "windows", "WittyMediaPlayer.wxs")))
	for progID, want := range map[string]string{"WittyMediaPlayer.Video": "VideoTypes", "WittyMediaPlayer.Music": "MusicTypes", "WittyMediaPlayer.Disc": "DiscTypes"} {
		got := strings.Split(defs[want], ";")
		a, b := append([]string(nil), got...), append([]string(nil), FileTypes[progID]...)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, " ") != strings.Join(b, " ") {
			t.Errorf("%s: the installer says %v and the uninstaller %v", progID, a, b)
		}
	}
	window := read(t, filepath.Join(root, "src", "ui", "mainwindow.cpp"))
	m := regexp.MustCompile(`Media and disc images \(([^)]*)\)`).FindStringSubmatch(window)
	if m == nil {
		t.Fatal("File, Open's list isn't where it was")
	}
	opens := map[string]bool{}
	for _, f := range strings.Fields(m[1]) {
		opens[strings.TrimPrefix(f, "*")] = true
	}
	offered := map[string]bool{}
	for _, ext := range allExts() {
		offered[ext] = true
		if !opens[ext] {
			t.Errorf("the installer offers %s, which File, Open doesn't take", ext)
		}
	}
	for ext := range opens {
		if !offered[ext] && ext != ".bin" { // see FileTypes
			t.Errorf("File, Open takes %s, and the installer doesn't offer it", ext)
		}
	}
}

// Every program with no reason to reach the network stays that way.
func TestTheUninstallerHasNoNetworkCode(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "../../cmd/wittymediaplayer-uninstall").CombinedOutput()
	if err != nil {
		t.Skipf("go list: %v: %s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "net" || strings.HasPrefix(dep, "net/") {
			t.Errorf("it brings in %s", dep)
		}
	}
}
