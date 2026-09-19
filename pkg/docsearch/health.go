package docsearch

import (
	"fmt"
	"os"
)

// MissingIndexWarningText is the warning text displayed when no valid indexes are registered.
const MissingIndexWarningText = "⚠️ 有効なインデックスがありません。[＋ 追加] から登録してください"

// FormatMissingIndexLabel formats the label for an index whose target directory cannot be found.
func FormatMissingIndexLabel(name string) string {
	return fmt.Sprintf("⚠️ %s (見つかりません)", name)
}

// IndexHealthStatus represents the health/existence status of a single search index.
type IndexHealthStatus struct {
	Index  IndexConfig `json:"index"`
	Exists bool        `json:"exists"`
}

// CheckIndexHealth inspects each index path and verifies whether it exists and is a directory.
func CheckIndexHealth(indexes []IndexConfig) []IndexHealthStatus {
	statuses := make([]IndexHealthStatus, len(indexes))
	for i, idx := range indexes {
		exists := false
		if idx.Path != "" {
			if fi, err := os.Stat(idx.Path); err == nil && fi.IsDir() {
				exists = true
			}
		}
		statuses[i] = IndexHealthStatus{
			Index:  idx,
			Exists: exists,
		}
	}
	return statuses
}

// ValidateIndexes partitions indexes into valid (existing directory) and missing indexes.
func ValidateIndexes(indexes []IndexConfig) (valid []IndexConfig, missing []IndexConfig) {
	valid = make([]IndexConfig, 0, len(indexes))
	missing = make([]IndexConfig, 0, len(indexes))
	statuses := CheckIndexHealth(indexes)
	for _, s := range statuses {
		if s.Exists {
			valid = append(valid, s.Index)
		} else {
			missing = append(missing, s.Index)
		}
	}
	return valid, missing
}
