// Command wittymediaplayer-uninstall is the uninstaller: Uninstall WittyMediaPlayer.exe, which
// Installed apps points at. The work is in internal/uninstall.
//
//	Uninstall WittyMediaPlayer.exe                 asks
//	Uninstall WittyMediaPlayer.exe /S              silently, keeps settings and modules
//	Uninstall WittyMediaPlayer.exe /S /RemoveData  silently, a full clean-up
//
// It asks first, running unprivileged from wherever it is. The work is then done by a worker, which
// may need raising to an administrator, and runs from a copy in the temp folder, since a running
// program can't be deleted on Windows.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wittywilla/wittymediaplayer/engine/internal/uninstall"
)

const title = "Uninstall WittyMediaPlayer"

func main() {
	attachConsole()
	a, err := uninstall.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if !a.Yes {
			show(title, err.Error()+"\n\n"+uninstall.Usage)
		}
		os.Exit(2)
	}
	if a.Help {
		fmt.Print(uninstall.Usage)
		return
	}
	e := uninstall.Here(a.UserSID)
	if a.ProgramFolder != "" {
		e.SelfRoot = a.ProgramFolder
	}
	switch {
	case a.StopPrograms:
		// The installer's own step, before it replaces or removes the files in this folder.
		if e.SelfRoot != "" {
			e.Sys.StopProgramsIn(e.SelfRoot)
		}
	case a.DryRun:
		out := uninstall.DryRun(uninstall.MakePlan(e))
		fmt.Print(out)
		if a.Report != "" {
			if err := os.WriteFile(a.Report, []byte(out), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	case a.Worker:
		if a.Parent != 0 {
			uninstall.WaitForProcess(a.Parent)
		}
		os.Exit(work(e, a))
	default:
		os.Exit(ask(e, a))
	}
}

// ask is the part a person sees first.
func ask(e *uninstall.Env, a uninstall.Args) int {
	pl := uninstall.MakePlan(e)
	if len(pl.Items) == 0 {
		if !a.Yes {
			show(title, "Nothing of WittyMediaPlayer was found on this computer.")
		}
		return 0
	}
	if !a.Yes {
		if !confirm(title, "Remove WittyMediaPlayer from this computer?\n\n"+
			"This closes it if it's open, and removes the program, its shortcuts and the file types it was set to open.\n\n"+
			"Your movies, music and discs, and the CDs and disc images you copied, are never touched.") {
			return 1
		}
		if !a.KeepData && !a.RemoveData && pl.Has(uninstall.GroupData) {
			a.RemoveData = confirm(title, "Also delete your settings, and the disc modules it downloaded?\n\n"+
				"That's your volume, theme and window size, and any newer WittyDVD or WittyBluray ("+uninstall.Size(pl.Bytes(uninstall.GroupData))+"). "+
				"The player keeps no logs and no caches of its own.\n\n"+
				"Choose Yes for a full clean-up. Choose No to keep them, so they're still there if you install it again.")
		}
	}
	a.ProgramFolder = e.SelfRoot
	if a.Report == "" {
		a.Report = uninstall.ReportPath()
	}
	a.Parent = os.Getpid()
	a.Unattended = a.Yes
	elevate := pl.NeedsAdmin(a.Options) && !e.IsAdmin
	if elevate {
		a.UserSID = uninstall.CurrentUserSID()
	}
	if err := uninstall.StartWorker(a.Flags(), elevate); err != nil {
		if !a.Yes {
			if elevate {
				show(title, "WittyMediaPlayer wasn't removed. It needs an administrator's permission, and that wasn't given.")
			} else {
				show(title, fmt.Sprintf("WittyMediaPlayer wasn't removed.\n\n%s", err))
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}

// work removes everything, and says what happened.
func work(e *uninstall.Env, a uninstall.Args) int {
	res := uninstall.Execute(uninstall.MakePlan(e), a.Options)
	reportErr := uninstall.WriteReport(a.Report, res)
	uninstall.RemoveSelfLater()
	code := 0
	if !res.OK() {
		code = 1
	}
	if a.Unattended {
		return code
	}
	text := "WittyMediaPlayer was removed."
	if !res.OK() {
		text = "Most of WittyMediaPlayer was removed, but some of it couldn't be."
	}
	text += "\n\n" + shorten(res.Report(), 30)
	if a.Report != "" && reportErr == nil {
		text += "\n\nThe full list is in " + filepath.ToSlash(a.Report)
	}
	show(title, text)
	return code
}

// shorten keeps a dialog on the screen: the full list is in the report.
func shorten(s string, lines int) string {
	n := 0
	for i, c := range s {
		if c == '\n' {
			n++
			if n == lines {
				return s[:i] + "\n..."
			}
		}
	}
	return s
}
