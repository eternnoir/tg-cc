package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Session represents a Claude Code session for a specific chat
type Session struct {
	mu         sync.Mutex
	chatID     int64
	workingDir string
	sessionID  string
}

// SessionManager manages Claude Code sessions for different chats
type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[int64]*Session
	workingDir string
}

// NewSessionManager creates a new session manager for a working directory
func NewSessionManager(workingDir string) *SessionManager {
	return &SessionManager{
		sessions:   make(map[int64]*Session),
		workingDir: workingDir,
	}
}

// GetOrCreateSession gets an existing session or creates a new one for a chat
func (m *SessionManager) GetOrCreateSession(chatID int64) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[chatID]; ok {
		return session
	}

	session := &Session{
		chatID:     chatID,
		workingDir: m.workingDir,
	}
	m.sessions[chatID] = session
	return session
}

// ClearSession clears the session for a chat
func (m *SessionManager) ClearSession(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, chatID)
}

// ResetSession resets the session for a chat (clears session ID but keeps the session)
func (m *SessionManager) ResetSession(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[chatID]; ok {
		session.mu.Lock()
		session.sessionID = ""
		session.mu.Unlock()
	}
}

// claudeResponse represents a response from Claude CLI in JSON mode
type claudeResponse struct {
	Type       string `json:"type"`
	SessionID  string `json:"session_id,omitempty"`
	Message    string `json:"message,omitempty"`
	Content    string `json:"content,omitempty"`
	Result     string `json:"result,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  any    `json:"tool_input,omitempty"`
	ToolResult any    `json:"tool_result,omitempty"`
}

// SendMessage sends a message to Claude Code and returns the response
func (s *Session) SendMessage(ctx context.Context, message string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{
		"--output-format", "stream-json",
		"--verbose",
	}

	// If we have an existing session, resume it
	if s.sessionID != "" {
		args = append(args, "--resume", s.sessionID)
	}

	// Add the prompt
	args = append(args, "--print", message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = s.workingDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Read stderr for error messages
	var stderrBuf bytes.Buffer
	go func() {
		io.Copy(&stderrBuf, stderr)
	}()

	// Parse JSON stream response
	var responseBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)

	// Increase scanner buffer for large responses
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var resp claudeResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Not JSON, might be raw output
			continue
		}

		switch resp.Type {
		case "system":
			// System message, might contain session ID
			if resp.SessionID != "" {
				s.sessionID = resp.SessionID
			}
		case "assistant":
			// Assistant response content
			if resp.Content != "" {
				responseBuilder.WriteString(resp.Content)
			}
			if resp.Message != "" {
				responseBuilder.WriteString(resp.Message)
			}
		case "result":
			// Final result
			if resp.Result != "" {
				if responseBuilder.Len() > 0 {
					responseBuilder.WriteString("\n")
				}
				responseBuilder.WriteString(resp.Result)
			}
			if resp.SessionID != "" {
				s.sessionID = resp.SessionID
			}
		case "tool_use":
			// Tool being used - we can show this as progress
			if resp.ToolName != "" {
				responseBuilder.WriteString(fmt.Sprintf("\n🔧 Using tool: %s\n", resp.ToolName))
			}
		case "tool_result":
			// Tool result - optional to show
		}
	}

	if err := cmd.Wait(); err != nil {
		stderrContent := stderrBuf.String()
		if stderrContent != "" {
			return "", fmt.Errorf("claude command failed: %w\nstderr: %s", err, stderrContent)
		}
		return "", fmt.Errorf("claude command failed: %w", err)
	}

	response := responseBuilder.String()
	if response == "" {
		return "No response from Claude.", nil
	}

	return response, nil
}

// GetSessionID returns the current session ID
func (s *Session) GetSessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// HasSession returns true if there's an active session
func (s *Session) HasSession() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID != ""
}
