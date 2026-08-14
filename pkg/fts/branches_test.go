package fts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doctools-cli/pkg/fts"
)

// TestBuildIndexWithOptions_IndexCreationFails points the index path below a
// regular file, so the directory the index needs can never be created. Both
// the Force and the non-Force branch have to report the failure.
func TestBuildIndexWithOptions_IndexCreationFails(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "doc.txt"), []byte("content"), 0644)

	blocker := filepath.Join(tmpDir, "blocker.txt")
	os.WriteFile(blocker, []byte("not a directory"), 0644)
	indexPath := filepath.Join(blocker, "index.bleve")

	for _, force := range []bool{true, false} {
		_, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
			IndexPath:   indexPath,
			Force:       force,
			IncludeExts: []string{"txt"},
		})
		if err == nil {
			t.Fatalf("expected index creation to fail (force=%t)", force)
		}
		if !strings.Contains(err.Error(), "failed to create bleve index") {
			t.Errorf("unexpected error (force=%t): %v", force, err)
		}
	}
}

// TestBuildIndexWithOptions_FreshBuildWithoutForce covers the branch that
// creates a brand new index when Force is off, and the extension-less file
// skip inside the walk function.
func TestBuildIndexWithOptions_FreshBuildWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "doc.txt"), []byte("fresh content"), 0644)
	// No extension at all: normalizeExt returns "" and the file is ignored.
	os.WriteFile(filepath.Join(sourceDir, "LICENSE"), []byte("license text"), 0644)

	res, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   filepath.Join(tmpDir, "fresh.bleve"),
		Force:       false,
		IncludeExts: []string{"txt"},
	})
	if err != nil {
		t.Fatalf("BuildIndexWithOptions failed: %v", err)
	}
	if res.IndexedFiles != 1 {
		t.Errorf("expected only doc.txt to be indexed, got %d files", res.IndexedFiles)
	}
}

// TestBuildIndexWithOptions_VerboseSkip covers the verbose logging of an
// unchanged file during a differential build.
func TestBuildIndexWithOptions_VerboseSkip(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "stable.txt"), []byte("unchanged content"), 0644)

	opts := fts.BuildOptions{
		IndexPath:   filepath.Join(tmpDir, "verbose_skip.bleve"),
		Force:       false,
		Verbose:     true,
		IncludeExts: []string{"txt"},
	}

	if _, err := fts.BuildIndexWithOptions(sourceDir, opts); err != nil {
		t.Fatalf("initial build failed: %v", err)
	}

	res, err := fts.BuildIndexWithOptions(sourceDir, opts)
	if err != nil {
		t.Fatalf("differential build failed: %v", err)
	}
	if res.SkippedFiles != 1 {
		t.Errorf("expected the unchanged file to be skipped, got %d", res.SkippedFiles)
	}
}

// TestQueryIndex_SnippetWithoutFragments searches a field other than content,
// so Bleve produces no content fragment and QueryIndex has to fall back to the
// stored content: truncated past 200 runes, used as-is below that.
func TestQueryIndex_SnippetWithoutFragments(t *testing.T) {
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	shortContent := "alpha beta gamma"
	longContent := strings.Repeat("delta ", 100) // well past 200 runes

	os.WriteFile(filepath.Join(sourceDir, "short.txt"), []byte(shortContent), 0644)
	os.WriteFile(filepath.Join(sourceDir, "long.txt"), []byte(longContent), 0644)

	indexPath := filepath.Join(tmpDir, "snippet.bleve")
	if _, err := fts.BuildIndexWithOptions(sourceDir, fts.BuildOptions{
		IndexPath:   indexPath,
		Force:       true,
		IncludeExts: []string{"txt"},
	}); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	res, err := fts.QueryIndex(indexPath, "file_type:txt", 10)
	if err != nil {
		t.Fatalf("QueryIndex failed: %v", err)
	}
	if res.TotalHits != 2 {
		t.Fatalf("expected 2 hits, got %d", res.TotalHits)
	}

	sawTruncated := false
	sawFull := false
	for _, hit := range res.Hits {
		switch hit.Source.FileName {
		case "long.txt":
			if !strings.HasSuffix(hit.Snippet, "...") {
				t.Errorf("expected the long snippet to be truncated, got %q", hit.Snippet)
			}
			if len([]rune(hit.Snippet)) != 203 {
				t.Errorf("expected 200 runes plus the ellipsis, got %d", len([]rune(hit.Snippet)))
			}
			sawTruncated = true
		case "short.txt":
			if hit.Snippet != shortContent {
				t.Errorf("expected the short content verbatim, got %q", hit.Snippet)
			}
			sawFull = true
		}
	}

	if !sawTruncated || !sawFull {
		t.Errorf("missing hits: truncated=%t full=%t", sawTruncated, sawFull)
	}
}
