package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
	_ "time/tzdata"

	_ "modernc.org/sqlite"
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

func TestCollectMessages(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	targetDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.Local)
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 9, 15, 0, 0, time.Local), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "fix collection tests"})},
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "add sqlite fixture"})},
	})
	insertMessage(t, dbPath, "assistant", time.Date(2026, 7, 2, 9, 16, 0, 0, time.Local), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "assistant reply"})},
	})
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 1, 23, 59, 0, 0, time.Local), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "previous day"})},
	})
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 3, 0, 0, 0, 0, time.Local), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "next day"})},
	})

	var progressCalls []int
	messages, err := CollectMessages([]string{dbPath}, targetDate, &CollectOptions{
		BaseDir: tmpDir,
		OnProgress: func(processed, total int) {
			if total != 1 {
				t.Errorf("progress total = %d, want 1", total)
			}
			progressCalls = append(progressCalls, processed)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(progressCalls) != 1 || progressCalls[0] != 1 {
		t.Fatalf("progress calls = %v, want [1]", progressCalls)
	}

	if len(messages) != 1 {
		t.Fatalf("CollectMessages returned %d messages, want 1", len(messages))
	}

	got := messages[0]
	if got.Text != "fix collection tests\nadd sqlite fixture" {
		t.Errorf("message text = %q", got.Text)
	}
	if got.Project != "repo" {
		t.Errorf("project = %q, want repo", got.Project)
	}
	if got.DBPath != dbPath {
		t.Errorf("DBPath = %q, want %q", got.DBPath, dbPath)
	}
}

func TestCollectMessages_UsesTargetDateLocation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	location := time.FixedZone("test-east", 2*60*60)
	targetDate := time.Date(2026, 7, 2, 0, 0, 0, 0, location)

	insertMessage(t, dbPath, "user", time.Date(2026, 7, 1, 21, 59, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "previous local day"})},
	})
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 1, 22, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "start of target local day"})},
	})
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 21, 59, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "end of target local day"})},
	})
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 22, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "next local day"})},
	})

	messages, err := CollectMessages([]string{dbPath}, targetDate, &CollectOptions{
		BaseDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 {
		t.Fatalf("CollectMessages returned %d messages, want 2", len(messages))
	}

	gotTexts := []string{messages[0].Text, messages[1].Text}
	wantTexts := []string{"start of target local day", "end of target local day"}
	for i, want := range wantTexts {
		if gotTexts[i] != want {
			t.Fatalf("message %d text = %q, want %q", i, gotTexts[i], want)
		}
	}
	for i, message := range messages {
		if message.Timestamp.Location() != location {
			t.Fatalf("message %d location = %v, want %v", i, message.Timestamp.Location(), location)
		}
	}
}

func TestCollectMessages_HandlesDSTDayBoundary(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	// 2026-03-08 is a US spring-forward day (23h long): the window must end at
	// the following local midnight, not startOfDay+24h.
	targetDate := time.Date(2026, 3, 8, 0, 0, 0, 0, location)

	insertMessage(t, dbPath, "user", time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "inside target day"})},
	})
	// 04:30 UTC on 2026-03-09 is 00:30 EDT (next local day). It falls before
	// startOfDay+24h (05:00 UTC) but at/after the true next local midnight
	// (04:00 UTC), so it must be excluded.
	insertMessage(t, dbPath, "user", time.Date(2026, 3, 9, 4, 30, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "next local day"})},
	})

	messages, err := CollectMessages([]string{dbPath}, targetDate, &CollectOptions{
		BaseDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 1 {
		t.Fatalf("CollectMessages returned %d messages, want 1", len(messages))
	}
	if messages[0].Text != "inside target day" {
		t.Fatalf("message text = %q, want %q", messages[0].Text, "inside target day")
	}
}

func TestCollectMessages_ReportsUnreadableDatabases(t *testing.T) {
	missingDB := filepath.Join(t.TempDir(), "missing", ".crush", "crush.db")

	var errorPath string
	messages, err := CollectMessages([]string{missingDB}, time.Now(), &CollectOptions{
		OnError: func(dbPath string, err error) {
			errorPath = dbPath
			if err == nil {
				t.Error("OnError received nil error")
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 0 {
		t.Fatalf("CollectMessages returned %d messages, want 0", len(messages))
	}
	if errorPath != missingDB {
		t.Fatalf("OnError path = %q, want %q", errorPath, missingDB)
	}
}

func TestFileURI(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "unix absolute",
			path: "/home/user/proj/.crush/crush.db",
			want: "file:///home/user/proj/.crush/crush.db?mode=ro",
		},
		{
			name: "path with space",
			path: "/home/user/my proj/.crush/crush.db",
			want: "file:///home/user/my%20proj/.crush/crush.db?mode=ro",
		},
		{
			name: "path with hash and percent",
			path: "/home/user/50%off#1/.crush/crush.db",
			want: "file:///home/user/50%25off%231/.crush/crush.db?mode=ro",
		},
		{
			name: "windows drive path",
			path: "C:/Users/me/proj/.crush/crush.db",
			want: "file:///C:/Users/me/proj/.crush/crush.db?mode=ro",
		},
		{
			name: "windows unc path",
			path: "//server/share/proj/.crush/crush.db",
			want: "file://server/share/proj/.crush/crush.db?mode=ro",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// fileURI receives an absolute path already normalized to
			// forward slashes (as filepath.ToSlash would produce on the
			// native OS); feeding that form keeps the test deterministic
			// on any host.
			if got := fileURI(tc.path); got != tc.want {
				t.Fatalf("fileURI(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestReadOnlyDSN_OpensReadOnly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	database, err := sql.Open("sqlite", readOnlyDSN(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Exec("INSERT INTO messages (created_at, role, parts) VALUES (1, 'user', '[]')"); err == nil {
		t.Fatal("expected write to fail on a read-only connection, got nil error")
	}
}

func TestCollectMessages_HandlesWALDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	// Real crush.db files run in WAL mode; enabling it here exercises the
	// read-only open path against a WAL database (needs -wal/-shm access).
	wdb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wdb.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if err := wdb.Close(); err != nil {
		t.Fatal(err)
	}

	targetDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "wal message"})},
	})

	messages, err := CollectMessages([]string{dbPath}, targetDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("CollectMessages returned %d messages, want 1", len(messages))
	}
	if messages[0].Text != "wal message" {
		t.Fatalf("message text = %q, want %q", messages[0].Text, "wal message")
	}
}

