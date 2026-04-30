package db

import (
	"testing"
)

func TestExtractProject(t *testing.T) {
	const fakeHome = "/home/testuser"

	tests := []struct {
		name     string
		dbPath   string
		baseDir  string
		expected string
	}{
		{
			name:     "standard project path",
			dbPath:   "/home/testuser/code/foss/crunch/.crush/crush.db",
			baseDir:  "",
			expected: "foss/crunch",
		},
		{
			name:     "deep project path",
			dbPath:   "/home/testuser/code/contracting/client/project/sub/.crush/crush.db",
			baseDir:  "",
			expected: "contracting/client/project",
		},
		{
			name:     "short path",
			dbPath:   "/home/testuser/code/myproj/.crush/crush.db",
			baseDir:  "",
			expected: "myproj",
		},
		{
			name:     "with custom base dir",
			dbPath:   "/home/testuser/code/foss/crunch/.crush/crush.db",
			baseDir:  "/home/testuser/code",
			expected: "foss/crunch",
		},
		{
			name:     "base dir with trailing slash",
			dbPath:   "/home/testuser/work/project/.crush/crush.db",
			baseDir:  "/home/testuser/work/",
			expected: "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractProjectWithHome(tc.dbPath, tc.baseDir, fakeHome)
			if got != tc.expected {
				t.Errorf("extractProjectWithHome(%q, %q, %q) = %q, want %q", tc.dbPath, tc.baseDir, fakeHome, got, tc.expected)
			}
		})
	}
}
