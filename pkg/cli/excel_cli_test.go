package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeExcel(t *testing.T, path string, build func(f *excelize.File)) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	build(f)
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("failed to save %s: %v", path, err)
	}
}

// TestExcelDiffTextOutput covers the human readable rendering of every
// CellDiff type, including the merged range suffix and the no-difference case.
func TestExcelDiffTextOutput(t *testing.T) {
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.xlsx")
	writeExcel(t, fileA, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "Hello")
		f.SetCellFormula("Sheet1", "B1", "1+1")
		f.SetCellValue("Sheet1", "C1", "merged-a")
		f.MergeCell("Sheet1", "C1", "D1")
		f.SetCellValue("Sheet1", "E1", "plain")
	})

	fileB := filepath.Join(tmpDir, "b.xlsx")
	writeExcel(t, fileB, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "World")
		f.SetCellFormula("Sheet1", "B1", "2+2")
		f.SetCellValue("Sheet1", "C1", "merged-b")
		f.MergeCell("Sheet1", "C1", "D1")
		f.SetCellValue("Sheet1", "E1", "plain")
		f.MergeCell("Sheet1", "E1", "F1")
	})

	out, err, exit := runCLI("excel", "diff", fileA, fileB)
	if err != nil || exit != nil {
		t.Fatalf("excel diff failed: err=%v exit=%v", err, exit)
	}

	for _, want := range []string{
		"Excel Diff Result:",
		"Value changed: 'Hello' -> 'World'",
		"Formula changed: '1+1' -> '2+2'",
		"Merge range changed:",
		"(Merged: C1:D1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diff output missing %q:\n%s", want, out)
		}
	}

	// Identical files take the "no differences" branch.
	out, err, exit = runCLI("excel", "diff", fileA, fileA)
	if err != nil || exit != nil {
		t.Fatalf("excel diff (identical) failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "No differences found.") {
		t.Errorf("expected no-difference message, got:\n%s", out)
	}
}

func TestExcelDiffError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.xlsx")

	_, _, exit := runCLI("excel", "diff", missing, missing)
	if exit == nil {
		t.Fatal("expected excel diff to exit with an error for a missing file")
	}
}

// TestExcelPatchArgs covers the custom Args validator, which only enforces the
// single-argument rule when --schema is absent.
func TestExcelPatchArgs(t *testing.T) {
	_, err, _ := runCLI("excel", "patch")
	if err == nil {
		t.Fatal("expected an error when patch is called without a target file")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Errorf("unexpected args error: %v", err)
	}

	out, err, exit := runCLI("excel", "patch", "--schema")
	if err != nil || exit != nil {
		t.Fatalf("excel patch --schema failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, `"sheet"`) || !strings.Contains(out, `"new_value"`) {
		t.Errorf("unexpected schema output: %s", out)
	}
}

func TestExcelPatchErrorPaths(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.xlsx")
	writeExcel(t, target, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "Hello")
	})

	validPatch := filepath.Join(tmpDir, "patch.json")
	os.WriteFile(validPatch, []byte(`[{"sheet":"Sheet1","cell":"A1","old_value":"Hello","new_value":"Patched"}]`), 0644)

	brokenPatch := filepath.Join(tmpDir, "broken.json")
	os.WriteFile(brokenPatch, []byte(`{ this is not a patch array `), 0644)

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHint bool
	}{
		{
			name:     "patch file flag omitted",
			args:     []string{"excel", "patch", target},
			wantErr:  "--patch-file flag is required",
			wantHint: true,
		},
		{
			name:     "patch file unreadable",
			args:     []string{"excel", "patch", target, "-p", filepath.Join(tmpDir, "nope.json")},
			wantErr:  "failed to read patch file",
			wantHint: true,
		},
		{
			name:     "patch file not valid json",
			args:     []string{"excel", "patch", target, "-p", brokenPatch},
			wantErr:  "failed to parse patch JSON file",
			wantHint: true,
		},
		{
			name:    "target file missing",
			args:    []string{"excel", "patch", filepath.Join(tmpDir, "missing.xlsx"), "-p", validPatch},
			wantErr: "target file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, exit := runCLI(tt.args...)
			if exit == nil {
				t.Fatalf("expected the command to exit with an error")
			}
			if !strings.Contains(exit.err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, exit.err)
			}
			if tt.wantHint && exit.hint != patchHint {
				t.Errorf("expected the patch hint to be attached, got: %q", exit.hint)
			}
		})
	}
}

