// Package summarizer builds prompts for summarizing user activity.
package summarizer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/taigrr/crunch/internal/db"
)

const (
	// MaxPromptChars is the maximum prompt size (~900k chars for 1M token context).
	MaxPromptChars = 900000
)

// BuildPrompt constructs the summarization prompt from messages.
func BuildPrompt(messages []db.UserMessage, targetDate time.Time) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are summarizing a developer's work activities for %s.\n\n", targetDate.Format("Monday, January 2, 2006"))
	b.WriteString("Below are all the prompts/requests the user made to their AI coding assistant throughout the day, grouped by project (inferred from the database path).\n\n")
	b.WriteString("Summarize what the user worked on, organized by project or theme. Focus on:\n")
	b.WriteString("- Main projects/repos touched\n")
	b.WriteString("- Key tasks accomplished or attempted\n")
	b.WriteString("- Technologies or tools used\n")
	b.WriteString("- Any notable patterns (debugging, feature work, refactoring, etc.)\n\n")
	b.WriteString("Keep the summary concise but informative, suitable for a daily journal entry. Use bullet points.\n\n")
	b.WriteString("---\n\n")

	projectMessages := groupByProject(messages)

	// Sort projects for deterministic output
	projects := make([]string, 0, len(projectMessages))
	for p := range projectMessages {
		projects = append(projects, p)
	}
	sort.Strings(projects)

	for _, project := range projects {
		msgs := projectMessages[project]
		sort.SliceStable(msgs, func(i, j int) bool {
			return msgs[i].Timestamp.Before(msgs[j].Timestamp)
		})
		fmt.Fprintf(&b, "## Project: %s\n\n", project)
		for _, msg := range msgs {
			fmt.Fprintf(&b, "[%s] %s\n\n", msg.Timestamp.Format("15:04"), TruncateText(msg.Text, 2000))
		}
	}

	prompt := b.String()

	if len(prompt) > MaxPromptChars {
		prompt = prompt[:MaxPromptChars] + "\n\n[...truncated due to length...]"
	}

	return prompt
}

func groupByProject(messages []db.UserMessage) map[string][]db.UserMessage {
	result := make(map[string][]db.UserMessage)
	for _, msg := range messages {
		result[msg.Project] = append(result[msg.Project], msg)
	}
	return result
}

// TruncateText truncates text to maxLen runes, adding ellipsis if truncated.
func TruncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
