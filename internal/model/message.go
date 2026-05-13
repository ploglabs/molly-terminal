package model

import "time"

type Attachment struct {
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Size        int    `json:"size,omitempty"`
}

type Message struct {
	ID             string       `json:"id"`
	Username       string       `json:"username"`
	Content        string       `json:"content"`
	Channel        string       `json:"channel"`
	Timestamp      time.Time    `json:"timestamp"`
	ReplyToID      string       `json:"reply_to_id,omitempty"`
	ReplyToContent string       `json:"reply_to_content,omitempty"`
	ReplyToAuthor  string       `json:"reply_to_author,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
}

type Channel struct {
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joined_at"`
}

type TypingEvent struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Channel  string `json:"channel"`
}

type RawEvent struct {
	Type string `json:"type"`
}

type UserPresence struct {
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Online    bool      `json:"online"`
	LastSeen  time.Time `json:"last_seen"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Notification struct {
	Channel   string
	Username  string
	Content   string
	Timestamp time.Time
	MsgID     string
}
