package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/eternnoir/tg-cc/internal/claude"
	"github.com/eternnoir/tg-cc/internal/config"
	"github.com/eternnoir/tg-cc/internal/logger"
	"github.com/eternnoir/tg-cc/internal/storage"
	"github.com/eternnoir/tg-cc/internal/whitelist"
)

const (
	maxMessageLength = 4096 // Telegram message length limit
)

// Bot represents a Telegram bot instance
type Bot struct {
	name           string
	api            *tgbotapi.BotAPI
	whitelist      *whitelist.Whitelist
	sessionManager *claude.SessionManager
	workingDir     string
	tempDir        string
}

// New creates a new Bot instance
func New(cfg config.BotConfig, store *storage.SQLiteStorage) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	return &Bot{
		name:           cfg.Name,
		api:            api,
		whitelist:      whitelist.New(cfg.Whitelist),
		sessionManager: claude.NewSessionManager(cfg.WorkingDir, cfg.ClaudeArgs, cfg.Name, store),
		workingDir:     cfg.WorkingDir,
		tempDir:        cfg.TempDir,
	}, nil
}

// Start starts the bot and begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	logger.Sugar.Infof("[%s] Bot started. Working directory: %s", b.name, b.workingDir)
	logger.Sugar.Infof("[%s] Bot username: @%s", b.name, b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			logger.Sugar.Infof("[%s] Bot stopping...", b.name)
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			go b.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	// Check whitelist
	if !b.whitelist.IsAllowed(userID) {
		logger.Sugar.Warnf("[%s] Unauthorized user %d attempted to use the bot", b.name, userID)
		b.sendMessage(chatID, "⛔ You are not authorized to use this bot.")
		return
	}

	// Handle commands
	if msg.IsCommand() {
		b.handleCommand(ctx, msg)
		return
	}

	// Collect text and file paths
	text := msg.Text
	if msg.Caption != "" {
		text = msg.Caption
	}

	// Handle file uploads
	var filePaths []string
	if b.tempDir != "" {
		filePaths = b.downloadFiles(ctx, msg)
	}

	// If no text and no files, ignore
	if text == "" && len(filePaths) == 0 {
		return
	}

	// Append file paths to message
	if len(filePaths) > 0 {
		if text != "" {
			text += "\n\n"
		}
		text += "Attached files:\n"
		for _, fp := range filePaths {
			text += fmt.Sprintf("- %s\n", fp)
		}
	}

	b.handleClaudeMessage(ctx, chatID, text)
}

// handleCommand processes bot commands
func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	command := msg.Command()

	switch command {
	case "start":
		b.sendMessage(chatID, fmt.Sprintf(`👋 Welcome to Claude Code Bot!

📁 Working Directory: %s

Available commands:
/new - Start a new session (clears previous context)
/clear - Clear the current session
/status - Show current session status
/help - Show this help message

Just send me a message to start chatting with Claude Code!`, b.workingDir))

	case "new", "reset":
		b.sessionManager.ResetSession(chatID)
		b.sendMessage(chatID, "🔄 Session has been reset. Starting fresh!")

	case "clear":
		b.sessionManager.ClearSession(chatID)
		b.sendMessage(chatID, "🗑️ Session has been cleared.")

	case "status":
		session := b.sessionManager.GetOrCreateSession(chatID)
		sessionID := session.GetSessionID()
		if sessionID != "" {
			b.sendMessage(chatID, fmt.Sprintf(`📊 Session Status:
• Session ID: %s
• Working Directory: %s
• Status: Active`, sessionID, b.workingDir))
		} else {
			b.sendMessage(chatID, fmt.Sprintf(`📊 Session Status:
• Session ID: (none)
• Working Directory: %s
• Status: No active session`, b.workingDir))
		}

	case "help":
		b.sendMessage(chatID, `🤖 Claude Code Bot Help

Commands:
/new - Start a new session (clears previous context)
/clear - Clear the current session completely
/status - Show current session status
/help - Show this help message

Usage:
Simply send any message to interact with Claude Code. The bot will maintain your conversation context within a session.

Tips:
• Use /new to start fresh if Claude seems confused
• Each chat has its own session
• Claude Code runs in the configured working directory`)

	default:
		b.sendMessage(chatID, fmt.Sprintf("❓ Unknown command: /%s\nUse /help to see available commands.", command))
	}
}

