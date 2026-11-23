package mcp

import (
	"reflect"
	"testing"
)

func TestParsePageSelection(t *testing.T) {
	tests := []struct {
		name        string
		s           string
		totalPages  int
		wantPages   []int
		wantSkipped []int
		wantErr     bool
	}{
		{
			name:        "empty string returns all pages",
			s:           "",
			totalPages:  5,
			wantPages:   []int{1, 2, 3, 4, 5},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "whitespace only returns all pages",
			s:           "   ",
			totalPages:  3,
			wantPages:   []int{1, 2, 3},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "single page within range",
			s:           "3",
			totalPages:  5,
			wantPages:   []int{3},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "multiple pages comma separated",
			s:           "1, 3, 5",
			totalPages:  5,
			wantPages:   []int{1, 3, 5},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "range specification",
			s:           "2-4",
			totalPages:  5,
			wantPages:   []int{2, 3, 4},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "reverse range specification",
			s:           "4-2",
			totalPages:  5,
			wantPages:   []int{2, 3, 4},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "mixed and duplicates with spaces",
			s:           " 1, 2-3, 2, 4-5, 3 ",
			totalPages:  5,
			wantPages:   []int{1, 2, 3, 4, 5},
			wantSkipped: nil,
			wantErr:     false,
		},
		{
			name:        "partial out of range",
			s:           "1, 3, 8",
			totalPages:  5,
			wantPages:   []int{1, 3},
			wantSkipped: []int{8},
			wantErr:     false,
		},
		{
			name:        "partial non-positive and zero",
			s:           "0, 2, 3",
			totalPages:  5,
			wantPages:   []int{2, 3},
			wantSkipped: []int{0},
			wantErr:     false,
		},
		{
			name:        "all out of range returns error",
			s:           "8, 9, 10",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid syntax non-numeric",
			s:           "abc",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid hyphen syntax multiple hyphens",
			s:           "1-2-3",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid hyphen syntax missing end",
			s:           "1-",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid hyphen syntax missing start",
			s:           "-5",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid totalPages zero",
			s:           "1",
			totalPages:  0,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "invalid totalPages negative",
			s:           "1",
			totalPages:  -1,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
		{
			name:        "comma only with no numbers returns error",
			s:           ",,,",
			totalPages:  5,
			wantPages:   nil,
			wantSkipped: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPages, gotSkipped, err := parsePageSelection(tt.s, tt.totalPages)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePageSelection(%q, %d) error = %v, wantErr %v", tt.s, tt.totalPages, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(gotPages, tt.wantPages) {
				t.Errorf("parsePageSelection(%q, %d) gotPages = %v, want %v", tt.s, tt.totalPages, gotPages, tt.wantPages)
			}
			if !reflect.DeepEqual(gotSkipped, tt.wantSkipped) {
				t.Errorf("parsePageSelection(%q, %d) gotSkipped = %v, want %v", tt.s, tt.totalPages, gotSkipped, tt.wantSkipped)
			}
		})
	}
}
