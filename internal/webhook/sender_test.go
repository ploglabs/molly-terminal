package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSender(t *testing.T) {
	s := New("https://discord.com/api/webhooks/test", "arnav")
	if s.webhookURL != "https://discord.com/api/webhooks/test" {
		t.Errorf("expected webhook URL, got %q", s.webhookURL)
	}
	if s.username != "arnav" {
		t.Errorf("expected username 'arnav', got %q", s.username)
	}
	if s.client.Timeout != defaultTimeout {
		t.Errorf("expected timeout %v, got %v", defaultTimeout, s.client.Timeout)
	}
}

func TestSendSuccess(t *testing.T) {
	var receivedPayload discordPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		defer r.Body.Close()

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Fatalf("parsing JSON payload: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	err := s.Send("hello from terminal")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if receivedPayload.Username != "arnav" {
		t.Errorf("expected username 'arnav', got %q", receivedPayload.Username)
	}
	if receivedPayload.Content != "hello from terminal" {
		t.Errorf("expected content 'hello from terminal', got %q", receivedPayload.Content)
	}
}

func TestSendWithEmptyWebhookURL(t *testing.T) {
	s := New("", "arnav")
	err := s.Send("hello")
	if err == nil {
		t.Fatal("expected error for empty webhook URL")
	}
	if err.Error() == "" {
		t.Error("expected user-friendly error message for empty webhook URL")
	}
}

func TestSendHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	err := s.Send("hello")
	if err == nil {
		t.Fatal("expected error for HTTP 403 response")
	}
	if err.Error() == "" {
		t.Error("expected user-friendly error message for HTTP error")
	}
}

func TestSendBadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	err := s.Send("hello")
	if err == nil {
		t.Fatal("expected error for HTTP 400 response")
	}
}

func TestSendConnectionError(t *testing.T) {
	s := New("http://127.0.0.1:0", "arnav")
	err := s.Send("hello")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
	if err.Error() == "" {
		t.Error("expected user-friendly error message for connection failure")
	}
}

func TestSendTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	s.client.Timeout = 100 * time.Millisecond

	err := s.Send("hello")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSendAsyncReturnsTeaMsg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	cmd := s.SendAsync("hello from terminal")
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from SendAsync")
	}

	msg := cmd()
	result, ok := msg.(SendResultMsg)
	if !ok {
		t.Fatalf("expected SendResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("expected no error in SendResultMsg, got: %v", result.Err)
	}
	if result.Content != "hello from terminal" {
		t.Errorf("expected content 'hello from terminal', got %q", result.Content)
	}
}

func TestSendAsyncReturnsErrorOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	s := New(server.URL, "arnav")
	cmd := s.SendAsync("hello")
	msg := cmd()
	result, ok := msg.(SendResultMsg)
	if !ok {
		t.Fatalf("expected SendResultMsg, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error in SendResultMsg for failed request")
	}
}

func TestDefaultTimeout(t *testing.T) {
	if defaultTimeout != 5*time.Second {
		t.Errorf("expected default timeout of 5s, got %v", defaultTimeout)
	}
}

func TestPayloadJSONFormat(t *testing.T) {
	p := discordPayload{
		Username: "arnav",
		Content:  "hello from terminal",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed["username"] != "arnav" {
		t.Errorf("expected username 'arnav', got %v", parsed["username"])
	}
	if parsed["content"] != "hello from terminal" {
		t.Errorf("expected content 'hello from terminal', got %v", parsed["content"])
	}

	if _, exists := parsed["username"]; !exists {
		t.Error("payload missing 'username' field")
	}
	if _, exists := parsed["content"]; !exists {
		t.Error("payload missing 'content' field")
	}
}
