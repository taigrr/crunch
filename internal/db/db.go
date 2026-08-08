// Package db provides SQLite message extraction from crush.db files.
package db

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// MessagePart represents a part of a message with type and data.
type MessagePart struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// TextData represents the text content of a message part.
type TextData struct {
	Text string `json:"text"`
}

// UserMessage represents a user message extracted from a crush.db.
type UserMessage struct {
	Timestamp time.Time
	Text      string
	DBPath    string
	Project   string
}

// CollectOptions configures message collection.
type CollectOptions struct {
	BaseDir    string
	OnProgress func(processed, total int)
	OnError    func(dbPath string, err error)
}

// CollectMessages extracts user messages from the given database files for the target date.
// If baseDir is provided, it will be used as the base for project path extraction.
func CollectMessages(dbFiles []string, targetDate time.Time, opts *CollectOptions) ([]UserMessage, error) {
	if opts == nil {
		opts = &CollectOptions{}
	}

	var messages []UserMessage

	location := targetDate.Location()
	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, location)
	endOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day()+1, 0, 0, 0, 0, location)

	startUnix := startOfDay.Unix()
	endUnix := endOfDay.Unix()

	total := len(dbFiles)
	for i, dbPath := range dbFiles {
		if opts.OnProgress != nil {
			opts.OnProgress(i+1, total)
		}

		msgs, err := extractFromDB(dbPath, startUnix, endUnix, opts.BaseDir, location)
		if err != nil {
			if opts.OnError != nil {
				opts.OnError(dbPath, err)
			}
			continue
		}
		messages = append(messages, msgs...)
	}

	return messages, nil
}

// readOnlyDSN builds a modernc.org/sqlite DSN that opens dbPath read-only.
// The read-only "mode=ro" flag is only honored when the DSN is a file: URI, so
// the path is resolved to an absolute path and encoded as a proper file URL
// (with forward slashes and percent-encoding) to stay correct for relative
// paths, Windows paths, and paths containing URI special characters such as
// spaces, '#', '?' or '%'.
func readOnlyDSN(dbPath string) string {
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	p := filepath.ToSlash(dbPath)
	if !strings.HasPrefix(p, "/") {
		// Absolute Windows paths (C:/...) need a leading slash to form a
		// valid file URI (file:///C:/...).
		p = "/" + p
	}
	u := url.URL{
		Scheme:   "file",
		Path:     p,
		RawQuery: "mode=ro",
	}
	return u.String()
}

func extractFromDB(dbPath string, startUnix, endUnix int64, baseDir string, location *time.Location) ([]UserMessage, error) {
	db, err := sql.Open("sqlite", readOnlyDSN(dbPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT created_at, parts 
		FROM messages 
		WHERE role = 'user' 
		  AND created_at >= ? 
		  AND created_at < ?
		ORDER BY created_at ASC
	`

	rows, err := db.Query(query, startUnix, endUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []UserMessage
	project := ExtractProject(dbPath, baseDir)

	for rows.Next() {
		var createdAt int64
		var partsJSON string

		if err := rows.Scan(&createdAt, &partsJSON); err != nil {
			continue
		}

		var parts []MessagePart
		if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
			continue
		}

		var textContent strings.Builder
		for _, part := range parts {
			if part.Type == "text" {
				var textData TextData
				if err := json.Unmarshal(part.Data, &textData); err == nil {
					textContent.WriteString(textData.Text)
					textContent.WriteString("\n")
				}
			}
		}

		text := strings.TrimSpace(textContent.String())
		if text != "" {
			messages = append(messages, UserMessage{
				Timestamp: time.Unix(createdAt, 0).In(location),
				Text:      text,
				DBPath:    dbPath,
				Project:   project,
			})
		}
	}

	return messages, rows.Err()
}

// ExtractProject derives a project name from a crush.db path.
// If baseDir is provided, it will be stripped from the path instead of the default prefixes.
func ExtractProject(dbPath string, baseDir string) string {
	home, _ := os.UserHomeDir()
	return extractProjectWithHome(dbPath, baseDir, home)
}

// extractProjectWithHome is the testable implementation of ExtractProject.
func extractProjectWithHome(dbPath string, baseDir string, home string) string {
	dir := filepath.Dir(dbPath)
	dir = filepath.Dir(dir)

	if baseDir != "" {
		baseDir = strings.TrimSuffix(baseDir, "/")
		dir = strings.TrimPrefix(dir, baseDir+"/")
	} else {
		dir = strings.TrimPrefix(dir, home+"/")
		dir = strings.TrimPrefix(dir, "code/")
	}

	parts := strings.Split(dir, "/")
	if len(parts) > 3 {
		parts = parts[:3]
	}

	return strings.Join(parts, "/")
}
