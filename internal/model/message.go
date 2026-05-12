package model

import "time"

type Message struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
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
