package docsearch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestAddHistory(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.json")

	// 1. Initial add
	err := AddHistory(histPath, "有給 申請")
	if err != nil {
		t.Fatalf("unexpected error on AddHistory: %v", err)
	}

	hist, err := LoadHistory(histPath)
	if err != nil {
		t.Fatalf("unexpected error on LoadHistory: %v", err)
	}
	if len(hist.History) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(hist.History))
	}
	item := hist.History[0]
	if item.Query != "有給 申請" {
		t.Errorf("expected query '有給 申請', got '%s'", item.Query)
	}
	if item.UseCount != 1 {
		t.Errorf("expected UseCount 1, got %d", item.UseCount)
	}
	if item.LastUsedAt.IsZero() {
		t.Errorf("expected LastUsedAt to be set")
	}

	time.Sleep(10 * time.Millisecond) // slight delay to verify time update

	// 2. Duplicate add increments count and updates timestamp
	err = AddHistory(histPath, "有給 申請")
	if err != nil {
		t.Fatalf("unexpected error on second AddHistory: %v", err)
	}

	hist, err = LoadHistory(histPath)
	if err != nil {
		t.Fatalf("unexpected error on LoadHistory: %v", err)
	}
	if len(hist.History) != 1 {
		t.Fatalf("expected 1 history item after duplicate, got %d", len(hist.History))
	}
	item2 := hist.History[0]
	if item2.UseCount != 2 {
		t.Errorf("expected UseCount 2, got %d", item2.UseCount)
	}
	if !item2.LastUsedAt.After(item.LastUsedAt) {
		t.Errorf("expected LastUsedAt (%v) to be after previous (%v)", item2.LastUsedAt, item.LastUsedAt)
	}

	// 3. Whitespace handling
	err = AddHistory(histPath, "   ")
	if err != nil {
		t.Fatalf("unexpected error on whitespace query: %v", err)
	}
	hist, _ = LoadHistory(histPath)
	if len(hist.History) != 1 {
		t.Errorf("whitespace query should not add item, got %d items", len(hist.History))
	}
}

func TestGetSuggestions(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.json")

	// Non-existent history file returns empty suggestions
	suggs, err := GetSuggestions(histPath, "有給", 10)
	if err != nil {
		t.Fatalf("unexpected error on non-existent file: %v", err)
	}
	if len(suggs) != 0 {
		t.Errorf("expected empty suggestions, got %v", suggs)
	}

	// Seed history items
	// "有給 申請" -> 2 calls
	_ = AddHistory(histPath, "有給 申請")
	_ = AddHistory(histPath, "有給 申請")

	// "有給 残日数" -> 3 calls
	_ = AddHistory(histPath, "有給 残日数")
	_ = AddHistory(histPath, "有給 残日数")
	_ = AddHistory(histPath, "有給 残日数")

	// "年次 有給" -> 1 call (partial match test)
	_ = AddHistory(histPath, "年次 有給")

	// "就業規則" -> 5 calls (unrelated query)
	for i := 0; i < 5; i++ {
		_ = AddHistory(histPath, "就業規則")
	}

	// Test 1: Match by prefix / partial match with UseCount order
	suggs, err = GetSuggestions(histPath, "有給", 10)
	if err != nil {
		t.Fatalf("unexpected error on GetSuggestions: %v", err)
	}
	expected := []string{"有給 残日数", "有給 申請", "年次 有給"}
	if !reflect.DeepEqual(suggs, expected) {
		t.Errorf("expected suggestions %v, got %v", expected, suggs)
	}

	// Test 2: Limit parameter
	suggs, err = GetSuggestions(histPath, "有給", 2)
	if err != nil {
		t.Fatalf("unexpected error on GetSuggestions with limit: %v", err)
	}
	expectedLimit := []string{"有給 残日数", "有給 申請"}
	if !reflect.DeepEqual(suggs, expectedLimit) {
		t.Errorf("expected limit suggestions %v, got %v", expectedLimit, suggs)
	}

	// Test 3: Empty prefix returns all queries sorted by UseCount
	allSuggs, err := GetSuggestions(histPath, "", 10)
	if err != nil {
		t.Fatalf("unexpected error on empty prefix: %v", err)
	}
	expectedAll := []string{"就業規則", "有給 残日数", "有給 申請", "年次 有給"}
	if !reflect.DeepEqual(allSuggs, expectedAll) {
		t.Errorf("expected all suggestions %v, got %v", expectedAll, allSuggs)
	}

	// Test 4: No match
	noMatch, err := GetSuggestions(histPath, "存在しないクエリ", 5)
	if err != nil {
		t.Fatalf("unexpected error on no match: %v", err)
	}
	if len(noMatch) != 0 {
		t.Errorf("expected 0 results, got %v", noMatch)
	}
}

func TestGetSuggestions_SameCountTimestampOrder(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.json")

	now := time.Now().UTC()
	qh := &QueryHistory{
		History: []HistoryItem{
			{Query: "target older", UseCount: 2, LastUsedAt: now.Add(-1 * time.Hour)},
			{Query: "target newer", UseCount: 2, LastUsedAt: now},
		},
	}
	if err := SaveHistory(histPath, qh); err != nil {
		t.Fatalf("failed to seed history: %v", err)
	}

	suggs, err := GetSuggestions(histPath, "target", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"target newer", "target older"}
	if !reflect.DeepEqual(suggs, expected) {
		t.Errorf("expected timestamp sort %v, got %v", expected, suggs)
	}
}

func TestHistory_ErrorAndEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// LoadHistory with empty path
	qh, err := LoadHistory("")
	if err != nil || len(qh.History) != 0 {
		t.Errorf("expected empty history on empty path, got %v, err: %v", qh, err)
	}

	// SaveHistory with empty path
	if err := SaveHistory("", qh); err == nil {
		t.Errorf("expected error saving to empty path")
	}

	// SaveHistory with nil qh
	nilHistPath := filepath.Join(tmpDir, "nil_hist.json")
	if err := SaveHistory(nilHistPath, nil); err != nil {
		t.Fatalf("unexpected error saving nil history: %v", err)
	}
	loadedNil, err := LoadHistory(nilHistPath)
	if err != nil || len(loadedNil.History) != 0 {
		t.Errorf("expected empty history loaded from nil save")
	}

	// LoadHistory invalid JSON
	badHistPath := filepath.Join(tmpDir, "bad_hist.json")
	if err := os.WriteFile(badHistPath, []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to write bad history: %v", err)
	}
	if _, err := LoadHistory(badHistPath); err == nil {
		t.Errorf("expected error loading invalid json history")
	}

	// AddHistory / GetSuggestions with invalid json file
	if err := AddHistory(badHistPath, "test"); err == nil {
		t.Errorf("expected error on AddHistory with corrupted history file")
	}
	if _, err := GetSuggestions(badHistPath, "test", 10); err == nil {
		t.Errorf("expected error on GetSuggestions with corrupted history file")
	}
}
