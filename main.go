package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	_ "github.com/mattn/go-sqlite3"
)

var skipDirs = map[string]bool{
	"node_modules":     true,
	".git":             true,
	"vendor":           true,
	".venv":            true,
	"venv":             true,
	"__pycache__":      true,
	".cache":           true,
	".npm":             true,
	".pnpm":            true,
	"dist":             true,
	"build":            true,
	".next":            true,
	".nuxt":            true,
	"target":           true,
	"pkg":              true,
	".cargo":           true,
	".rustup":          true,
	"Library":          true,
	"Applications":     true,
	".Trash":           true,
	"Movies":           true,
	"Music":            true,
	"Pictures":         true,
	".local":           true,
	".gradle":          true,
	".m2":              true,
	".cocoapods":       true,
	"Pods":             true,
	".pub-cache":       true,
	".dartServer":      true,
	".docker":          true,
	".orbstack":        true,
	"go":               true, // ~/go typically has pkg/mod
	".ollama":          true,
	".android":         true,
	".gem":             true,
	".bundle":          true,
	".terraform":       true,
	".stack":           true,
	".cabal":           true,
	".ghcup":           true,
	".pyenv":           true,
	".rbenv":           true,
	".nvm":             true,
	".volta":           true,
	".sdkman":          true,
	".asdf":            true,
	".mix":             true,
	".hex":             true,
	"_build":           true,
	"deps":             true,
	"elm-stuff":        true,
	"bower_components": true,
	".bun":             true,
	".antigravity":     true,
	".vscode":          true,
	".zed":             true,
	".config":          true,
	"Downloads":        true,
	"Documents":        true,
	"Desktop":          true,
	".Spotlight-V100":  true,
	".fseventsd":       true,
}

type MessagePart struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type TextData struct {
	Text string `json:"text"`
}

type UserMessage struct {
	Timestamp time.Time
	Text      string
	DBPath    string
}

func main() {
	dateStr := flag.String("date", "", "Date to summarize (YYYY-MM-DD)")
	searchPath := flag.String("path", "", "Path to search (default: home directory)")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	if *dateStr == "" {
		fmt.Fprintln(os.Stderr, "Usage: crunch -date YYYY-MM-DD [-path /search/path] [-v]")
		os.Exit(1)
	}

	targetDate, err := time.ParseInLocation("2006-01-02", *dateStr, time.Local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date format: %v\n", err)
		os.Exit(1)
	}

	root := *searchPath
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get home directory: %v\n", err)
			os.Exit(1)
		}
		root = home
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Searching for crush.db files in %s...\n", root)
	}

	dbFiles := findCrushDBs(root, *verbose)
	if *verbose {
		fmt.Fprintf(os.Stderr, "Found %d crush.db files\n", len(dbFiles))
	}

	messages := collectMessages(dbFiles, targetDate, *verbose)
	if len(messages) == 0 {
		fmt.Printf("No user messages found for %s\n", *dateStr)
		return
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Found %d user messages, summarizing...\n", len(messages))
	}

	summary, err := summarizeWithBedrock(messages, targetDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error summarizing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}

func findCrushDBs(root string, verbose bool) []string {
	var dbFiles []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] {
				if verbose {
					fmt.Fprintf(os.Stderr, "Skipping: %s\n", path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() == "crush.db" {
			dbFiles = append(dbFiles, path)
			if verbose {
				fmt.Fprintf(os.Stderr, "Found: %s\n", path)
			}
		}
		return nil
	})

	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Walk error: %v\n", err)
	}

	return dbFiles
}

func collectMessages(dbFiles []string, targetDate time.Time, verbose bool) []UserMessage {
	var messages []UserMessage

	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.Local)
	endOfDay := startOfDay.Add(24 * time.Hour)

	startUnix := startOfDay.Unix()
	endUnix := endOfDay.Unix()

	for _, dbPath := range dbFiles {
		db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Cannot open %s: %v\n", dbPath, err)
			}
			continue
		}

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
			db.Close()
			if verbose {
				fmt.Fprintf(os.Stderr, "Query error on %s: %v\n", dbPath, err)
			}
			continue
		}

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
					Timestamp: time.Unix(createdAt, 0),
					Text:      text,
					DBPath:    dbPath,
				})
			}
		}
		rows.Close()
		db.Close()
	}

	return messages
}

func summarizeWithBedrock(messages []UserMessage, targetDate time.Time) (string, error) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("loading AWS config: %w", err)
	}

	client := bedrockruntime.New(bedrockruntime.Options{
		Region:      cfg.Region,
		Credentials: cfg.Credentials,
	})

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("You are summarizing a developer's work activities for %s.\n\n", targetDate.Format("Monday, January 2, 2006")))
	promptBuilder.WriteString("Below are all the prompts/requests the user made to their AI coding assistant throughout the day, grouped by project (inferred from the database path).\n\n")
	promptBuilder.WriteString("Summarize what the user worked on, organized by project or theme. Focus on:\n")
	promptBuilder.WriteString("- Main projects/repos touched\n")
	promptBuilder.WriteString("- Key tasks accomplished or attempted\n")
	promptBuilder.WriteString("- Technologies or tools used\n")
	promptBuilder.WriteString("- Any notable patterns (debugging, feature work, refactoring, etc.)\n\n")
	promptBuilder.WriteString("Keep the summary concise but informative, suitable for a daily journal entry. Use bullet points.\n\n")
	promptBuilder.WriteString("---\n\n")

	// Group messages by project (extract from path)
	projectMessages := make(map[string][]UserMessage)
	for _, msg := range messages {
		project := extractProject(msg.DBPath)
		projectMessages[project] = append(projectMessages[project], msg)
	}

	for project, msgs := range projectMessages {
		promptBuilder.WriteString(fmt.Sprintf("## Project: %s\n\n", project))
		for _, msg := range msgs {
			promptBuilder.WriteString(fmt.Sprintf("[%s] %s\n\n", msg.Timestamp.Format("15:04"), truncateText(msg.Text, 500)))
		}
	}

	prompt := promptBuilder.String()

	// Truncate if too long
	if len(prompt) > 180000 {
		prompt = prompt[:180000] + "\n\n[...truncated due to length...]"
	}

	requestBody := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        2048,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	output, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     strPtr("us.anthropic.claude-sonnet-4-20250514-v1:0"),
		ContentType: strPtr("application/json"),
		Accept:      strPtr("application/json"),
		Body:        bodyBytes,
	})
	if err != nil {
		return "", fmt.Errorf("invoking model: %w", err)
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(output.Body, &response); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("empty response from model")
	}

	return response.Content[0].Text, nil
}

func extractProject(dbPath string) string {
	// Extract meaningful project name from path
	// e.g., /Users/tai/code/foss/crunch/.crush/crush.db -> foss/crunch
	// e.g., /Users/tai/code/contracting/mck/code/.crush/crush.db -> contracting/mck/code

	dir := filepath.Dir(dbPath) // Remove crush.db
	dir = filepath.Dir(dir)     // Remove .crush

	home, _ := os.UserHomeDir()
	dir = strings.TrimPrefix(dir, home+"/")
	dir = strings.TrimPrefix(dir, "code/")

	// Limit depth
	parts := strings.Split(dir, "/")
	if len(parts) > 3 {
		parts = parts[:3]
	}

	return strings.Join(parts, "/")
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func strPtr(s string) *string {
	return &s
}
