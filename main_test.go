package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/crunch/internal/db"
	"github.com/taigrr/crunch/internal/llm"
)

func TestInitialModel(t *testing.T) {
	targetDate := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	model := initialModel(targetDate, llm.ProviderOpenAI)

	if model.phase != phaseScanning {
		t.Fatalf("expected phaseScanning, got %v", model.phase)
	}
	if !model.targetDate.Equal(targetDate) {
		t.Fatalf("expected target date %s, got %s", targetDate, model.targetDate)
	}
	if model.provider != llm.ProviderOpenAI {
		t.Fatalf("expected provider %s, got %s", llm.ProviderOpenAI, model.provider)
	}
	if model.progressCh == nil {
		t.Fatal("expected progress channel to be initialized")
	}
}

func TestUpdateCollectDoneNoMessages(t *testing.T) {
	targetDate := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	model := initialModel(targetDate, "")

	updated, cmd := updateModel(t, model, collectDoneMsg{})

	if updated.phase != phaseDone {
		t.Fatalf("expected phaseDone, got %v", updated.phase)
	}
	if updated.summary != "No user messages found for 2026-07-21" {
		t.Fatalf("unexpected summary: %q", updated.summary)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestUpdateCollectDoneWithMessagesInitializesClient(t *testing.T) {
	model := initialModel(time.Now(), "")
	messages := []db.UserMessage{{Text: "worked on tests"}}

	updated, cmd := updateModel(t, model, collectDoneMsg{messages: messages})

	if updated.phase != phaseInitializing {
		t.Fatalf("expected phaseInitializing, got %v", updated.phase)
	}
	if updated.msgCount != len(messages) {
		t.Fatalf("expected message count %d, got %d", len(messages), updated.msgCount)
	}
	if cmd == nil {
		t.Fatal("expected init client command")
	}
}

func TestUpdatePropagatesErrors(t *testing.T) {
	expectedErr := errors.New("scan failed")
	model := initialModel(time.Now(), "")

	updated, cmd := updateModel(t, model, scanDoneMsg{err: expectedErr})

	if updated.phase != phaseDone {
		t.Fatalf("expected phaseDone, got %v", updated.phase)
	}
	if !errors.Is(updated.err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, updated.err)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestViewDoneStates(t *testing.T) {
	t.Run("summary", func(t *testing.T) {
		model := initialModel(time.Now(), "")
		model.phase = phaseDone
		model.summary = "daily summary"

		view := model.View().Content
		if view != "daily summary\n" {
			t.Fatalf("unexpected view: %q", view)
		}
	})

	t.Run("error", func(t *testing.T) {
		model := initialModel(time.Now(), "")
		model.phase = phaseDone
		model.err = errors.New("failed")

		view := model.View().Content
		if !strings.Contains(view, "Error: failed") {
			t.Fatalf("expected error in view, got %q", view)
		}
	})
}

func TestViewSummarizingIncludesProgress(t *testing.T) {
	model := initialModel(time.Now(), "")
	model.phase = phaseSummarizing
	model.msgCount = 3

	view := model.View().Content
	if !strings.Contains(view, "Generating summary") || !strings.Contains(view, "[3 messages]") {
		t.Fatalf("expected summarizing message count, got %q", view)
	}

	model.streamChars = 120
	view = model.View().Content
	if !strings.Contains(view, "Generating summary") || !strings.Contains(view, "[120 chars]") {
		t.Fatalf("expected summarizing character count, got %q", view)
	}
}

func TestUpdateTracksScanAndCollectionProgress(t *testing.T) {
	model := initialModel(time.Now(), "")

	updated, cmd := updateModel(t, model, scanProgressMsg{count: 2})
	if updated.scanCount != 2 {
		t.Fatalf("expected scan count 2, got %d", updated.scanCount)
	}
	if cmd == nil {
		t.Fatal("expected progress listener command")
	}

	updated, cmd = updateModel(t, updated, scanDoneMsg{dbFiles: []string{"one.db", "two.db"}})
	if updated.phase != phaseCollecting {
		t.Fatalf("expected phaseCollecting, got %v", updated.phase)
	}
	if updated.dbTotal != 2 {
		t.Fatalf("expected db total 2, got %d", updated.dbTotal)
	}
	if cmd == nil {
		t.Fatal("expected collection command")
	}

	updated, cmd = updateModel(t, updated, collectProgressMsg{processed: 1, total: 2})
	if updated.dbProcessed != 1 || updated.dbTotal != 2 {
		t.Fatalf("expected collection progress 1/2, got %d/%d", updated.dbProcessed, updated.dbTotal)
	}
	if cmd == nil {
		t.Fatal("expected progress listener command")
	}
}

func TestUpdateStreamFinishUsesActualUsageInView(t *testing.T) {
	model := initialModel(time.Now(), "")
	model.phase = phaseSummarizing
	model.streamChars = 120

	updated, cmd := updateModel(t, model, streamFinishMsg{
		usage: llm.Usage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			Cost:         0.0042,
		},
	})

	if updated.inputTokens != 100 {
		t.Fatalf("expected input tokens 100, got %d", updated.inputTokens)
	}
	if updated.outputTokens != 50 {
		t.Fatalf("expected output tokens 50, got %d", updated.outputTokens)
	}
	if updated.finalCost != 0.0042 {
		t.Fatalf("expected final cost 0.0042, got %f", updated.finalCost)
	}
	if cmd == nil {
		t.Fatal("expected progress listener command")
	}

	view := updated.View().Content
	if !strings.Contains(view, "~$0.0042") {
		t.Fatalf("expected actual cost in view, got %q", view)
	}
}

func updateModel(t *testing.T, currentModel model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()

	updated, cmd := currentModel.Update(msg)
	typed, ok := updated.(model)
	if !ok {
		t.Fatalf("expected model, got %T", updated)
	}
	return typed, cmd
}
