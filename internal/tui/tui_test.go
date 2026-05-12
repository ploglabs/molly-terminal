package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/model"
)

func msg(id string, ts time.Time) model.Message {
	return model.Message{
		ID:        id,
		Username:  "user",
		Content:   "content-" + id,
		Channel:   "general",
		Timestamp: ts,
	}
}

func TestInsertSortedAppendsToEnd(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	msgs := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}

	result := insertSorted(msgs, msg("3", base.Add(2*time.Minute)))
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[2].ID != "3" {
		t.Errorf("expected last message ID '3', got %q", result[2].ID)
	}
}

func TestInsertSortedInsertsInMiddle(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	msgs := []model.Message{msg("1", base), msg("3", base.Add(2*time.Minute))}

	result := insertSorted(msgs, msg("2", base.Add(time.Minute)))
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[1].ID != "2" {
		t.Errorf("expected middle message ID '2', got %q", result[1].ID)
	}
}

func TestInsertSortedDeduplicatesByID(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	msgs := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}

	result := insertSorted(msgs, msg("1", base))
	if len(result) != 2 {
		t.Errorf("expected 2 messages after duplicate insert, got %d", len(result))
	}
}

func TestInsertSortedAtBeginning(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	msgs := []model.Message{msg("2", base.Add(time.Minute)), msg("3", base.Add(2*time.Minute))}

	result := insertSorted(msgs, msg("1", base))
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].ID != "1" {
		t.Errorf("expected first message ID '1', got %q", result[0].ID)
	}
}

func TestInsertSortedEmptySlice(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var msgs []model.Message

	result := insertSorted(msgs, msg("1", base))
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].ID != "1" {
		t.Errorf("expected message ID '1', got %q", result[0].ID)
	}
}

func TestMergeMessagesNoDuplicates(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}
	incoming := []model.Message{msg("3", base.Add(2*time.Minute)), msg("4", base.Add(3*time.Minute))}

	result := mergeMessages(existing, incoming)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	for i := 0; i < len(result)-1; i++ {
		if !result[i].Timestamp.Before(result[i+1].Timestamp) {
			t.Errorf("messages not sorted chronologically at index %d: %v >= %v", i, result[i].Timestamp, result[i+1].Timestamp)
		}
	}
}

func TestMergeMessagesWithDuplicates(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}
	incoming := []model.Message{msg("2", base.Add(time.Minute)), msg("3", base.Add(2*time.Minute))}

	result := mergeMessages(existing, incoming)
	if len(result) != 3 {
		t.Errorf("expected 3 messages after dedup, got %d", len(result))
	}
}

func TestMergeMessagesAllDuplicates(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}
	incoming := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}

	result := mergeMessages(existing, incoming)
	if len(result) != 2 {
		t.Errorf("expected 2 messages when all incoming are duplicates, got %d", len(result))
	}
}

func TestMergeMessagesEmptyExisting(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var existing []model.Message
	incoming := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}

	result := mergeMessages(existing, incoming)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

func TestMergeMessagesEmptyIncoming(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := []model.Message{msg("1", base), msg("2", base.Add(time.Minute))}
	var incoming []model.Message

	result := mergeMessages(existing, incoming)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

func TestMergeMessagesBothEmpty(t *testing.T) {
	result := mergeMessages(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}

func TestMergeMessagesSortedChronologically(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := []model.Message{msg("3", base.Add(2*time.Minute)), msg("5", base.Add(4*time.Minute))}
	incoming := []model.Message{msg("1", base), msg("4", base.Add(3*time.Minute)), msg("2", base.Add(time.Minute))}

	result := mergeMessages(existing, incoming)
	if len(result) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result))
	}

	expectedOrder := []string{"1", "2", "3", "4", "5"}
	for i, m := range result {
		if m.ID != expectedOrder[i] {
			t.Errorf("expected message %d to have ID %q, got %q", i, expectedOrder[i], m.ID)
		}
	}
}

func TestMergeMessagesWithRealtimeOverlap(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	existing := []model.Message{
		msg("rt-1", base.Add(90*time.Minute)),
		msg("rt-2", base.Add(95*time.Minute)),
	}

	incoming := []model.Message{
		msg("h-1", base),
		msg("h-2", base.Add(30*time.Minute)),
		msg("h-3", base.Add(60*time.Minute)),
		msg("rt-1", base.Add(90*time.Minute)),
	}

	result := mergeMessages(existing, incoming)
	if len(result) != 5 {
		t.Fatalf("expected 5 messages after overlap dedup, got %d", len(result))
	}

	for i := 0; i < len(result)-1; i++ {
		if !result[i].Timestamp.Before(result[i+1].Timestamp) {
			t.Errorf("messages not sorted at index %d", i)
		}
	}
}

func TestMergeMessagesIncomingOlderThanExisting(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	existing := []model.Message{msg("new", base.Add(10*time.Hour))}
	incoming := []model.Message{msg("old", base)}

	result := mergeMessages(existing, incoming)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].ID != "old" {
		t.Errorf("expected oldest message first, got %q", result[0].ID)
	}
	if result[1].ID != "new" {
		t.Errorf("expected newest message last, got %q", result[1].ID)
	}
}

func TestViewFitsWindowWithSidebarsAndMultilineInput(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	m := New(nil, nil, nil, nil, nil, "general", "me")
	m.width = 120
	m.height = 30
	m.channels = []string{"general", "backend", "frontend", "ops", "random"}
	m.terminalOnline = []string{"me", "alice", "bob", "charlie"}
	m.presences["alice"] = model.UserPresence{Username: "alice", Status: "reviewing the release notes", Online: true}
	m.notifications = []model.Notification{
		{
			Channel:   "backend",
			Username:  "alice",
			Content:   "@me can you look at the deploy failure before standup?",
			Timestamp: base,
			MsgID:     "mention-1",
		},
	}

	for i := 0; i < 24; i++ {
		m.msgs = append(m.msgs, model.Message{
			ID:        fmt.Sprintf("msg-%02d", i),
			Username:  "alice",
			Content:   "this is a message with enough text to exercise wrapping inside the chat panel",
			Channel:   "general",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}
	m.replyTo = &m.msgs[len(m.msgs)-1]
	m.input.SetValue("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine")

	assertViewFits(t, m.View(), m.width, m.height)
}

func TestViewFitsSmallWindow(t *testing.T) {
	m := New(nil, nil, nil, nil, nil, "general", "me")
	m.width = 40
	m.height = 12
	m.input.SetValue("one\ntwo\nthree\nfour\nfive\nsix\nseven")

	assertViewFits(t, m.View(), m.width, m.height)
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()

	if got := lipgloss.Height(view); got != height {
		t.Fatalf("expected view height %d, got %d\n%s", height, got, view)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d exceeds width %d: got %d\n%s", i+1, width, got, line)
		}
	}
}
