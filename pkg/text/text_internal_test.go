package text

import (
	"fmt"
	"testing"

	"github.com/go-ole/go-ole"
)

func TestCopyClipboard_Mock(t *testing.T) {
	oldHook := CopyClipboardImpl
	defer func() { CopyClipboardImpl = oldHook }()

	CopyClipboardImpl = func(content string) (string, error) {
		if content == "err" {
			return "", fmt.Errorf("mock clipboard error")
		}
		return "Text copied to clipboard successfully.", nil
	}

	res, err := CopyClipboard("test")
	if err != nil || res == "" {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = CopyClipboard("err")
	if err == nil {
		t.Errorf("expected error from mock")
	}
}

func TestDefaultCopyClipboard(t *testing.T) {
	// Call defaultCopyClipboard directly
	_, _ = defaultCopyClipboard("test default")
}

// TestDefaultCopyClipboard_CoInitializeFailure checks that an unusable COM
// apartment falls back instead of calling into OLE anyway.
func TestDefaultCopyClipboard_CoInitializeFailure(t *testing.T) {
	old := coInitialize
	defer func() { coInitialize = old }()
	coInitialize = func(p uintptr) error {
		return ole.NewError(ole.E_UNEXPECTED)
	}

	msg, err := defaultCopyClipboard("text")
	if err != nil {
		t.Fatalf("expected the fallback path, got error: %v", err)
	}
	if msg != "Text processed (Clipboard fallback)" {
		t.Errorf("expected the clipboard fallback message, got %q", msg)
	}
}
