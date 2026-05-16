package wsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ploglabs/molly-terminal/internal/model"
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2
	writeWait      = 10 * time.Second
)

type Status string

const (
	StatusConnected    Status = "connected"
	StatusDisconnected Status = "disconnected"
	StatusReconnecting Status = "reconnecting"
)

type StatusChange struct {
	Status Status
	Err    error
}

type Client struct {
	url        string
	username   string
	channel    string
	conn       *websocket.Conn
	connMu     sync.Mutex
	msgCh      chan model.Message
	typingCh   chan model.TypingEvent
	statusCh   chan StatusChange
	presenceCh chan model.UserPresence
	termUsersCh chan []string
	done       chan struct{}
	closeOnce  sync.Once
	log        *log.Logger
}

func New(url, username, channel string) *Client {
	return &Client{
		url:         url,
		username:    username,
		channel:     channel,
		msgCh:       make(chan model.Message, 256),
		typingCh:    make(chan model.TypingEvent, 32),
		statusCh:    make(chan StatusChange, 16),
		presenceCh:  make(chan model.UserPresence, 32),
		termUsersCh: make(chan []string, 16),
		done:        make(chan struct{}),
		log:         log.Default(),
	}
}

func (c *Client) Connect() error {
	if err := c.dial(); err != nil {
		return fmt.Errorf("initial connection failed: %w", err)
	}
	c.emitStatus(StatusConnected, nil)
	_ = c.identify()
	go c.readLoop()
	return nil
}

func (c *Client) ConnectWithRetry(ctx context.Context) {
	backoff := initialBackoff
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := c.dial()
		if err != nil {
			c.log.Printf("ws: connect failed: %v, retrying in %v", err, backoff)
			c.emitStatus(StatusReconnecting, err)
			select {
			case <-time.After(backoff):
				backoff = scaleBackoff(backoff)
				continue
			case <-ctx.Done():
				return
			}
		}

		c.emitStatus(StatusConnected, nil)
		_ = c.identify()
		backoff = initialBackoff
		c.readLoop()

		select {
		case <-c.done:
			return
		default:
		}

		c.emitStatus(StatusDisconnected, nil)
		c.log.Printf("ws: connection lost, reconnecting in %v", backoff)
		c.emitStatus(StatusReconnecting, nil)
		select {
		case <-time.After(backoff):
			backoff = scaleBackoff(backoff)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) Messages() <-chan model.Message {
	return c.msgCh
}

func (c *Client) TypingEvents() <-chan model.TypingEvent {
	return c.typingCh
}

func (c *Client) StatusChanges() <-chan StatusChange {
	return c.statusCh
}

func (c *Client) Presences() <-chan model.UserPresence {
	return c.presenceCh
}

func (c *Client) TerminalUsers() <-chan []string {
	return c.termUsersCh
}

func (c *Client) identify() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return nil
	}
	msg := struct {
		Action   string `json:"action"`
		Username string `json:"username"`
	}{Action: "identify", Username: c.username}
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteJSON(msg)
}

func (c *Client) SendStatus(status string) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	msg := struct {
		Action   string `json:"action"`
		Status   string `json:"status"`
		Username string `json:"username"`
	}{
		Action:   "status_update",
		Status:   status,
		Username: c.username,
	}
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteJSON(msg)
}

func (c *Client) Subscribe(channel string) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	msg := struct {
		Action  string `json:"action"`
		Channel string `json:"channel"`
	}{
		Action:  "subscribe",
		Channel: channel,
	}
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("subscribe write: %w", err)
	}
	c.channel = channel
	return nil
}

func (c *Client) Unsubscribe(channel string) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	msg := struct {
		Action  string `json:"action"`
		Channel string `json:"channel"`
	}{
		Action:  "unsubscribe",
		Channel: channel,
	}
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("unsubscribe write: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = c.conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()
	})
	return err
}

func (c *Client) dial() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(context.Background(), c.url, nil)
	if err != nil {
		return err
	}

	c.conn = conn
	return nil
}

type relayAttachment struct {
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Size        int    `json:"size"`
}

type relayEvent struct {
	Type           string            `json:"type"`
	Channel        string            `json:"channel"`
	ChannelID      string            `json:"channel_id"`
	Username       string            `json:"username"`
	UserID         string            `json:"user_id"`
	Content        string            `json:"content"`
	MessageID      string            `json:"message_id"`
	Timestamp      string            `json:"timestamp"`
	Status         string            `json:"status"`
	UpdatedAt      string            `json:"updated_at"`
	ReplyToID      string            `json:"reply_to_id"`
	ReplyToContent string            `json:"reply_to_content"`
	ReplyToAuthor  string            `json:"reply_to_author"`
	Users          []string          `json:"users"`
	Attachments    []relayAttachment `json:"attachments"`
}

func (c *Client) readLoop() {
	defer func() {
		c.connMu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()
		if conn == nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-c.done:
				return
			default:
			}
			c.log.Printf("ws: read error: %v", err)
			return
		}

		var evt relayEvent
		if err := json.Unmarshal(raw, &evt); err != nil {
			c.log.Printf("ws: invalid event: %v", err)
			continue
		}

		switch evt.Type {
		case "typing_start", "typing":
			te := model.TypingEvent{
				Type:     "typing",
				Username: evt.Username,
				Channel:  evt.Channel,
			}
			select {
			case c.typingCh <- te:
			case <-c.done:
				return
			default:
			}

		case "message_create":
			ts, _ := time.Parse(time.RFC3339, evt.Timestamp)
			msg := model.Message{
				ID:             evt.MessageID,
				Username:       evt.Username,
				UserID:         evt.UserID,
				Content:        evt.Content,
				Channel:        evt.Channel,
				Timestamp:      ts,
				ReplyToID:      evt.ReplyToID,
				ReplyToContent: evt.ReplyToContent,
				ReplyToAuthor:  evt.ReplyToAuthor,
				Attachments:    convertAttachments(evt.Attachments),
			}
			select {
			case c.msgCh <- msg:
			case <-c.done:
				return
			default:
				c.log.Printf("ws: message channel full, dropping message")
			}

		case "terminal_online":
			users := evt.Users
			if users == nil {
				users = []string{}
			}
			select {
			case c.termUsersCh <- users:
			case <-c.done:
				return
			default:
			}

		case "status_update":
			updatedAt, _ := time.Parse(time.RFC3339, evt.UpdatedAt)
			if updatedAt.IsZero() {
				updatedAt = time.Now()
			}
			p := model.UserPresence{
				Username:  evt.Username,
				Status:    evt.Status,
				Online:    true,
				LastSeen:  updatedAt,
				UpdatedAt: updatedAt,
			}
			select {
			case c.presenceCh <- p:
			case <-c.done:
				return
			default:
			}
		}
	}
}

func (c *Client) emitStatus(s Status, err error) {
	select {
	case c.statusCh <- StatusChange{Status: s, Err: err}:
	default:
	}
}

func scaleBackoff(current time.Duration) time.Duration {
	next := current * backoffFactor
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

func convertAttachments(ras []relayAttachment) []model.Attachment {
	result := make([]model.Attachment, 0, len(ras))
	for _, ra := range ras {
		result = append(result, model.Attachment{
			URL:         ra.URL,
			ProxyURL:    ra.ProxyURL,
			Filename:    ra.Filename,
			ContentType: ra.ContentType,
			Width:       ra.Width,
			Height:      ra.Height,
			Size:        ra.Size,
		})
	}
	return result
}
