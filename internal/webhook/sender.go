package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultTimeout = 5 * time.Second

type Sender struct {
	webhookURL string
	username   string
	client     *http.Client
}

type discordPayload struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

type SendResultMsg struct {
	Content string
	Err     error
}

func New(webhookURL, username string) *Sender {
	return &Sender{
		webhookURL: webhookURL,
		username:   username,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (s *Sender) Send(content string) error {
	if s.webhookURL == "" {
		return fmt.Errorf("webhook URL is not configured — set server.webhook_url in config or MOLLY_WEBHOOK_URL env var")
	}

	payload := discordPayload{
		Username: s.username,
		Content:  content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode message payload: %w", err)
	}

	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send message to Discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord rejected the message (HTTP %d): check your webhook URL and try again", resp.StatusCode)
	}

	return nil
}

func (s *Sender) SendAsync(content string) tea.Cmd {
	return func() tea.Msg {
		err := s.Send(content)
		return SendResultMsg{
			Content: content,
			Err:     err,
		}
	}
}
