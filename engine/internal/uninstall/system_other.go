//go:build !windows

package uninstall

import (
	"errors"
	"os/exec"
)

// The player is only built for Windows. This is here so the package's plan and its tests build
// and run anywhere.
type realSystem struct{}

func newSystem(string) System { return realSystem{} }

func hide(*exec.Cmd) {}

func isAdmin() bool { return false }

var errNotWindows = errors.New("there is no registry here")

func (realSystem) Run(name string, args ...string) (string, error) { return runProgram(name, args...) }
func (realSystem) StopProgramsIn(string)                           {}
func (realSystem) RegKeyExists(string, string) bool                { return false }
func (realSystem) RegValue(string, string, string) (string, bool)  { return "", false }
func (realSystem) RegValueNames(string, string) []string           { return nil }
func (realSystem) RegSubkeys(string, string) []string              { return nil }
func (realSystem) RegSetValue(string, string, string, string) error {
	return errNotWindows
}
func (realSystem) RegDeleteTree(string, string) error          { return errNotWindows }
func (realSystem) RegDeleteValue(string, string, string) error { return errNotWindows }

// StartWorker is for Windows.
func StartWorker([]string, bool) error { return errNotWindows }

// RemoveSelfLater is for Windows.
func RemoveSelfLater() {}

// WaitForProcess is for Windows.
func WaitForProcess(int) {}

// CurrentUserSID is for Windows.
func CurrentUserSID() string { return "" }
