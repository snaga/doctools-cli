package mcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// parsePageSelection parses printer-style page selection strings (e.g., "1", "2-4", "1,3,5", "")
// and returns sorted, deduplicated 1-based page indices and any out-of-range pages that were skipped.
// If totalPages <= 0, it returns an error.
// If s is empty or whitespace only, it returns all pages from 1 to totalPages.
// If no requested pages fall within 1..totalPages, it returns an error explaining the valid range.
// If some pages fall outside 1..totalPages, they are collected in skipped, and valid pages are returned.
func parsePageSelection(s string, totalPages int) (pages []int, skipped []int, err error) {
	if totalPages <= 0 {
		return nil, nil, fmt.Errorf("totalPages must be greater than 0, got %d", totalPages)
	}

	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		allPages := make([]int, totalPages)
		for i := 1; i <= totalPages; i++ {
			allPages[i-1] = i
		}
		return allPages, nil, nil
	}

	validMap := make(map[int]bool)
	skippedMap := make(map[int]bool)

	tokens := strings.Split(trimmed, ",")
	hasTokens := false
	for _, token := range tokens {
		t := strings.TrimSpace(token)
		if t == "" {
			continue
		}
		hasTokens = true

		if strings.Contains(t, "-") {
			parts := strings.Split(t, "-")
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
				return nil, nil, fmt.Errorf("invalid range format: %s", t)
			}
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, nil, fmt.Errorf("invalid page number: %w", err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, nil, fmt.Errorf("invalid page number: %w", err)
			}
			if start > end {
				start, end = end, start
			}
			// Extract valid pages within [1, totalPages]
			vStart := start
			if vStart < 1 {
				vStart = 1
			}
			vEnd := end
			if vEnd > totalPages {
				vEnd = totalPages
			}
			if vStart <= vEnd {
				for p := vStart; p <= vEnd; p++ {
					validMap[p] = true
				}
			}
			// Record skipped pages with a cap to avoid memory explosion on huge ranges (e.g. 1-10000000)
			const maxSkippedCap = 100
			for p := start; p <= end && len(skippedMap) < maxSkippedCap; p++ {
				if p < 1 || p > totalPages {
					skippedMap[p] = true
				}
			}
		} else {
			num, err := strconv.Atoi(t)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid page number: %w", err)
			}
			if 1 <= num && num <= totalPages {
				validMap[num] = true
			} else {
				skippedMap[num] = true
			}
		}
	}

	if !hasTokens || len(validMap) == 0 {
		return nil, nil, fmt.Errorf("no valid pages in range 1-%d (requested: %s)", totalPages, s)
	}

	pages = make([]int, 0, len(validMap))
	for p := range validMap {
		pages = append(pages, p)
	}
	sort.Ints(pages)

	if len(skippedMap) > 0 {
		skipped = make([]int, 0, len(skippedMap))
		for p := range skippedMap {
			skipped = append(skipped, p)
		}
		sort.Ints(skipped)
	}

	return pages, skipped, nil
}
