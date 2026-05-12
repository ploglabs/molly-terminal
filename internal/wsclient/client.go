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
	url       string
	username  string
	channel   string
	conn      *websocket.Conn
	connMu    sync.Mutex
	msgCh     chan model.Message
	typingCh  chan model.TypingEvent
	statusCh  chan StatusChange
	done      chan struct{}
	closeOnce sync.Once
	log       *log.Logger
}

func New(url, username, channel string) *Client {
	return &Client{
		url:      url,
		username: username,
		channel:  channel,
		msgCh:    make(chan model.Message, 256),
		typingCh: make(chan model.TypingEvent, 32),
		statusCh: make(chan StatusChange, 16),
		done:     make(chan struct{}),
		log:      log.Default(),
	}
}

func (c *Client) Connect() error {
	if err := c.dial(); err != nil {
		return fmt.Errorf("initial connection failed: %w", err)
	}
	c.emitStatus(StatusConnected, nil)
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

// relayEvent is the wire format broadcast by molly-discord-relay.
type relayEvent struct {
	Type      string `json:"type"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	MessageID string `json:"message_id"`
	Timestamp string `json:"timestamp"`
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
				ID:        evt.MessageID,
				Username:  evt.Username,
				Content:   evt.Content,
				Channel:   evt.Channel,
				Timestamp: ts,
			}
			select {
			case c.msgCh <- msg:
			case <-c.done:
				return
			default:
				c.log.Printf("ws: message channel full, dropping message")
			}

		default:
			// ignore message_update, message_delete, reaction_*, user_join, user_leave
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