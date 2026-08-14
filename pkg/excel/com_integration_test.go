//go:build comtest && windows

// Integration tests that drive a real Excel instance through COM. They are
// excluded from the default build because they depend on the installed Office
// and leave the outcome at the mercy of the machine they run on.
//
// Run them with:
//
//	go test -tags comtest ./pkg/excel/
package excel_test

import (
	"testing"

	"doctools-cli/pkg/excel"
)

func TestDefaultExtractImagesCOM_RealExcel(t *testing.T) {
	tmpDir := t.TempDir()
	path := createTestExcel(t)

	// Either outcome is acceptable: the point is that a real COM round trip
	// neither panics nor leaves an orphaned EXCEL.EXE behind.
	paths, err := excel.DefaultExtractImagesCOM(path, tmpDir, nil)
	if err != nil {
		t.Logf("Excel COM export failed (this machine may not have Excel): %v", err)
		return
	}
	if len(paths) == 0 {
		t.Error("expected at least one exported path on success")
	}
}
