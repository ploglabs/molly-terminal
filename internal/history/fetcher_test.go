package history

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ploglabs/molly-terminal/internal/model"
)

func makeMessages(n int, baseTime time.Time) []model.Message {
	msgs := make([]model.Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = model.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Username:  "user",
			Content:   fmt.Sprintf("message %d", i),
			Channel:   "general",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
	}
	return msgs
}

func TestNewFetcher(t *testing.T) {
	f := New("https://relay.example.com")
	if f.baseURL != "https://relay.example.com" {
		t.Errorf("expected baseURL 'https://relay.example.com', got %q", f.baseURL)
	}
	if f.httpClient.Timeout != defaultTimeout {
		t.Errorf("expected timeout %v, got %v", defaultTimeout, f.httpClient.Timeout)
	}
}

func TestNewFetcherWithEmptyURL(t *testing.T) {
	f := New("")
	if f.baseURL != "" {
		t.Errorf("expected empty baseURL, got %q", f.baseURL)
	}
}

func TestFetchSuccess(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	expected := makeMessages(3, baseTime)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if limit := r.URL.Query().Get("limit"); limit != "100" {
			t.Errorf("expected limit=100, got %s", limit)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	f := New(server.URL)
	msgs, err := f.Fetch("general", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-0" {
		t.Errorf("expected first message ID 'msg-0', got %q", msgs[0].ID)
	}
}

func TestFetchWithBeforeCursor(t *testing.T) {
	baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	expected := makeMessages(5, baseTime)
	before := baseTime.Add(10 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beforeParam := r.URL.Query().Get("before")
		if beforeParam == "" {
			t.Error("expected 'before' query parameter to be set")
		}
		limit := r.URL.Query().Get("limit")
		if limit != "100" {
			t.Errorf("expected limit=100, got %s", limit)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	f := New(server.URL)
	msgs, err := f.Fetch("general", 100, &before)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}
}

func TestFetchWithCustomLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		if limit != "50" {
			t.Errorf("expected limit=50, got %s", limit)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{})
	}))
	defer server.Close()

	f := New(server.URL)
	_, err := f.Fetch("general", 50, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
}

func TestFetchChannelInURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/channels/dev/messages"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{})
	}))
	defer server.Close()

	f := New(server.URL)
	_, err := f.Fetch("dev", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
}

func TestFetchEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{})
	}))
	defer server.Close()

	f := New(server.URL)
	msgs, err := f.Fetch("general", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestFetchWithEmptyBaseURL(t *testing.T) {
	f := New("")
	msgs, err := f.Fetch("general", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() with empty URL should not error, got: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil messages for empty URL, got %d", len(msgs))
	}
}

func TestFetchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	f := New(server.URL)
	_, err := f.Fetch("general", 100, nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500 response")
	}
}

func TestFetchForbiddenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	f := New(server.URL)
	_, err := f.Fetch("general", 100, nil)
	if err == nil {
		t.Fatal("expected error for HTTP 403 response")
	}
}

func TestFetchConnectionError(t *testing.T) {
	f := New("http://127.0.0.1:0")
	_, err := f.Fetch("general", 100, nil)
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestFetchInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	f := New(server.URL)
	_, err := f.Fetch("general", 100, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestFetchAsyncReturnsTeaMsg(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	expected := makeMessages(2, baseTime)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	f := New(server.URL)
	cmd := f.FetchAsync("general", 100, nil)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from FetchAsync")
	}

	msg := cmd()
	result, ok := msg.(FetchResultMsg)
	if !ok {
		t.Fatalf("expected FetchResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("expected no error in FetchResultMsg, got: %v", result.Err)
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Channel != "general" {
		t.Errorf("expected channel 'general', got %q", result.Channel)
	}
}

func TestFetchAsyncErrorOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	f := New(server.URL)
	cmd := f.FetchAsync("general", 100, nil)
	msg := cmd()
	result, ok := msg.(FetchResultMsg)
	if !ok {
		t.Fatalf("expected FetchResultMsg, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error in FetchResultMsg for failed request")
	}
}

func TestInitialFetchWithValidFetcher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{})
	}))
	defer server.Close()

	f := New(server.URL)
	cmd := InitialFetch(f, "general", 100)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from InitialFetch")
	}

	msg := cmd()
	result, ok := msg.(FetchResultMsg)
	if !ok {
		t.Fatalf("expected FetchResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("expected no error, got: %v", result.Err)
	}
}

