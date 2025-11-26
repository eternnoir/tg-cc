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

	"github.com/eternnoir/tg-cc/internal/logger"
)

// Session represents a Claude Code session for a specific chat
type Session struct {
	mu         sync.Mutex
	chatID     int64
	workingDir string
	sessionID  string
	claudeArgs []string
}

// SessionManager manages Claude Code sessions for different chats
type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[int64]*Session
	workingDir string
	claudeArgs []string
}

// NewSessionManager creates a new session manager for a working directory
func NewSessionManager(workingDir string, claudeArgs []string) *SessionManager {
	return &SessionManager{
		sessions:   make(map[int64]*Session),
		workingDir: workingDir,
		claudeArgs: claudeArgs,
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
		claudeArgs: m.claudeArgs,
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

	// Add custom Claude arguments from config
	if len(s.claudeArgs) > 0 {
		args = append(args, s.claudeArgs...)
	}

	// If we have an existing session, resume it
	if s.sessionID != "" {
		args = append(args, "--resume", s.sessionID)
	}

	// Add the prompt
	args = append(args, "--print", message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = s.workingDir

	// Log the command being executed
	logger.Sugar.Debugf("[claude] Executing command: claude %s (workingDir=%s)", strings.Join(args, " "), s.workingDir)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Sugar.Errorf("[claude] Failed to create stdout pipe: %v", err)
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Sugar.Errorf("[claude] Failed to create stderr pipe: %v", err)
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		logger.Sugar.Errorf("[claude] Failed to start claude command: %v", err)
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	logger.Sugar.Debugf("[claude] Claude process started (pid=%d)", cmd.Process.Pid)

	// Read stderr for error messages
	var stderrBuf bytes.Buffer
	go func() {
		io.Copy(&stderrBuf, stderr)
	}()

	// Parse JSON stream response
	var responseBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)

	// Increase scanner buffer for large responses (e.g., base64 encoded images)
	const maxScanTokenSize = 50 * 1024 * 1024 // 50MB
	buf := make([]byte, 64*1024)              // Start with 64KB initial buffer
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var resp claudeResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Not JSON, might be raw output
			logger.Sugar.Debugf("[claude] Non-JSON output: %s", line)
			continue
		}

		// Log raw JSON response at debug level
		logger.Sugar.Debugf("[claude] Response (type=%s): %s", resp.Type, truncateString(line, 500))

		switch resp.Type {
		case "system":
			// System message, might contain session ID
			if resp.SessionID != "" {
				s.sessionID = resp.SessionID
				logger.Sugar.Debugf("[claude] Session ID set: %s", s.sessionID)
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
				logger.Sugar.Debugf("[claude] Session ID updated: %s", s.sessionID)
			}
		case "tool_use":
			// Tool being used - we can show this as progress
			if resp.ToolName != "" {
				logger.Sugar.Debugf("[claude] Tool use: %s", resp.ToolName)
				responseBuilder.WriteString(fmt.Sprintf("\n🔧 Using tool: %s\n", resp.ToolName))
			}
		case "tool_result":
			// Tool result - optional to show
			logger.Sugar.Debugf("[claude] Tool result received")
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		logger.Sugar.Errorf("[claude] Scanner error while reading stdout: %v", scanErr)
	}

	if err := cmd.Wait(); err != nil {
		stderrContent := stderrBuf.String()
		if stderrContent != "" {
			logger.Sugar.Errorf("[claude] Command failed with stderr: %s, error: %v", stderrContent, err)
			return "", fmt.Errorf("claude command failed: %w\nstderr: %s", err, stderrContent)
		}
		logger.Sugar.Errorf("[claude] Command failed: %v", err)
		return "", fmt.Errorf("claude command failed: %w", err)
	}

	response := responseBuilder.String()
	if response == "" {
		logger.Sugar.Warnf("[claude] Empty response from Claude")
		return "No response from Claude.", nil
	}

	logger.Sugar.Debugf("[claude] Final response length: %d bytes", len(response))
	return response, nil
}

// truncateString truncates a string for logging purposes
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