// handleClaudeMessage sends a message to Claude and returns the response
func (b *Bot) handleClaudeMessage(ctx context.Context, chatID int64, text string) {
	// Log user input
	logger.Sugar.Infof("[%s] User input (chatID=%d): %s", b.name, chatID, text)

	// Send typing action
	typingAction := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	b.api.Send(typingAction)

	// Create a context with timeout for Claude
	claudeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Keep sending typing action while waiting
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				b.api.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
			}
		}
	}()

	session := b.sessionManager.GetOrCreateSession(chatID)
	response, err := session.SendMessage(claudeCtx, text)
	close(done)

	if err != nil {
		logger.Sugar.Errorf("[%s] Error from Claude (chatID=%d): %v", b.name, chatID, err)
		b.sendMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Log response being sent to user
	logger.Sugar.Infof("[%s] Response to user (chatID=%d): %s", b.name, chatID, truncateForLog(response, 500))

	// Send the response (splitting if necessary)
	b.sendLongMessage(chatID, response)
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// downloadFiles downloads all files from a message and returns their local paths
func (b *Bot) downloadFiles(ctx context.Context, msg *tgbotapi.Message) []string {
	var filePaths []string

	// Collect all file IDs from different message types
	var files []struct {
		fileID   string
		fileName string
	}

	// Photo (get the largest size)
	if msg.Photo != nil && len(msg.Photo) > 0 {
		largest := msg.Photo[len(msg.Photo)-1]
		files = append(files, struct {
			fileID   string
			fileName string
		}{largest.FileID, fmt.Sprintf("photo_%d.jpg", msg.MessageID)})
	}

	// Document
	if msg.Document != nil {
		fileName := msg.Document.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("document_%d", msg.MessageID)
		}
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.Document.FileID, fileName})
	}

	// Audio
	if msg.Audio != nil {
		fileName := msg.Audio.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("audio_%d.mp3", msg.MessageID)
		}
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.Audio.FileID, fileName})
	}

	// Video
	if msg.Video != nil {
		fileName := msg.Video.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("video_%d.mp4", msg.MessageID)
		}
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.Video.FileID, fileName})
	}

	// Voice
	if msg.Voice != nil {
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.Voice.FileID, fmt.Sprintf("voice_%d.ogg", msg.MessageID)})
	}

	// VideoNote
	if msg.VideoNote != nil {
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.VideoNote.FileID, fmt.Sprintf("videonote_%d.mp4", msg.MessageID)})
	}

	// Sticker
	if msg.Sticker != nil {
		ext := "webp"
		if msg.Sticker.IsAnimated {
			ext = "tgs"
		}
		files = append(files, struct {
			fileID   string
			fileName string
		}{msg.Sticker.FileID, fmt.Sprintf("sticker_%d.%s", msg.MessageID, ext)})
	}

	// Download each file
	for _, f := range files {
		localPath, err := b.downloadFile(ctx, f.fileID, f.fileName)
		if err != nil {
			logger.Sugar.Errorf("[%s] Failed to download file %s: %v", b.name, f.fileName, err)
			continue
		}
		filePaths = append(filePaths, localPath)
		logger.Sugar.Infof("[%s] Downloaded file: %s", b.name, localPath)
	}

	return filePaths
}

// downloadFile downloads a single file from Telegram and saves it to the temp directory
func (b *Bot) downloadFile(ctx context.Context, fileID, fileName string) (string, error) {
	// Get file info from Telegram
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := b.api.GetFile(fileConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Create unique filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	uniqueFileName := fmt.Sprintf("%s_%s", timestamp, fileName)
	localPath := filepath.Join(b.tempDir, uniqueFileName)

	// Download the file
	fileURL := file.Link(b.api.Token)
	logger.Sugar.Debugf("[%s] Downloading file from: %s", b.name, fileURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	// Create local file
	outFile, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	// Copy content
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return localPath, nil
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2
func escapeMarkdownV2(text string) string {
	// Characters that need to be escaped in MarkdownV2
	// Note: We don't escape *, _, `, [, ] to preserve basic markdown formatting
	specialChars := []string{"\\", "~", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// sendMessage sends a message to a chat
func (b *Bot) sendMessage(chatID int64, text string) {
	// Try MarkdownV2 first with escaped text
	escapedText := escapeMarkdownV2(text)
	msg := tgbotapi.NewMessage(chatID, escapedText)
	msg.ParseMode = "MarkdownV2"
	if _, err := b.api.Send(msg); err != nil {
		// Try without markdown if it fails
		msg.Text = text
		msg.ParseMode = ""
		if _, err := b.api.Send(msg); err != nil {
			logger.Sugar.Errorf("[%s] Failed to send message: %v", b.name, err)
		}
	}
}

// sendLongMessage sends a message that might exceed Telegram's length limit
func (b *Bot) sendLongMessage(chatID int64, text string) {
	if len(text) <= maxMessageLength {
		b.sendMessage(chatID, text)
		return
	}

	// Split the message into chunks
	chunks := splitMessage(text, maxMessageLength)
	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(100 * time.Millisecond) // Small delay between messages
		}
		b.sendMessage(chatID, chunk)
	}
}

// splitMessage splits a long message into chunks
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	var current strings.Builder

	for _, line := range lines {
		if current.Len()+len(line)+1 > maxLen {
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			// If a single line is too long, split it
			if len(line) > maxLen {
				for len(line) > maxLen {
					chunks = append(chunks, line[:maxLen])
					line = line[maxLen:]
				}
				if len(line) > 0 {
					current.WriteString(line)
				}
			} else {
				current.WriteString(line)
			}
		} else {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

// Manager manages multiple bots
type Manager struct {
	bots []*Bot
}

// NewManager creates a new bot manager
func NewManager() *Manager {
	return &Manager{
		bots: make([]*Bot, 0),
	}
}

// AddBot adds a bot to the manager
func (m *Manager) AddBot(bot *Bot) {
	m.bots = append(m.bots, bot)
}

// StartAll starts all bots concurrently
func (m *Manager) StartAll(ctx context.Context) error {
	if len(m.bots) == 0 {
		return fmt.Errorf("no bots to start")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(m.bots))

	for _, bot := range m.bots {
		wg.Add(1)
		go func(b *Bot) {
			defer wg.Done()
			if err := b.Start(ctx); err != nil && err != context.Canceled {
				errCh <- fmt.Errorf("bot %s error: %w", b.name, err)
			}
		}(bot)
	}

	// Wait for context cancellation or error
	go func() {
		wg.Wait()
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
