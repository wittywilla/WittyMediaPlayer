//go:build windows

package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var procRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")

func makeKey(t *testing.T, key string) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		t.Fatal(err)
	}
	var k syscall.Handle
	var disp uint32
	r, _, _ := procRegCreateKeyExW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(p)), 0, 0, 0, syscall.KEY_ALL_ACCESS, 0, uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&disp)))
	if r != 0 {
		t.Fatal(syscall.Errno(r))
	}
	syscall.RegCloseKey(k)
}

// The real registry, in a key of this test's own that it always takes away again.
func TestTheRealRegistry(t *testing.T) {
	base := fmt.Sprintf(`Software\WittyMediaPlayer-test-%d`, os.Getpid())
	s := newSystem("")
	t.Cleanup(func() { _ = s.RegDeleteTree("HKCU", base) })

	if s.RegKeyExists("HKCU", base) {
		t.Fatal("it's there before it was made")
	}
	if err := s.RegDeleteTree("HKCU", base); err != nil {
		t.Errorf("removing what isn't there: %v", err)
	}
	if err := s.RegDeleteValue("HKCU", base, "x"); err != nil {
		t.Errorf("removing a value that isn't there: %v", err)
	}
	makeKey(t, base+`\one\deeper`)
	makeKey(t, base+`\two`)
	if !s.RegKeyExists("HKCU", base) {
		t.Fatal("it isn't there after it was made")
	}
	for name, value := range map[string]string{"a": "first", "MRUList": "ab", "": "the default", `C:\Some Folder\x.exe.FriendlyAppName`: "with spaces"} {
		if err := s.RegSetValue("HKCU", base, name, value); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{"a": "first", "": "the default", `C:\Some Folder\x.exe.FriendlyAppName`: "with spaces"} {
		if got, ok := s.RegValue("HKCU", base, name); !ok || got != want {
			t.Errorf("value %q is %q, %v", name, got, ok)
		}
	}
	if _, ok := s.RegValue("HKCU", base, "absent"); ok {
		t.Error("found a value that isn't there")
	}
	names := s.RegValueNames("HKCU", base)
	sort.Strings(names)
	if strings.Join(names, "|") != `|C:\Some Folder\x.exe.FriendlyAppName|MRUList|a` {
		t.Errorf("names: %q", names)
	}
	subs := s.RegSubkeys("HKCU", base)
	sort.Strings(subs)
	if strings.Join(subs, "|") != "one|two" {
		t.Errorf("subkeys: %q", subs)
	}
	// The same key by way of the account's SID, as a worker raised to an administrator sees it.
	if sid := CurrentUserSID(); !strings.HasPrefix(sid, "S-1-") {
		t.Errorf("SID: %q", sid)
	} else if v, ok := newSystem(sid).RegValue("HKCU", base, "a"); !ok || v != "first" {
		t.Errorf("through the SID: %q, %v", v, ok)
	}
	if err := s.RegDeleteValue("HKCU", base, "a"); err != nil {
		t.Error(err)
	}
	if _, ok := s.RegValue("HKCU", base, "a"); ok {
		t.Error("the value stayed")
	}
	// Removing the Open with entry prunes what it leaves empty, using the real calls.
	env := &Env{Sys: s}
	if err := pruneKey(env, "HKCU", base+`\two`); err != nil {
		t.Error(err)
	}
	if s.RegKeyExists("HKCU", base+`\two`) {
		t.Error("an empty key wasn't pruned")
	}
	if err := pruneKey(env, "HKCU", base+`\one`); err != nil {
		t.Error(err)
	}
	if !s.RegKeyExists("HKCU", base+`\one`) {
		t.Error("a key with something in it was pruned")
	}
	if err := s.RegDeleteTree("HKCU", base); err != nil {
		t.Fatal(err)
	}
	if s.RegKeyExists("HKCU", base) {
		t.Error("it's still there")
	}
}

// TestSleeper isn't a test: it's a program for the test below to find and stop.
func TestSleeper(t *testing.T) {
	if os.Getenv("WMP_TEST_SLEEP") == "" {
		t.Skip("only run by TestStopProgramsIn")
	}
	time.Sleep(time.Minute)
}

func TestStopProgramsIn(t *testing.T) {
	if testing.Short() {
		t.Skip("it waits for a program to close")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	copyTo := filepath.Join(dir, "helper.exe")
	if err := copyFile(self, copyTo); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(copyTo, "-test.run=TestSleeper")
	cmd.Env = append(os.Environ(), "WMP_TEST_SLEEP=1")
	hide(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(20 * time.Second)
	for len(running(dir)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("it never showed up as running")
		}
		time.Sleep(50 * time.Millisecond)
	}
	realSystem{}.StopProgramsIn(dir)
	if left := running(dir); len(left) != 0 {
		t.Errorf("still running: %v", left)
	}
	_ = cmd.Wait()
	if err := os.Remove(copyTo); err != nil {
		t.Errorf("its file is still held: %v", err)
	}
}
