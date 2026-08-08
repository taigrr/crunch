package main

import (
	"context"
	"fmt"
	"os"
	"time"
	_ "time/tzdata" // embed zoneinfo so time.Local is DST-correct on minimal systems

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/taigrr/crunch/internal/db"
	"github.com/taigrr/crunch/internal/llm"
	"github.com/taigrr/crunch/internal/scanner"
	"github.com/taigrr/crunch/internal/summarizer"
	"github.com/taigrr/crunch/internal/version"
)

var (
	dateStr     string
	searchPath  string
	baseDir     string
	verbose     bool
	providerStr string
	modelStr    string
	apiKey      string
)

func main() {
	cmd := &cobra.Command{
		Use:   "crunch",
		Short: "Summarize daily AI coding assistant activity",
		Long:  "Scans for crush.db files and generates a summary of your daily coding activity.",
		RunE:  run,
	}

	cmd.Flags().StringVarP(&dateStr, "date", "d", "", "Date to summarize (YYYY-MM-DD, default: today)")
	cmd.Flags().StringVarP(&searchPath, "path", "p", "", "Path to search (default: home directory)")
	cmd.Flags().StringVar(&baseDir, "dir", "", "Base directory to strip from project paths")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().StringVar(&providerStr, "provider", "", "LLM provider (bedrock, anthropic, openai, openrouter)")
	cmd.Flags().StringVarP(&modelStr, "model", "m", "", "Model to use (provider-specific)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (or use env: ANTHROPIC_API_KEY, OPENAI_API_KEY, OPENROUTER_API_KEY)")

	if err := fang.Execute(context.Background(), cmd, fang.WithVersion(version.Version)); err != nil {
		os.Exit(1)
	}
}

type phase int

const (
	phaseScanning phase = iota
	phaseCollecting
	phaseInitializing
	phaseSummarizing
	phaseDone
)

type model struct {
	spinner      spinner.Model
	phase        phase
	err          error
	summary      string
	targetDate   time.Time
	dbFiles      []string
	messages     []db.UserMessage
	client       *llm.Client
	scanCount    int
	dbProcessed  int
	dbTotal      int
	msgCount     int
	streaming    bool
	streamChars  int
	inputTokens  int64
	outputTokens int64
	finalCost    float64
	provider     llm.Provider
	progressCh   chan tea.Msg
}

type scanProgressMsg struct {
	count int
}

type scanDoneMsg struct {
	dbFiles []string
	err     error
}

type collectProgressMsg struct {
	processed int
	total     int
}

type collectDoneMsg struct {
	messages []db.UserMessage
	err      error
}

type clientReadyMsg struct {
	client *llm.Client
	err    error
}

type streamProgressMsg struct {
	chars        int
	inputTokens  int64
	outputTokens int64
}

type streamFinishMsg struct {
	usage llm.Usage
}

type summaryDoneMsg struct {
	summary string
	err     error
}

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	countStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

func initialModel(targetDate time.Time, provider llm.Provider) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return model{
		spinner:    s,
		phase:      phaseScanning,
		targetDate: targetDate,
		provider:   provider,
		progressCh: make(chan tea.Msg, 100),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.scanCmd(), m.listenProgress())
}

func (m model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		root := searchPath
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return scanDoneMsg{err: err}
			}
			root = home
		}

		var count int
		opts := &scanner.Options{
			OnFound: func(path string) {
				count++
				m.progressCh <- scanProgressMsg{count: count}
				if verbose {
					fmt.Fprintf(os.Stderr, "Found: %s\n", path)
				}
			},
		}

		dbFiles, err := scanner.FindCrushDBs(root, opts)
		return scanDoneMsg{dbFiles: dbFiles, err: err}
	}
}

func (m model) collectCmd() tea.Cmd {
	return func() tea.Msg {
		opts := &db.CollectOptions{
			BaseDir: baseDir,
			OnProgress: func(processed, total int) {
				m.progressCh <- collectProgressMsg{processed: processed, total: total}
			},
			OnError: func(dbPath string, err error) {
				if verbose {
					fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", dbPath, err)
				}
			},
		}
		messages, err := db.CollectMessages(m.dbFiles, m.targetDate, opts)
		return collectDoneMsg{messages: messages, err: err}
	}
}

func (m model) listenProgress() tea.Cmd {
	return func() tea.Msg {
		return <-m.progressCh
	}
}

func (m model) initClientCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		opts := &llm.Options{
			Provider: m.provider,
			Model:    modelStr,
			APIKey:   apiKey,
		}
		client, err := llm.NewClient(ctx, opts)
		return clientReadyMsg{client: client, err: err}
	}
}

