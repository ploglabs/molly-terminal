package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ploglabs/molly-terminal/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "molly.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestNewStoreCreatesDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "molly.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}
}

func TestNewStoreCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "deep", "molly.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file with nested directories to be created")
	}
}

func TestNewStoreSetsWALMode(t *testing.T) {
	store := openTestStore(t)

	var mode string
	err := store.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", mode)
	}
}

func TestSchemaCreatesTablesAndIndexes(t *testing.T) {
	store := openTestStore(t)

	rows, err := store.db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("querying tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}

	foundMessages := false
	foundChannels := false
	for _, tName := range tables {
		if tName == "messages" {
			foundMessages = true
		}
		if tName == "channels" {
			foundChannels = true
		}
	}
	if !foundMessages {
		t.Error("expected 'messages' table to exist")
	}
	if !foundChannels {
		t.Error("expected 'channels' table to exist")
	}
}

func TestInsertMessage(t *testing.T) {
	store := openTestStore(t)

	msg := model.Message{
		ID:        "msg-1",
		Username:  "arnav",
		Content:   "hello from terminal",
		Channel:   "general",
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	if err := store.InsertMessage(msg); err != nil {
		t.Fatalf("InsertMessage() error: %v", err)
	}

	msgs, err := store.GetMessages("general", 10, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-1" {
		t.Errorf("expected ID 'msg-1', got %q", msgs[0].ID)
	}
	if msgs[0].Username != "arnav" {
		t.Errorf("expected username 'arnav', got %q", msgs[0].Username)
	}
	if msgs[0].Content != "hello from terminal" {
		t.Errorf("expected content 'hello from terminal', got %q", msgs[0].Content)
	}
	if msgs[0].Channel != "general" {
		t.Errorf("expected channel 'general', got %q", msgs[0].Channel)
	}
}

func TestInsertMessageDeduplication(t *testing.T) {
	store := openTestStore(t)

	msg := model.Message{
		ID:        "msg-dup",
		Username:  "arnav",
		Content:   "hello",
		Channel:   "general",
		Timestamp: time.Now().UTC(),
	}

	if err := store.InsertMessage(msg); err != nil {
		t.Fatalf("first InsertMessage() error: %v", err)
	}
	if err := store.InsertMessage(msg); err != nil {
		t.Fatalf("second InsertMessage() error: %v", err)
	}

	msgs, err := store.GetMessages("general", 10, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message after duplicate insert, got %d", len(msgs))
	}
}

func TestGetMessagesByChannel(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	channels := []string{"general", "dev", "general"}
	contents := []string{"hello", "world", "hi again"}

	for i, ch := range channels {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "arnav",
			Content:   contents[i],
			Channel:   ch,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	msgs, err := store.GetMessages("general", 10, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in 'general', got %d", len(msgs))
	}
}

func TestGetMessagesWithLimit(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	msgs, err := store.GetMessages("general", 3, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages with limit, got %d", len(msgs))
	}
}

func TestGetMessagesOrderedByTimestampDesc(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	msgs, err := store.GetMessages("general", 10, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}

	if len(msgs) < 2 {
		t.Fatal("need at least 2 messages to check ordering")
	}
	if !msgs[0].Timestamp.After(msgs[1].Timestamp) {
		t.Errorf("expected messages ordered by timestamp DESC, but %v is not after %v", msgs[0].Timestamp, msgs[1].Timestamp)
	}
}

func TestGetMessagesBeforeTime(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	before := baseTime.Add(3 * time.Hour)
	msgs, err := store.GetMessages("general", 10, &before)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}

	for _, m := range msgs {
		if !m.Timestamp.Before(before) {
			t.Errorf("expected message timestamp before %v, got %v", before, m.Timestamp)
		}
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages before cutoff, got %d", len(msgs))
	}
}

func TestGetMessagesPagination(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	page1, err := store.GetMessages("general", 5, nil)
	if err != nil {
		t.Fatalf("GetMessages page 1: %v", err)
	}
	if len(page1) != 5 {
		t.Fatalf("expected 5 messages on page 1, got %d", len(page1))
	}

	before := page1[len(page1)-1].Timestamp
	page2, err := store.GetMessages("general", 5, &before)
	if err != nil {
		t.Fatalf("GetMessages page 2: %v", err)
	}
	if len(page2) != 5 {
		t.Fatalf("expected 5 messages on page 2, got %d", len(page2))
	}

	for _, m := range page2 {
		if !m.Timestamp.Before(before) {
			t.Errorf("page 2 message timestamp %v should be before %v", m.Timestamp, before)
		}
	}
}

func TestGetMessagesEmptyChannel(t *testing.T) {
	store := openTestStore(t)

	msgs, err := store.GetMessages("nonexistent", 10, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for empty channel, got %d", len(msgs))
	}
}

func TestSearchMessages(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	messages := []struct {
		id      string
		content string
		channel string
	}{
		{"msg-1", "hello world", "general"},
		{"msg-2", "goodbye world", "general"},
		{"msg-3", "hello universe", "dev"},
		{"msg-4", "random text", "general"},
	}

	for i, m := range messages {
		msg := model.Message{
			ID:        m.id,
			Username:  "user",
			Content:   m.content,
			Channel:   m.channel,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	results, err := store.SearchMessages("hello")
	if err != nil {
		t.Fatalf("SearchMessages() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'hello', got %d", len(results))
	}

	results, err = store.SearchMessages("world")
	if err != nil {
		t.Fatalf("SearchMessages() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'world', got %d", len(results))
	}
}

func TestSearchMessagesNoResults(t *testing.T) {
	store := openTestStore(t)

	results, err := store.SearchMessages("nonexistent")
	if err != nil {
		t.Fatalf("SearchMessages() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchMessagesPartialMatch(t *testing.T) {
	store := openTestStore(t)

	msg := model.Message{
		ID:        "msg-1",
		Username:  "user",
		Content:   "the quick brown fox jumps over the lazy dog",
		Channel:   "general",
		Timestamp: time.Now().UTC(),
	}
	if err := store.InsertMessage(msg); err != nil {
		t.Fatalf("InsertMessage() error: %v", err)
	}

	results, err := store.SearchMessages("brown fox")
	if err != nil {
		t.Fatalf("SearchMessages() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for partial match, got %d", len(results))
	}
}

func TestSearchMessagesLimit100(t *testing.T) {
	store := openTestStore(t)

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 150; i++ {
		msg := model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("findme message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := store.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error: %v", err)
		}
	}

	results, err := store.SearchMessages("findme")
	if err != nil {
		t.Fatalf("SearchMessages() error: %v", err)
	}
	if len(results) != 100 {
		t.Errorf("expected search results capped at 100, got %d", len(results))
	}
}

func TestInsertChannel(t *testing.T) {
	store := openTestStore(t)

	if err := store.InsertChannel("general"); err != nil {
		t.Fatalf("InsertChannel() error: %v", err)
	}

	channels, err := store.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels() error: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0] != "general" {
		t.Errorf("expected channel 'general', got %q", channels[0])
	}
}

func TestInsertChannelDeduplication(t *testing.T) {
	store := openTestStore(t)

	if err := store.InsertChannel("general"); err != nil {
		t.Fatalf("first InsertChannel() error: %v", err)
	}
	if err := store.InsertChannel("general"); err != nil {
		t.Fatalf("second InsertChannel() error: %v", err)
	}

	channels, err := store.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels() error: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("expected 1 channel after duplicate insert, got %d", len(channels))
	}
}

func TestGetChannelsOrderedByName(t *testing.T) {
	store := openTestStore(t)

	for _, name := range []string{"dev", "general", "random"} {
		if err := store.InsertChannel(name); err != nil {
			t.Fatalf("InsertChannel() error: %v", err)
		}
	}

	channels, err := store.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels() error: %v", err)
	}
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}

	expected := []string{"dev", "general", "random"}
	for i, ch := range channels {
		if ch != expected[i] {
			t.Errorf("expected channel %q at position %d, got %q", expected[i], i, ch)
		}
	}
}

func TestGetChannelsEmpty(t *testing.T) {
	store := openTestStore(t)

	channels, err := store.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels() error: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(channels))
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "molly.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "molly.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_ = store.Close()
	_ = store.Close()
}

