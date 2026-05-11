package history

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/model"
)

const defaultTimeout = 5 * time.Second
const defaultLimit = 100

type Fetcher struct {
	baseURL    string
	httpClient *http.Client
}

type FetchResultMsg struct {
	Messages []model.Message
	Channel  string
	Err      error
}

func New(baseURL string) *Fetcher {
	return &Fetcher{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (f *Fetcher) Fetch(channel string, limit int, before *time.Time) ([]model.Message, error) {
	if f.baseURL == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/channels/%s/messages?limit=%d", f.baseURL, channel, limit)
	if before != nil {
		url += fmt.Sprintf("&before=%s", before.UTC().Format(time.RFC3339Nano))
	}

	resp, err := f.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching message history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relay API returned HTTP %d", resp.StatusCode)
	}

	var msgs []model.Message
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, fmt.Errorf("decoding message history response: %w", err)
	}

	return msgs, nil
}

func (f *Fetcher) FetchAsync(channel string, limit int, before *time.Time) tea.Cmd {
	return func() tea.Msg {
		msgs, err := f.Fetch(channel, limit, before)
		return FetchResultMsg{
			Messages: msgs,
			Channel:  channel,
			Err:      err,
		}
	}
}

func InitialFetch(f *Fetcher, channel string, limit int) tea.Cmd {
	if f == nil || f.baseURL == "" {
		return nil
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	return f.FetchAsync(channel, limit, nil)
}

func LoadOlder(f *Fetcher, channel string, oldestTimestamp time.Time) tea.Cmd {
	if f == nil || f.baseURL == "" {
		return nil
	}
	before := oldestTimestamp
	return f.FetchAsync(channel, defaultLimit, &before)
}
