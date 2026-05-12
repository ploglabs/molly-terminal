package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ploglabs/molly-terminal/internal/model"
)

type Store struct {
	db *sql.DB
}

var schema = []string{
	`CREATE TABLE IF NOT EXISTS messages (
		id        TEXT PRIMARY KEY,
		username  TEXT NOT NULL,
		content   TEXT NOT NULL,
		channel   TEXT NOT NULL,
		timestamp DATETIME NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS channels (
		name      TEXT PRIMARY KEY,
		joined_at DATETIME NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS user_presence (
		username   TEXT PRIMARY KEY,
		status     TEXT NOT NULL DEFAULT '',
		online     INTEGER NOT NULL DEFAULT 0,
		last_seen  DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel, timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_content ON messages(content)`,
}

func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		var err error
		dbPath, err = DefaultDBPath()
		if err != nil {
			return nil, fmt.Errorf("resolving default DB path: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("executing schema migration: %w", err)
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) InsertMessage(msg model.Message) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO messages (id, username, content, channel, timestamp) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.Username, msg.Content, msg.Channel, msg.Timestamp.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting message: %w", err)
	}
	return nil
}

func (s *Store) GetMessages(channel string, limit int, before *time.Time) ([]model.Message, error) {
	var args []interface{}
	var query string

	if before != nil {
		query = `SELECT id, username, content, channel, timestamp FROM messages WHERE channel = ? AND timestamp < ? ORDER BY timestamp DESC LIMIT ?`
		args = []interface{}{channel, before.UTC(), limit}
	} else {
		query = `SELECT id, username, content, channel, timestamp FROM messages WHERE channel = ? ORDER BY timestamp DESC LIMIT ?`
		args = []interface{}{channel, limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.Username, &m.Content, &m.Channel, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("scanning message row: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating message rows: %w", err)
	}

	return msgs, nil
}

func (s *Store) SearchMessages(query string) ([]model.Message, error) {
	rows, err := s.db.Query(
		`SELECT id, username, content, channel, timestamp FROM messages WHERE content LIKE ? ORDER BY timestamp DESC LIMIT 100`,
		"%"+query+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("searching messages: %w", err)
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.Username, &m.Content, &m.Channel, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("scanning search result row: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search result rows: %w", err)
	}

	return msgs, nil
}

func (s *Store) InsertChannel(name string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO channels (name, joined_at) VALUES (?, ?)`,
		name, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting channel: %w", err)
	}
	return nil
}

func (s *Store) GetChannels() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM channels ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying channels: %w", err)
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning channel row: %w", err)
		}
		channels = append(channels, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating channel rows: %w", err)
	}

	return channels, nil
}

func (s *Store) DeleteChannel(name string) error {
	_, err := s.db.Exec(`DELETE FROM channels WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting channel: %w", err)
	}
	return nil
}

func (s *Store) UpsertPresence(p model.UserPresence) error {
	online := 0
	if p.Online {
		online = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO user_presence (username, status, online, last_seen, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(username) DO UPDATE SET
		   status=excluded.status, online=excluded.online,
		   last_seen=excluded.last_seen, updated_at=excluded.updated_at`,
		p.Username, p.Status, online, p.LastSeen.UTC(), p.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upserting presence: %w", err)
	}
	return nil
}

func (s *Store) GetAllPresence() ([]model.UserPresence, error) {
	rows, err := s.db.Query(
		`SELECT username, status, online, last_seen, updated_at FROM user_presence ORDER BY username`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying presence: %w", err)
	}
	defer rows.Close()

	var result []model.UserPresence
	for rows.Next() {
		var p model.UserPresence
		var onlineInt int
		if err := rows.Scan(&p.Username, &p.Status, &onlineInt, &p.LastSeen, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning presence row: %w", err)
		}
		p.Online = onlineInt != 0
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating presence rows: %w", err)
	}
	return result, nil
}

func (s *Store) SetUserOffline(username string) error {
	_, err := s.db.Exec(
		`UPDATE user_presence SET online=0, last_seen=? WHERE username=?`,
		time.Now().UTC(), username,
	)
	if err != nil {
		return fmt.Errorf("setting user offline: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func DefaultDBPath() (string, error) {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = os.Getenv("APPDATA")
		}
		if localAppData == "" {
			return "", fmt.Errorf("cannot determine Windows data directory: LOCALAPPDATA and APPDATA not set")
		}
		return filepath.Join(localAppData, "molly", "molly.db"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "molly", "molly.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "molly", "molly.db"), nil
}
