package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/eternnoir/tg-cc/internal/claude"
	"github.com/eternnoir/tg-cc/internal/config"
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
}

// New creates a new Bot instance
func New(cfg config.BotConfig) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	return &Bot{
		name:           cfg.Name,
		api:            api,
		whitelist:      whitelist.New(cfg.Whitelist),
		sessionManager: claude.NewSessionManager(cfg.WorkingDir),
		workingDir:     cfg.WorkingDir,
	}, nil
}

// Start starts the bot and begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	log.Printf("[%s] Bot started. Working directory: %s", b.name, b.workingDir)
	log.Printf("[%s] Bot username: @%s", b.name, b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Bot stopping...", b.name)
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
		log.Printf("[%s] Unauthorized user %d attempted to use the bot", b.name, userID)
		b.sendMessage(chatID, "⛔ You are not authorized to use this bot.")
		return
	}

	text := msg.Text

	// Handle commands
	if msg.IsCommand() {
		b.handleCommand(ctx, msg)
		return
	}

	// Handle regular messages - send to Claude
	if text == "" {
		return
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
		log.Printf("[%s] Error from Claude: %v", b.name, err)
		b.sendMessage(chatID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Send the response (splitting if necessary)
	b.sendLongMessage(chatID, response)
}

// sendMessage sends a message to a chat
func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if _, err := b.api.Send(msg); err != nil {
		// Try without markdown if it fails
		msg.ParseMode = ""
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("[%s] Failed to send message: %v", b.name, err)
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
