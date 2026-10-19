package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCollectMessages_FallBackDayBoundaries(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// November 1 has 25 hours, including two distinct occurrences of 01:30.
	targetDate := time.Date(2026, time.November, 1, 12, 0, 0, 0, location)
	dbPath := filepath.Join(t.TempDir(), "repo", ".crush", "crush.db")
	createTestDB(t, dbPath)
	fixtures := []struct {
		utc  string
		text string
		keep bool
	}{
		{"2026-11-01T03:59:59Z", "previous day", false},
		{"2026-11-01T04:00:00Z", "start of day", true},
		{"2026-11-01T05:30:00Z", "first 01:30", true},
		{"2026-11-01T06:30:00Z", "second 01:30", true},
		{"2026-11-02T04:59:59Z", "last second of day", true},
		{"2026-11-02T05:00:00Z", "next day", false},
	}
	var expected []UserMessage
	// Insert backwards to verify extraction orders by timestamp, not insertion.
	for index := len(fixtures) - 1; index >= 0; index-- {
		fixture := fixtures[index]
		stamp, err := time.Parse(time.RFC3339, fixture.utc)
		if err != nil {
			t.Fatal(err)
		}
		insertMessage(t, dbPath, "user", stamp, []MessagePart{
			{Type: "text", Data: mustMarshalRaw(t, TextData{Text: fixture.text})},
		})
		if fixture.keep {
			expected = append([]UserMessage{{Timestamp: stamp, Text: fixture.text}}, expected...)
		}
	}

	messages, err := CollectMessages([]string{dbPath}, targetDate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != len(expected) {
		t.Fatalf("got %d messages, want %d", len(messages), len(expected))
	}
	for index, want := range expected {
		got := messages[index]
		if got.Text != want.Text || !got.Timestamp.Equal(want.Timestamp) {
			t.Errorf("message %d = (%q, %s), want (%q, %s)", index, got.Text, got.Timestamp, want.Text, want.Timestamp)
		}
		if got.Timestamp.Location() != location {
			t.Errorf("message %d location = %v, want %v", index, got.Timestamp.Location(), location)
		}
	}
}