func TestDefaultDBPath(t *testing.T) {
	path, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath() error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty default DB path")
	}
}

func TestDefaultDBPathWithXDG(t *testing.T) {
	os.Setenv("XDG_DATA_HOME", "/tmp/xdg-test-molly")
	defer os.Unsetenv("XDG_DATA_HOME")

	path, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath() error: %v", err)
	}
	expected := "/tmp/xdg-test-molly/molly/molly.db"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestNewStoreWithEmptyPath(t *testing.T) {
	origXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", "")
	defer os.Setenv("XDG_DATA_HOME", origXDG)

	store, err := New("")
	if err != nil {
		t.Fatalf("New('') error: %v", err)
	}
	defer store.Close()
}

func TestConcurrentInserts(t *testing.T) {
	store := openTestStore(t)

	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			msg := model.Message{
				ID:        fmt.Sprintf("concurrent-%d", n),
				Username:  "user",
				Content:   fmt.Sprintf("concurrent message %d", n),
				Channel:   "general",
				Timestamp: time.Now().UTC().Add(time.Duration(n) * time.Millisecond),
			}
			done <- store.InsertMessage(msg)
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent insert %d failed: %v", i, err)
		}
	}

	msgs, err := store.GetMessages("general", 20, nil)
	if err != nil {
		t.Fatalf("GetMessages() error: %v", err)
	}
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages after concurrent inserts, got %d", len(msgs))
	}
}
