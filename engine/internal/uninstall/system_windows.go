//go:build windows

package uninstall

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Straight system calls, as in the rest of this repository: the usual Go package for these isn't
// needed, and every program here but the updater stays free of network code.
var (
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procRegDeleteTreeW  = advapi32.NewProc("RegDeleteTreeW")
	procRegDeleteValueW = advapi32.NewProc("RegDeleteValueW")
	procRegEnumValueW   = advapi32.NewProc("RegEnumValueW")
	procRegSetValueExW  = advapi32.NewProc("RegSetValueExW")

	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")

	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW             = user32.NewProc("PostMessageW")
)

const (
	createNoWindow                 = 0x08000000
	processQueryLimitedInformation = 0x1000
	processTerminate               = 0x0001
	synchronize                    = 0x00100000
	wmClose                        = 0x0010
	tokenElevation                 = 20
	errorFileNotFound              = syscall.Errno(2)
	errorMoreData                  = syscall.Errno(234)
)

type realSystem struct{ userSID string }

func newSystem(userSID string) System { return realSystem{userSID} }

func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

func (realSystem) Run(name string, args ...string) (string, error) { return runProgram(name, args...) }

func isAdmin() bool {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer tok.Close()
	var elevated, n uint32
	err = syscall.GetTokenInformation(tok, tokenElevation, (*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &n)
	return err == nil && elevated != 0
}

// CurrentUserSID is this account's security ID as text.
func CurrentUserSID() string {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return ""
	}
	defer tok.Close()
	u, err := tok.GetTokenUser()
	if err != nil {
		return ""
	}
	s, err := u.User.Sid.String()
	if err != nil {
		return ""
	}
	return s
}

// root is where a key starts: HKCU is the person who asked, and only they know that once a worker
// is running as an administrator.
func (s realSystem) root(h, key string) (syscall.Handle, string) {
	if h == "HKLM" {
		return syscall.HKEY_LOCAL_MACHINE, key
	}
	if s.userSID != "" {
		return syscall.HKEY_USERS, s.userSID + `\` + key
	}
	return syscall.HKEY_CURRENT_USER, key
}

func (s realSystem) open(h, key string, access uint32) (syscall.Handle, error) {
	r, sub := s.root(h, key)
	p, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return 0, err
	}
	var k syscall.Handle
	if err := syscall.RegOpenKeyEx(r, p, 0, access, &k); err != nil {
		return 0, err
	}
	return k, nil
}

func (s realSystem) RegKeyExists(h, key string) bool {
	k, err := s.open(h, key, syscall.KEY_READ)
	if err != nil {
		return false
	}
	syscall.RegCloseKey(k)
	return true
}

func (s realSystem) RegValue(h, key, name string) (string, bool) {
	k, err := s.open(h, key, syscall.KEY_QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer syscall.RegCloseKey(k)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}
	var typ, size uint32
	if err := syscall.RegQueryValueEx(k, n, nil, &typ, nil, &size); err != nil {
		return "", false
	}
	if typ != syscall.REG_SZ && typ != syscall.REG_EXPAND_SZ || size == 0 || size > 1<<20 {
		// There, but not text.
		return "", true
	}
	buf := make([]uint16, size/2+1)
	if err := syscall.RegQueryValueEx(k, n, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &size); err != nil {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func (s realSystem) RegValueNames(h, key string) []string {
	k, err := s.open(h, key, syscall.KEY_QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer syscall.RegCloseKey(k)
	var out []string
	buf := make([]uint16, 16384)
	for i := uint32(0); ; i++ {
		n := uint32(len(buf))
		r, _, _ := procRegEnumValueW.Call(uintptr(k), uintptr(i), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0)
		// More data only means a value's data is long, and it isn't being asked for.
		if e := syscall.Errno(r); e != 0 && e != errorMoreData {
			return out
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
}

func (s realSystem) RegSubkeys(h, key string) []string {
	k, err := s.open(h, key, syscall.KEY_ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer syscall.RegCloseKey(k)
	var out []string
	buf := make([]uint16, 512)
	for i := uint32(0); ; i++ {
		n := uint32(len(buf))
		if err := syscall.RegEnumKeyEx(k, i, &buf[0], &n, nil, nil, nil, nil); err != nil {
			return out
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
}

func (s realSystem) RegSetValue(h, key, name, value string) error {
	k, err := s.open(h, key, syscall.KEY_SET_VALUE)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(k)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	r, _, _ := procRegSetValueExW.Call(uintptr(k), uintptr(unsafe.Pointer(n)), 0, syscall.REG_SZ, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func (s realSystem) RegDeleteTree(h, key string) error {
	r, sub := s.root(h, key)
	p, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return err
	}
	rc, _, _ := procRegDeleteTreeW.Call(uintptr(r), uintptr(unsafe.Pointer(p)))
	if e := syscall.Errno(rc); e != 0 && e != errorFileNotFound {
		return e
	}
	return nil
}

func (s realSystem) RegDeleteValue(h, key, name string) error {
	k, err := s.open(h, key, syscall.KEY_SET_VALUE)
	if err != nil {
		if errors.Is(err, errorFileNotFound) {
			return nil
		}
		return err
	}
	defer syscall.RegCloseKey(k)
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	rc, _, _ := procRegDeleteValueW.Call(uintptr(k), uintptr(unsafe.Pointer(n)))
	if e := syscall.Errno(rc); e != 0 && e != errorFileNotFound {
		return e
	}
	return nil
}

// longPath gives a path's full, long form in lower case for comparing (Windows may name the
// same folder C:\Users\LONGNA~1).
func longPath(p string) string {
	p = filepath.Clean(p)
	if u, err := syscall.UTF16PtrFromString(p); err == nil {
		buf := make([]uint16, 32768)
		if n, err := syscall.GetLongPathName(u, &buf[0], uint32(len(buf))); err == nil && n > 0 && int(n) < len(buf) {
			p = syscall.UTF16ToString(buf[:n])
		}
	}
	return strings.ToLower(p)
}

func imagePath(pid uint32) string {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return ""
	}
	return longPath(syscall.UTF16ToString(buf[:n]))
}

// running lists the processes whose program is inside dir, but not this one.
func running(dir string) []uint32 {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	prefix := longPath(dir) + `\`
	var out []uint32
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ProcessID == uint32(os.Getpid()) || e.ProcessID == 0 {
			continue
		}
		if strings.HasPrefix(imagePath(e.ProcessID), prefix) {
			out = append(out, e.ProcessID)
		}
	}
	return out
}

func kill(pid uint32) {
	h, err := syscall.OpenProcess(processTerminate, false, pid)
	if err != nil {
		return
	}
	_ = syscall.TerminateProcess(h, 1)
	syscall.CloseHandle(h)
}

// closeWindowsOf asks a program's windows to close, as clicking their X does.
func closeWindowsOf(pid uint32) {
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var owner uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner == pid {
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}

// StopProgramsIn asks the window to close first, which saves its place and stops a copy properly,
// and the helpers go with it. Whatever still runs after a while is ended.
func (realSystem) StopProgramsIn(dir string) {
	for _, pid := range running(dir) {
		if strings.EqualFold(filepath.Base(imagePath(pid)), exeName) {
			closeWindowsOf(pid)
		}
	}
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if len(running(dir)) == 0 {
			return
		}
	}
	for _, pid := range running(dir) {
		kill(pid)
	}
	time.Sleep(time.Second)
}

// WaitForProcess waits a few seconds for a process to end.
func WaitForProcess(pid int) {
	h, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	_, _ = syscall.WaitForSingleObject(h, 10000)
}
