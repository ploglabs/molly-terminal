package wsclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestNewClient(t *testing.T) {
	c := New("wss://example.com/ws", "user", "general")
	if c.url != "wss://example.com/ws" {
		t.Errorf("expected url 'wss://example.com/ws', got %q", c.url)
	}
	if c.username != "user" {
		t.Errorf("expected username 'user', got %q", c.username)
	}
	if c.channel != "general" {
		t.Errorf("expected channel 'general', got %q", c.channel)
	}
}

func TestClientConnectsAndReceivesMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		// Send in relay event format (what molly-discord-relay broadcasts)
		evt := relayEvent{
			Type:      "message_create",
			MessageID: "1",
			Username:  "alice",
			Content:   "hello",
			Channel:   "general",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.Marshal(evt)
		_ = ws.WriteMessage(websocket.TextMessage, data)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	client := New(url, "user", "general")

	if err := client.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	select {
	case msg := <-client.Messages():
		if msg.ID != "1" {
			t.Errorf("expected message ID '1', got %q", msg.ID)
		}
		if msg.Username != "alice" {
			t.Errorf("expected username 'alice', got %q", msg.Username)
		}
		if msg.Content != "hello" {
			t.Errorf("expected content 'hello', got %q", msg.Content)
		}
		if msg.Channel != "general" {
			t.Errorf("expected channel 'general', got %q", msg.Channel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	_ = client.Close()
}

func TestClientReconnection(t *testing.T) {
	var mu sync.Mutex
	connCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connCount++
		n := connCount
		mu.Unlock()

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if n == 1 {
			ws.Close()
			return
		}
		defer ws.Close()
		evt := relayEvent{
			Type:      "message_create",
			MessageID: "reconnect-msg",
			Username:  "bob",
			Content:   "back online",
			Channel:   "general",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.Marshal(evt)
		_ = ws.WriteMessage(websocket.TextMessage, data)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	serverURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	client := New(serverURL, "user", "general")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range client.Messages() {
			if msg.ID == "reconnect-msg" {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go client.ConnectWithRetry(ctx)

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for reconnection")
	}

	_ = client.Close()
}

func TestSubscribeUnsubscribe(t *testing.T) {
	var mu sync.Mutex
	var receivedActions []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var parsed struct {
				Action  string `json:"action"`
				Channel string `json:"channel"`
			}
			if err := json.Unmarshal(msg, &parsed); err != nil {
				continue
			}
			mu.Lock()
			receivedActions = append(receivedActions, parsed.Action)
			mu.Unlock()
		}
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	client := New(url, "user", "general")

	if err := client.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := client.Subscribe("dev"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if err := client.Unsubscribe("dev"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	_ = client.Close()

	mu.Lock()
	actions := receivedActions
	mu.Unlock()

	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}
	if actions[0] != "subscribe" {
		t.Errorf("expected first action 'subscribe', got %q", actions[0])
	}
	if actions[1] != "unsubscribe" {
		t.Errorf("expected second action 'unsubscribe', got %q", actions[1])
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	client := New("wss://localhost/fake", "user", "general")

	_ = client.Close()
	_ = client.Close()
}

func TestScaleBackoff(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{4 * time.Second, 8 * time.Second},
		{16 * time.Second, 30 * time.Second},
		{30 * time.Second, 30 * time.Second},
	}
	for _, tc := range tests {
		result := scaleBackoff(tc.input)
		if result != tc.expected {
			t.Errorf("scaleBackoff(%v) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestStatusChanges(t *testing.T) {
	client := New("wss://localhost/fake", "user", "general")

	client.emitStatus(StatusConnected, nil)
	client.emitStatus(StatusDisconnected, nil)

	select {
	case sc := <-client.StatusChanges():
		if sc.Status != StatusConnected {
			t.Errorf("expected connected status, got %q", sc.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status")
	}

	select {
	case sc := <-client.StatusChanges():
		if sc.Status != StatusDisconnected {
			t.Errorf("expected disconnected status, got %q", sc.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status")
	}
}

func TestSubscribeReturnsErrorWhenNotConnected(t *testing.T) {
	client := New("wss://localhost/fake", "user", "general")
	err := client.Subscribe("dev")
	if err == nil {
		t.Error("expected error when subscribing without connection")
	}
}

func TestUnsubscribeReturnsErrorWhenNotConnected(t *testing.T) {
	client := New("wss://localhost/fake", "user", "general")
	err := client.Unsubscribe("dev")
	if err == nil {
		t.Error("expected error when unsubscribing without connection")
	}
}