func TestInitialFetchWithNilFetcher(t *testing.T) {
	cmd := InitialFetch(nil, "general", 100)
	if cmd != nil {
		t.Error("expected nil tea.Cmd for nil fetcher")
	}
}

func TestInitialFetchWithEmptyBaseURL(t *testing.T) {
	f := New("")
	cmd := InitialFetch(f, "general", 100)
	if cmd != nil {
		t.Error("expected nil tea.Cmd for empty base URL")
	}
}

func TestInitialFetchWithZeroLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		if limit != "100" {
			t.Errorf("expected default limit=100, got %s", limit)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{})
	}))
	defer server.Close()

	f := New(server.URL)
	cmd := InitialFetch(f, "general", 0)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
	cmd()
}

func TestLoadOlderWithValidFetcher(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	oldest := baseTime

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		beforeParam := r.URL.Query().Get("before")
		if beforeParam == "" {
			t.Error("expected 'before' query parameter for LoadOlder")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeMessages(5, baseTime.Add(-5*time.Hour)))
	}))
	defer server.Close()

	f := New(server.URL)
	cmd := LoadOlder(f, "general", oldest)
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from LoadOlder")
	}

	msg := cmd()
	result, ok := msg.(FetchResultMsg)
	if !ok {
		t.Fatalf("expected FetchResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("expected no error, got: %v", result.Err)
	}
	if len(result.Messages) != 5 {
		t.Errorf("expected 5 older messages, got %d", len(result.Messages))
	}
}

func TestLoadOlderWithNilFetcher(t *testing.T) {
	cmd := LoadOlder(nil, "general", time.Now())
	if cmd != nil {
		t.Error("expected nil tea.Cmd for nil fetcher")
	}
}

func TestLoadOlderWithEmptyBaseURL(t *testing.T) {
	f := New("")
	cmd := LoadOlder(f, "general", time.Now())
	if cmd != nil {
		t.Error("expected nil tea.Cmd for empty base URL")
	}
}

func TestDefaultTimeout(t *testing.T) {
	if defaultTimeout != 5*time.Second {
		t.Errorf("expected default timeout of 5s, got %v", defaultTimeout)
	}
}

func TestDefaultLimit(t *testing.T) {
	if defaultLimit != 100 {
		t.Errorf("expected default limit of 100, got %d", defaultLimit)
	}
}

func TestFetchMessageFieldsParsedCorrectly(t *testing.T) {
	ts := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	expected := model.Message{
		ID:        "abc-123",
		Username:  "arnav",
		Content:   "hello from terminal",
		Channel:   "general",
		Timestamp: ts,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]model.Message{expected})
	}))
	defer server.Close()

	f := New(server.URL)
	msgs, err := f.Fetch("general", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %q", m.ID)
	}
	if m.Username != "arnav" {
		t.Errorf("expected username 'arnav', got %q", m.Username)
	}
	if m.Content != "hello from terminal" {
		t.Errorf("expected content 'hello from terminal', got %q", m.Content)
	}
	if m.Channel != "general" {
		t.Errorf("expected channel 'general', got %q", m.Channel)
	}
}

func TestFetchLargeBatch(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expected := makeMessages(100, baseTime)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	f := New(server.URL)
	msgs, err := f.Fetch("general", 100, nil)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(msgs) != 100 {
		t.Errorf("expected 100 messages, got %d", len(msgs))
	}
}
