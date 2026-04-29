package db

import (
	"testing"
)

func TestExtractProject(t *testing.T) {
	tests := []struct {
		name     string
		dbPath   string
		baseDir  string
		expected string
	}{
		{
			name:     "standard project path",
			dbPath:   "/Users/tai/code/foss/crunch/.crush/crush.db",
			baseDir:  "",
			expected: "foss/crunch",
		},
		{
			name:     "deep project path",
			dbPath:   "/Users/tai/code/contracting/client/project/sub/.crush/crush.db",
			baseDir:  "",
			expected: "contracting/client/project",
		},
		{
			name:     "short path",
			dbPath:   "/Users/tai/code/myproj/.crush/crush.db",
			baseDir:  "",
			expected: "myproj",
		},
		{
			name:     "with custom base dir",
			dbPath:   "/Users/tai/code/foss/crunch/.crush/crush.db",
			baseDir:  "/Users/tai/code",
			expected: "foss/crunch",
		},
		{
			name:     "base dir with trailing slash",
			dbPath:   "/Users/tai/work/project/.crush/crush.db",
			baseDir:  "/Users/tai/work/",
			expected: "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractProject(tc.dbPath, tc.baseDir)
			if got != tc.expected {
				t.Errorf("ExtractProject(%q, %q) = %q, want %q", tc.dbPath, tc.baseDir, got, tc.expected)
			}
		})
	}
}
