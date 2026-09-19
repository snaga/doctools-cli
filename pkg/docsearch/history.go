package docsearch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var historyMu sync.Mutex

// QueryHistory represents the persistent search query history structure.
type QueryHistory struct {
	History []HistoryItem `json:"history"`
}

// HistoryItem represents a single recorded search query and its usage statistics.
type HistoryItem struct {
	Query      string    `json:"query"`
	LastUsedAt time.Time `json:"last_used_at"`
	UseCount   int       `json:"use_count"`
}

// LoadHistory loads search history from the specified JSON file.
// If the file does not exist, an empty QueryHistory is returned without error.
func LoadHistory(path string) (*QueryHistory, error) {
	qh := &QueryHistory{
		History: make([]HistoryItem, 0),
	}
	if path == "" {
		return qh, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return qh, nil
		}
		return nil, fmt.Errorf("failed to read history file %s: %w", path, err)
	}

	if err := json.Unmarshal(data, qh); err != nil {
		return nil, fmt.Errorf("failed to parse history JSON from %s: %w", path, err)
	}

	if qh.History == nil {
		qh.History = make([]HistoryItem, 0)
	}

	return qh, nil
}

// SaveHistory writes the search history to the specified JSON file.
func SaveHistory(path string, qh *QueryHistory) error {
	if path == "" {
		return fmt.Errorf("history file path cannot be empty")
	}
	if qh == nil {
		qh = &QueryHistory{History: make([]HistoryItem, 0)}
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	data, err := json.MarshalIndent(qh, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file %s: %w", path, err)
	}

	return nil
}

// AddHistory adds or updates a search query in the history file.
// If the query already exists, its usage count is incremented and last used timestamp is refreshed.
func AddHistory(path string, query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	qh, err := LoadHistory(path)
	if err != nil {
		return err
	}

	found := false
	now := time.Now().UTC()
	for i := range qh.History {
		if strings.EqualFold(qh.History[i].Query, trimmed) {
			qh.History[i].UseCount++
			qh.History[i].LastUsedAt = now
			qh.History[i].Query = trimmed
			found = true
			break
		}
	}

	if !found {
		qh.History = append(qh.History, HistoryItem{
			Query:      trimmed,
			LastUsedAt: now,
			UseCount:   1,
		})
	}

	return SaveHistory(path, qh)
}

// GetSuggestions returns search query suggestions matching the given prefix.
// The candidates are filtered by prefix/partial matching (case-insensitive) and
// sorted by usage count descending, followed by last used timestamp descending.
func GetSuggestions(path string, prefix string, limit int) ([]string, error) {
	historyMu.Lock()
	defer historyMu.Unlock()

	qh, err := LoadHistory(path)
	if err != nil {
		return nil, err
	}

	cleanPrefix := strings.TrimSpace(prefix)
	lowerPrefix := strings.ToLower(cleanPrefix)

	matches := make([]HistoryItem, 0, len(qh.History))
	for _, item := range qh.History {
		if cleanPrefix == "" || strings.Contains(strings.ToLower(item.Query), lowerPrefix) {
			matches = append(matches, item)
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].UseCount != matches[j].UseCount {
			return matches[i].UseCount > matches[j].UseCount
		}
		return matches[i].LastUsedAt.After(matches[j].LastUsedAt)
	})

	maxResults := len(matches)
	if limit > 0 && limit < maxResults {
		maxResults = limit
	}

	suggestions := make([]string, 0, maxResults)
	for i := 0; i < maxResults; i++ {
		suggestions = append(suggestions, matches[i].Query)
	}

	return suggestions, nil
}
