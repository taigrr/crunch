package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

func createTestDB(t *testing.T, dbPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	database, err := sql.Open("sqlite3", dbPath)
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

	database, err := sql.Open("sqlite3", dbPath)
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