func (m model) summarizeStreamCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		prompt := summarizer.BuildPrompt(m.messages, m.targetDate)

		// Estimate input tokens from prompt length (~4 chars per token)
		inputTokens := int64(len(prompt) / 4)

		var totalChars int
		summary, err := m.client.InvokeStream(ctx, llm.StreamCall{
			Prompt: prompt,
			OnTextDelta: func(text string) error {
				totalChars += len(text)
				// Estimate output tokens from chars (~4 chars per token)
				outputTokens := int64(totalChars / 4)
				m.progressCh <- streamProgressMsg{
					chars:        totalChars,
					inputTokens:  inputTokens,
					outputTokens: outputTokens,
				}
				return nil
			},
			OnStreamFinish: func(usage llm.Usage) error {
				m.progressCh <- streamFinishMsg{usage: usage}
				return nil
			},
		})
		return summaryDoneMsg{summary: summary, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanProgressMsg:
		m.scanCount = msg.count
		return m, m.listenProgress()

	case scanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.dbFiles = msg.dbFiles
		m.dbTotal = len(msg.dbFiles)
		m.phase = phaseCollecting
		return m, tea.Batch(m.collectCmd(), m.listenProgress())

	case collectProgressMsg:
		m.dbProcessed = msg.processed
		m.dbTotal = msg.total
		return m, m.listenProgress()

	case collectDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.messages = msg.messages
		m.msgCount = len(msg.messages)
		if m.msgCount == 0 {
			m.summary = fmt.Sprintf("No user messages found for %s", m.targetDate.Format("2006-01-02"))
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.phase = phaseInitializing
		return m, m.initClientCmd()

	case clientReadyMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.client = msg.client
		m.phase = phaseSummarizing
		m.streaming = true
		return m, tea.Batch(m.summarizeStreamCmd(), m.listenProgress())

	case streamProgressMsg:
		m.streamChars = msg.chars
		m.inputTokens = msg.inputTokens
		m.outputTokens = msg.outputTokens
		return m, m.listenProgress()

	case streamFinishMsg:
		// Update with actual token counts and cost from API
		m.inputTokens = msg.usage.InputTokens
		m.outputTokens = msg.usage.OutputTokens
		m.finalCost = msg.usage.Cost
		return m, m.listenProgress()

	case summaryDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseDone
			return m, tea.Quit
		}
		m.summary = msg.summary
		m.phase = phaseDone
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.phase == phaseDone {
		if m.err != nil {
			return tea.NewView(fmt.Sprintf("Error: %v\n", m.err))
		}
		return tea.NewView(m.summary + "\n")
	}

	var status string
	switch m.phase {
	case phaseScanning:
		if m.scanCount == 0 {
			status = "Scanning for crush.db files..."
		} else {
			status = fmt.Sprintf("Scanning... %s found",
				countStyle.Render(fmt.Sprintf("%d", m.scanCount)))
		}
	case phaseCollecting:
		status = fmt.Sprintf("Collecting messages %s",
			countStyle.Render(fmt.Sprintf("[%d/%d]", m.dbProcessed, m.dbTotal)))
	case phaseInitializing:
		status = fmt.Sprintf("Found %s messages, initializing %s...",
			countStyle.Render(fmt.Sprintf("%d", m.msgCount)),
			dimStyle.Render(string(m.provider)))
	case phaseSummarizing:
		if m.streamChars > 0 {
			var cost float64
			if m.finalCost > 0 {
				// Use actual cost from API
				cost = m.finalCost
			} else if m.client != nil {
				// Estimate cost from token counts
				cost = m.client.CalculateCost(m.inputTokens, m.outputTokens)
			}
			costStr := ""
			if cost > 0 {
				costStr = fmt.Sprintf(" ~$%.4f", cost)
			}
			status = fmt.Sprintf("Generating summary %s%s",
				countStyle.Render(fmt.Sprintf("[%d chars]", m.streamChars)),
				dimStyle.Render(costStr))
		} else {
			status = fmt.Sprintf("Generating summary %s",
				dimStyle.Render(fmt.Sprintf("[%d messages]", m.msgCount)))
		}
	}

	return tea.NewView(fmt.Sprintf("%s %s\n", m.spinner.View(), status))
}

func run(cmd *cobra.Command, args []string) error {
	var targetDate time.Time
	var err error

	if dateStr == "" {
		targetDate = time.Now()
	} else {
		targetDate, err = time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			return fmt.Errorf("invalid date format: %w", err)
		}
	}

	var provider llm.Provider
	if providerStr != "" {
		provider, err = llm.ParseProvider(providerStr)
		if err != nil {
			return err
		}
	}

	p := tea.NewProgram(initialModel(targetDate, provider))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m := finalModel.(model)
	if m.err != nil {
		return m.err
	}

	return nil
}
