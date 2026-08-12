package summarizer

import (
	"strings"
	"testing"
	"time"

	"github.com/taigrr/crunch/internal/db"
)

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "short text unchanged",
			text:     "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			text:     "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long text truncated",
			text:     "hello world",
			maxLen:   5,
			expected: "hello...",
		},
		{
			name:     "zero length returns ellipsis",
			text:     "hello",
			maxLen:   0,
			expected: "...",
		},
		{
			name:     "negative length returns ellipsis",
			text:     "hello",
			maxLen:   -1,
			expected: "...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateText(tc.text, tc.maxLen)
			if got != tc.expected {
				t.Errorf("TruncateText(%q, %d) = %q, want %q", tc.text, tc.maxLen, got, tc.expected)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	targetDate := time.Date(2026, 4, 29, 0, 0, 0, 0, time.Local)

	messages := []db.UserMessage{
		{
			Timestamp: time.Date(2026, 4, 29, 10, 30, 0, 0, time.Local),
			Text:      "fix the login bug",
			Project:   "webapp",
		},
		{
			Timestamp: time.Date(2026, 4, 29, 14, 0, 0, 0, time.Local),
			Text:      "add unit tests",
			Project:   "webapp",
		},
		{
			Timestamp: time.Date(2026, 4, 29, 16, 0, 0, 0, time.Local),
			Text:      "refactor database layer",
			Project:   "api-server",
		},
	}

	prompt := BuildPrompt(messages, targetDate)

	if !strings.Contains(prompt, "Wednesday, April 29, 2026") {
		t.Error("prompt should contain formatted date")
	}

	if !strings.Contains(prompt, "## Project: webapp") {
		t.Error("prompt should contain webapp project header")
	}

	if !strings.Contains(prompt, "## Project: api-server") {
		t.Error("prompt should contain api-server project header")
	}

	if !strings.Contains(prompt, "fix the login bug") {
		t.Error("prompt should contain message text")
	}

	if !strings.Contains(prompt, "[10:30]") {
		t.Error("prompt should contain timestamp")
	}
}

func TestBuildPrompt_SortsMessagesWithinProjectByTimestamp(t *testing.T) {
	targetDate := time.Date(2026, 4, 29, 0, 0, 0, 0, time.Local)

	messages := []db.UserMessage{
		{
			Timestamp: time.Date(2026, 4, 29, 15, 0, 0, 0, time.Local),
			Text:      "ship the final fix",
			Project:   "webapp",
		},
		{
			Timestamp: time.Date(2026, 4, 29, 9, 0, 0, 0, time.Local),
			Text:      "start investigating",
			Project:   "webapp",
		},
		{
			Timestamp: time.Date(2026, 4, 29, 12, 0, 0, 0, time.Local),
			Text:      "narrow down the bug",
			Project:   "webapp",
		},
	}

	prompt := BuildPrompt(messages, targetDate)

	first := strings.Index(prompt, "start investigating")
	second := strings.Index(prompt, "narrow down the bug")
	third := strings.Index(prompt, "ship the final fix")

	if first == -1 || second == -1 || third == -1 {
		t.Fatalf("prompt missing expected messages:\n%s", prompt)
	}
	if first > second || second > third {
		t.Fatalf("messages rendered out of timestamp order:\n%s", prompt)
	}
}

func TestBuildPrompt_Truncation(t *testing.T) {
	targetDate := time.Date(2026, 4, 29, 0, 0, 0, 0, time.Local)

	// Create many messages to exceed MaxPromptChars
	var messages []db.UserMessage
	for range 1000 {
		messages = append(messages, db.UserMessage{
			Timestamp: time.Now(),
			Text:      strings.Repeat("x", 2000),
			Project:   "test",
		})
	}

	prompt := BuildPrompt(messages, targetDate)

	if len(prompt) > MaxPromptChars+100 {
		t.Errorf("prompt too long: %d chars (max %d)", len(prompt), MaxPromptChars)
	}

	if !strings.Contains(prompt, "[...truncated due to length...]") {
		t.Error("truncated prompt should contain truncation notice")
	}
}