func TestCollectMessages_HandlesSpecialCharPaths(t *testing.T) {
	// A directory containing URI-special characters must still open, proving
	// the DSN is properly encoded rather than concatenated.
	dbPath := filepath.Join(t.TempDir(), "my proj #1", ".crush", "crush.db")
	createTestDB(t, dbPath)

	targetDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "special path message"})},
	})

	messages, err := CollectMessages([]string{dbPath}, targetDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("CollectMessages returned %d messages, want 1", len(messages))
	}
	if messages[0].Text != "special path message" {
		t.Fatalf("message text = %q, want %q", messages[0].Text, "special path message")
	}
}

func TestCollectMessages_HandlesRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)

	targetDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	insertMessage(t, dbPath, "user", time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC), []MessagePart{
		{Type: "text", Data: mustMarshalRaw(t, TextData{Text: "relative path message"})},
	})

	t.Chdir(tmpDir)
	relPath := filepath.Join("repo", ".crush", "crush.db")

	messages, err := CollectMessages([]string{relPath}, targetDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("CollectMessages returned %d messages, want 1", len(messages))
	}
	if messages[0].Text != "relative path message" {
		t.Fatalf("message text = %q, want %q", messages[0].Text, "relative path message")
	}
}

func createTestDB(t *testing.T, dbPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE messages (
			created_at INTEGER NOT NULL,
			role TEXT NOT NULL,
			parts TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func insertMessage(t *testing.T, dbPath, role string, createdAt time.Time, parts []MessagePart) {
	t.Helper()

	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	partsJSON, err := json.Marshal(parts)
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.Exec(
		"INSERT INTO messages (created_at, role, parts) VALUES (?, ?, ?)",
		createdAt.Unix(),
		role,
		string(partsJSON),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func mustMarshalRaw(t *testing.T, value TextData) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
