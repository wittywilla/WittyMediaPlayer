//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsl = kernel32.NewProc("AttachConsole")
)

const (
	mbOK              = 0x00000000
	mbYesNo           = 0x00000004
	mbIconQuestion    = 0x00000020
	mbIconInformation = 0x00000040
	mbSetForeground   = 0x00010000
	idYes             = 6
)

func box(caption, text string, flags uintptr) int32 {
	t, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return 0
	}
	c, err := syscall.UTF16PtrFromString(caption)
	if err != nil {
		return 0
	}
	r, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags|mbSetForeground)
	return int32(r)
}

func show(caption, text string) { box(caption, text, mbOK|mbIconInformation) }

func confirm(caption, text string) bool {
	return box(caption, text, mbYesNo|mbIconQuestion) == idYes
}

// attachConsole answers in the terminal this was started from. It's built with no console of its
// own, so that one doesn't flash up behind its dialogs.
func attachConsole() {
	if _, err := os.Stdout.Stat(); err == nil {
		return // already going somewhere: a file, or a pipe someone is reading
	}
	const attachParent = ^uintptr(0)
	if r, _, _ := procAttachConsl.Call(attachParent); r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}
