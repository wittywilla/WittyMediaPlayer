//go:build windows

package uninstall

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// StartWorker copies this program to the temp folder and starts the copy, because Windows won't
// delete a running program, and the copy can delete this one. It doesn't wait: this process has
// to end for its own file to go.
func StartWorker(flags []string, elevate bool) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "wittymediaplayer-uninstall-")
	if err != nil {
		return err
	}
	copyTo := filepath.Join(dir, filepath.Base(self))
	if err := copyFile(self, copyTo); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if elevate {
		return runElevated(copyTo, dir, flags)
	}
	cmd := exec.Command(copyTo, flags...)
	cmd.Dir = dir
	hide(cmd)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return cmd.Process.Release()
}

func runElevated(file, dir string, flags []string) error {
	quoted := make([]string, len(flags))
	for i, f := range flags {
		quoted[i] = syscall.EscapeArg(f)
	}
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exe, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	args, err := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return err
	}
	wd, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	const showNormal = 1
	r, _, callErr := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(exe)), uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(wd)), showNormal)
	// Above 32 is success.
	if r <= 32 {
		_ = os.RemoveAll(dir)
		return callErr
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// RemoveSelfLater clears a worker's temp folder after it has exited. Programs by their full
// path: cmd.exe looks in the current folder first, and anything may be in a temp folder.
func RemoveSelfLater() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(self)
	if !strings.HasPrefix(strings.ToLower(filepath.Base(dir)), "wittymediaplayer-uninstall-") {
		return
	}
	sys := filepath.Join(os.Getenv("SystemRoot"), "System32")
	cmd := exec.Command(filepath.Join(sys, "cmd.exe"))
	cmd.Dir = os.TempDir()
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow,
		CmdLine: `cmd.exe /d /c "` + filepath.Join(sys, "PING.EXE") + `" -n 3 127.0.0.1 >nul & rmdir /s /q "` + dir + `"`}
	_ = cmd.Start()
}
