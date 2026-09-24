//go:build windows

package docsearch

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	procEnumWindows     = user32.NewProc("EnumWindows")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

func activateDocSearchWindowImpl() bool {
	var foundHWnd uintptr

	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1 // continue enumeration
		}

		lenRet, _, _ := procGetWindowTextLengthW.Call(hwnd)
		length := int(lenRet)
		if length == 0 {
			return 1 // continue enumeration
		}

		buf := make([]uint16, length+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
		title := syscall.UTF16ToString(buf)

		if strings.Contains(title, "DocSearch") {
			foundHWnd = hwnd
			return 0 // stop enumeration
		}
		return 1 // continue enumeration
	})

	procEnumWindows.Call(cb, 0)

	if foundHWnd != 0 {
		const swRestore = 9
		procShowWindow.Call(foundHWnd, uintptr(swRestore))
		procSetForegroundWindow.Call(foundHWnd)
		return true
	}
	return false
}