// TestExcelPatchTextOutput covers the human readable patch report, including
// the backup file line and the audit log block.
func TestExcelPatchTextOutput(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.xlsx")
	writeExcel(t, target, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "Hello")
	})

	patchFile := filepath.Join(tmpDir, "patch.json")
	os.WriteFile(patchFile, []byte(`[{"sheet":"Sheet1","cell":"A1","old_value":"Hello","new_value":"Patched"}]`), 0644)

	out, err, exit := runCLI("excel", "patch", target, "-p", patchFile)
	if err != nil || exit != nil {
		t.Fatalf("excel patch failed: err=%v exit=%v", err, exit)
	}

	for _, want := range []string{
		"Excel Patch Result:",
		"Target File: " + target,
		"Backup File: ",
		"Dry Run: false",
		"Total Patches: 1",
		"Applied Count: 1",
		"Audit Log:",
		"Status=success",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("patch output missing %q:\n%s", want, out)
		}
	}

	// The JSON branch of the same command.
	patchBack := filepath.Join(tmpDir, "patch_back.json")
	os.WriteFile(patchBack, []byte(`[{"sheet":"Sheet1","cell":"A1","old_value":"Patched","new_value":"Hello"}]`), 0644)

	out, err, exit = runCLI("excel", "patch", target, "-p", patchBack, "--dry-run", "--json")
	if err != nil || exit != nil {
		t.Fatalf("excel patch --json failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, `"target_file"`) || !strings.Contains(out, `"dry_run": true`) {
		t.Errorf("unexpected patch json output: %s", out)
	}
}

func TestExcelExtractMarkdownTextOutput(t *testing.T) {
	tmpDir := t.TempDir()

	input := filepath.Join(tmpDir, "input.xlsx")
	writeExcel(t, input, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "Header")
		f.SetCellValue("Sheet1", "A2", "Body")
	})

	// Without --output the markdown goes straight to stdout.
	out, err, exit := runCLI("excel", "extract-markdown", input)
	if err != nil || exit != nil {
		t.Fatalf("excel extract-markdown failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "Header") {
		t.Errorf("expected markdown content on stdout, got:\n%s", out)
	}

	// With --output only a confirmation line is printed.
	mdPath := filepath.Join(tmpDir, "out.md")
	out, err, exit = runCLI("excel", "extract-markdown", input, "-o", mdPath)
	if err != nil || exit != nil {
		t.Fatalf("excel extract-markdown -o failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "Markdown saved to "+mdPath) {
		t.Errorf("expected the saved-to message, got:\n%s", out)
	}
	if _, statErr := os.Stat(mdPath); statErr != nil {
		t.Errorf("expected %s to be written: %v", mdPath, statErr)
	}

	// The extract-text alias shares the same Run function.
	out, err, exit = runCLI("excel", "extract-text", input, "--with-coords")
	if err != nil || exit != nil {
		t.Fatalf("excel extract-text failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "Header") {
		t.Errorf("expected markdown content from the alias, got:\n%s", out)
	}
}

func TestExcelExtractMarkdownErrors(t *testing.T) {
	tmpDir := t.TempDir()

	input := filepath.Join(tmpDir, "input.xlsx")
	writeExcel(t, input, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "Header")
	})

	_, _, exit := runCLI("excel", "extract-markdown", filepath.Join(tmpDir, "missing.xlsx"))
	if exit == nil {
		t.Fatal("expected extract-markdown to exit for a missing input file")
	}

	// Writing into a directory that does not exist fails.
	badOut := filepath.Join(tmpDir, "no_such_dir", "out.md")
	_, _, exit = runCLI("excel", "extract-markdown", input, "-o", badOut)
	if exit == nil {
		t.Fatal("expected extract-markdown to exit when the output file cannot be written")
	}
	if !strings.Contains(exit.err.Error(), "failed to save output file") {
		t.Errorf("unexpected write error: %v", exit.err)
	}
}

func TestExcelSearchCellTextOutput(t *testing.T) {
	tmpDir := t.TempDir()

	input := filepath.Join(tmpDir, "input.xlsx")
	writeExcel(t, input, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "needle plain")
		f.SetCellValue("Sheet1", "C1", "needle merged")
		f.MergeCell("Sheet1", "C1", "D1")
	})

	out, err, exit := runCLI("excel", "search-cell", input, "needle")
	if err != nil || exit != nil {
		t.Fatalf("excel search-cell failed: err=%v exit=%v", err, exit)
	}

	for _, want := range []string{
		"Excel Cell Search Result:",
		"Query: needle",
		"Total Matches: 2",
		"Matches:",
		"Cell A1: 'needle plain'",
		"(Merged: C1:D1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("search output missing %q:\n%s", want, out)
		}
	}

	out, err, exit = runCLI("excel", "search-cell", input, "haystack")
	if err != nil || exit != nil {
		t.Fatalf("excel search-cell (no match) failed: err=%v exit=%v", err, exit)
	}
	if !strings.Contains(out, "No matches found.") {
		t.Errorf("expected the no-match message, got:\n%s", out)
	}
}

func TestExcelSearchCellError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.xlsx")

	_, _, exit := runCLI("excel", "search-cell", missing, "needle")
	if exit == nil {
		t.Fatal("expected search-cell to exit with an error for a missing file")
	}
}
