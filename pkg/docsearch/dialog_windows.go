//go:build windows

package docsearch

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")

	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")

	callSHBrowseForFolderW = func(bi *browseInfoW) uintptr {
		ret, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(bi)))
		return ret
	}
	callSHGetPathFromIDListW = func(pidl uintptr, pszPath *uint16) uint32 {
		ret, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(pszPath)))
		return uint32(ret)
	}
	callCoTaskMemFree = func(pv uintptr) {
		procCoTaskMemFree.Call(pv)
	}
)

const (
	bifReturnOnlyFsDirs  = 0x00000001
	bifNewDialogStyle    = 0x00000040
	bifNoNewFolderButton = 0x00000200
)

type browseInfoW struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

func openIndexDialogImpl(parentHWnd uintptr) (string, bool, error) {
	titlePtr := windows.StringToUTF16Ptr("Select Bleve Index Directory (*.bleve)")

	bi := browseInfoW{
		hwndOwner: parentHWnd,
		lpszTitle: titlePtr,
		ulFlags:   bifReturnOnlyFsDirs | bifNewDialogStyle | bifNoNewFolderButton,
	}

	pidl := callSHBrowseForFolderW(&bi)
	if pidl == 0 {
		// User canceled
		return "", false, nil
	}
	defer callCoTaskMemFree(pidl)

	buf := make([]uint16, 1024)
	ret := callSHGetPathFromIDListW(pidl, &buf[0])
	if ret == 0 {
		return "", false, fmt.Errorf("failed to retrieve directory path from selected item")
	}

	selectedPath := windows.UTF16ToString(buf)
	return selectedPath, true, nil
}
