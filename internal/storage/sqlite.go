package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/eternnoir/tg-cc/internal/logger"
)

// SessionRecord represents a persisted session record
type SessionRecord struct {
	ChatID    int64
	BotName   string
	SessionID string
	UpdatedAt time.Time
}

// SQLiteStorage handles SQLite database operations for session persistence
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	storage := &SQLiteStorage{db: db}

	// Initialize database schema
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Sugar.Infof("[storage] SQLite storage initialized at %s", dbPath)
	return storage, nil
}

// initSchema creates the necessary database tables
func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		chat_id INTEGER NOT NULL,
		bot_name TEXT NOT NULL,
		session_id TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chat_id, bot_name)
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_bot_name ON sessions(bot_name);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// SaveSession saves or updates a session record
func (s *SQLiteStorage) SaveSession(chatID int64, botName, sessionID string) error {
	query := `
	INSERT INTO sessions (chat_id, bot_name, session_id, updated_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(chat_id, bot_name) DO UPDATE SET
		session_id = excluded.session_id,
		updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.Exec(query, chatID, botName, sessionID)
	if err != nil {
		logger.Sugar.Errorf("[storage] Failed to save session for chat %d: %v", chatID, err)
		return fmt.Errorf("failed to save session: %w", err)
	}

	logger.Sugar.Debugf("[storage] Session saved for chat %d, bot %s", chatID, botName)
	return nil
}

// LoadSession loads a session record for a specific chat and bot
func (s *SQLiteStorage) LoadSession(chatID int64, botName string) (*SessionRecord, error) {
	query := `
	SELECT chat_id, bot_name, session_id, updated_at
	FROM sessions
	WHERE chat_id = ? AND bot_name = ?
	`

	var record SessionRecord
	err := s.db.QueryRow(query, chatID, botName).Scan(
		&record.ChatID,
		&record.BotName,
		&record.SessionID,
		&record.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No session found
	}
	if err != nil {
		logger.Sugar.Errorf("[storage] Failed to load session for chat %d: %v", chatID, err)
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	logger.Sugar.Debugf("[storage] Session loaded for chat %d, bot %s: %s", chatID, botName, record.SessionID)
	return &record, nil
}

// LoadAllSessions loads all session records for a specific bot
func (s *SQLiteStorage) LoadAllSessions(botName string) ([]SessionRecord, error) {
	query := `
	SELECT chat_id, bot_name, session_id, updated_at
	FROM sessions
	WHERE bot_name = ?
	`

	rows, err := s.db.Query(query, botName)
	if err != nil {
		logger.Sugar.Errorf("[storage] Failed to load sessions for bot %s: %v", botName, err)
		return nil, fmt.Errorf("failed to load sessions: %w", err)
	}
	defer rows.Close()

	var records []SessionRecord
	for rows.Next() {
		var record SessionRecord
		if err := rows.Scan(&record.ChatID, &record.BotName, &record.SessionID, &record.UpdatedAt); err != nil {
			logger.Sugar.Errorf("[storage] Failed to scan session record: %v", err)
			continue
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	logger.Sugar.Infof("[storage] Loaded %d sessions for bot %s", len(records), botName)
	return records, nil
}

// DeleteSession deletes a session record
func (s *SQLiteStorage) DeleteSession(chatID int64, botName string) error {
	query := `DELETE FROM sessions WHERE chat_id = ? AND bot_name = ?`

	_, err := s.db.Exec(query, chatID, botName)
	if err != nil {
		logger.Sugar.Errorf("[storage] Failed to delete session for chat %d: %v", chatID, err)
		return fmt.Errorf("failed to delete session: %w", err)
	}

	logger.Sugar.Debugf("[storage] Session deleted for chat %d, bot %s", chatID, botName)
	return nil
}

// ClearSessionID clears the session ID but keeps the record (for reset operation)
func (s *SQLiteStorage) ClearSessionID(chatID int64, botName string) error {
	query := `
	UPDATE sessions
	SET session_id = '', updated_at = CURRENT_TIMESTAMP
	WHERE chat_id = ? AND bot_name = ?
	`

	_, err := s.db.Exec(query, chatID, botName)
	if err != nil {
		logger.Sugar.Errorf("[storage] Failed to clear session ID for chat %d: %v", chatID, err)
		return fmt.Errorf("failed to clear session ID: %w", err)
	}

	logger.Sugar.Debugf("[storage] Session ID cleared for chat %d, bot %s", chatID, botName)
	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		logger.Sugar.Info("[storage] Closing SQLite storage")
		return s.db.Close()
	}
	return nil
}
