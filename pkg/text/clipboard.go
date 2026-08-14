package text

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// coInitialize is a variable so that tests can exercise the failure path
// without a real COM apartment.
var coInitialize = ole.CoInitialize

var CopyClipboardImpl = defaultCopyClipboard

func CopyClipboard(content string) (string, error) {
	return CopyClipboardImpl(content)
}

func defaultCopyClipboard(content string) (string, error) {
	// COM apartments are per OS thread, so the goroutine must stay on the
	// thread it initialized. Without this the runtime is free to move it and
	// the following OLE calls land on a thread where COM was never set up.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (code 1) only means COM was already initialized on this thread.
	// Any other failure leaves no usable apartment, so fall back the same way a
	// missing "htmlfile" object does rather than calling into OLE regardless.
	if err := coInitialize(0); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return "Text processed (Clipboard fallback)", nil
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("htmlfile")
	if err != nil {
		return "Text processed (Clipboard fallback)", nil
	}
	// Deliberately not releasing this IUnknown. Dropping its reference on top of
	// the IDispatch release below tears the htmlfile object down far enough that
	// the *next* CreateObject("htmlfile") in the same process hands back a handle
	// whose parentWindow lookup fails. Only the IDispatch is released.
	html, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "Text processed (Clipboard fallback)", nil
	}
	defer html.Release()

	parentWindow := oleutil.MustGetProperty(html, "parentWindow").ToIDispatch()
	defer parentWindow.Release()

	clipboardData := oleutil.MustGetProperty(parentWindow, "clipboardData").ToIDispatch()
	defer clipboardData.Release()

	_, err = oleutil.CallMethod(clipboardData, "setData", "Text", content)
	if err != nil {
		return "", fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return "Text copied to clipboard successfully.", nil
}
