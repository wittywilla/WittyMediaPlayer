package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Args is how the uninstaller is told what to do.
type Args struct {
	Options
	KeepData      bool   // the answer to the data question, given up front
	Yes           bool   // don't ask anything
	DryRun        bool   // say what would go, and stop
	Help          bool   // say how to use it
	StopPrograms  bool   // only close what's running from this folder: the installer asks this
	ProgramFolder string // the folder a copy in the temp folder was started from
	Worker        bool   // the questions were asked: do the work
	Report        string // write the result here as well
	Unattended    bool   // nobody is there: a worker shows no dialog when it's done
	Parent        int    // a worker waits for this process to end, so its file can go
	UserSID       string // the account that asked, when a worker was raised to an administrator
}

// Usage is how to use it, for --help.
const Usage = `Uninstall WittyMediaPlayer.exe                  asks first
Uninstall WittyMediaPlayer.exe /S               silently, keeps your settings and modules
Uninstall WittyMediaPlayer.exe /S /RemoveData   silently, a full clean-up

  /S, --yes             ask nothing. It returns at once and works in the background.
  /RemoveData           also delete settings and downloaded modules
  /KeepData             keep them, without being asked
  --dry-run             say what it would do, and change nothing
  --report <file>       write what it did to this file, which appears when it has finished
`

// ParseArgs reads --name, and /Name as Windows installers have them.
func ParseArgs(argv []string) (Args, error) {
	var a Args
	for i := 0; i < len(argv); i++ {
		name := strings.ToLower(strings.TrimLeft(argv[i], "-/"))
		next := func() (string, error) {
			if i+1 >= len(argv) {
				return "", fmt.Errorf("%s needs a value", argv[i])
			}
			i++
			return argv[i], nil
		}
		var err error
		switch name {
		case "uninstall":
		case "remove-data", "removedata":
			a.RemoveData = true
		case "keep-data", "keepdata":
			a.KeepData = true
		case "yes", "y", "s", "silent", "quiet", "q":
			a.Yes = true
		case "dry-run", "dryrun":
			a.DryRun = true
		case "help", "h", "?":
			a.Help = true
		case "stop-programs":
			a.StopPrograms = true
		case "worker":
			a.Worker = true
		case "unattended":
			a.Unattended = true
		case "program-folder":
			a.ProgramFolder, err = next()
		case "report":
			a.Report, err = next()
		case "user-sid":
			a.UserSID, err = next()
		case "parent":
			var v string
			if v, err = next(); err == nil {
				a.Parent, err = strconv.Atoi(v)
			}
		default:
			return a, fmt.Errorf("unknown option %s", argv[i])
		}
		if err != nil {
			return a, err
		}
	}
	if a.RemoveData && a.KeepData {
		return a, fmt.Errorf("/RemoveData and /KeepData can't go together")
	}
	return a, nil
}

// Flags is the worker's command line for these answers.
func (a Args) Flags() []string {
	f := []string{"--worker", "--yes"}
	if a.RemoveData {
		f = append(f, "--remove-data")
	} else {
		f = append(f, "--keep-data")
	}
	if a.ProgramFolder != "" {
		f = append(f, "--program-folder", a.ProgramFolder)
	}
	if a.Report != "" {
		f = append(f, "--report", a.Report)
	}
	if a.UserSID != "" {
		f = append(f, "--user-sid", a.UserSID)
	}
	if a.Parent != 0 {
		f = append(f, "--parent", strconv.Itoa(a.Parent))
	}
	if a.Unattended {
		f = append(f, "--unattended")
	}
	return f
}

// ReportPath is a place for the list of what was removed that outlives everything removed. Its name
// is short so that a dialog can show it whole, and a later uninstall replaces it.
func ReportPath() string {
	return filepath.Join(os.TempDir(), "WittyMediaPlayer-uninstall.txt")
}

// WriteReport keeps the result where it was asked for. It appears all at once, so that a script
// waiting for it knows the work is over.
func WriteReport(path string, res Result) error {
	if path == "" {
		return nil
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, []byte(res.Report()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DryRun is what a plan would do, in plain words.
func DryRun(pl *Plan) string {
	var b strings.Builder
	if len(pl.Items) == 0 {
		b.WriteString("Nothing of WittyMediaPlayer was found on this computer.\n")
	} else {
		b.WriteString("It would remove:\n")
		b.WriteString(pl.Describe(GroupProgram))
		if pl.Has(GroupData) {
			b.WriteString("\nWith /RemoveData it would also remove:\n")
			b.WriteString(pl.Describe(GroupData))
		}
	}
	if len(pl.Kept) > 0 {
		b.WriteString("\nIt would leave in place:\n")
		for _, k := range pl.Kept {
			fmt.Fprintf(&b, "  %s: %s\n", k.Name, k.Reason)
		}
	}
	b.WriteString("\nNothing was changed.\n")
	return b.String()
}
