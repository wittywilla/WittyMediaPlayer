package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Here is this machine, as the running program sees it. userSID is the account that asked, for a
// worker raised to an administrator: "HKCU" then still means that person, not the administrator.
func Here(userSID string) *Env {
	root := ""
	if self, err := os.Executable(); err == nil {
		if _, err := ReadPackageList(filepath.Dir(self)); err == nil {
			root = filepath.Dir(self)
		}
	}
	return &Env{Getenv: os.Getenv, IsAdmin: isAdmin(), SelfRoot: root, Sys: newSystem(userSID)}
}

func runProgram(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	hide(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if len(text) > 300 {
			text = text[:300]
		}
		if text != "" {
			return string(out), fmt.Errorf("%w: %s", err, text)
		}
	}
	return string(out), err
}
