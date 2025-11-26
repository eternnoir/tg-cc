package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/eternnoir/tg-cc/internal/bot"
	"github.com/eternnoir/tg-cc/internal/config"
	"github.com/eternnoir/tg-cc/internal/logger"
	"github.com/eternnoir/tg-cc/internal/storage"
)

var (
	version = "dev"
	envFile = flag.String("env", ".env", "Path to .env file")
	showHelp = flag.Bool("help", false, "Show help message")
	showVer  = flag.Bool("version", false, "Show version")
)

func main() {
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVer {
		fmt.Printf("tg-cc version %s\n", version)
		os.Exit(0)
	}

	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load(*envFile)

	// Initialize logger
	if err := logger.InitWithOptions(logger.GetLevelFromEnv(), logger.IsJSONFormatFromEnv()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Sugar.Debugf("Loaded .env file from %s", *envFile)

	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Sugar.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Sugar.Infof("Loaded configuration with %d bot(s)", len(cfg.Bots))

	// Initialize SQLite storage for session persistence
	store, err := storage.NewSQLiteStorage(cfg.DBPath)
	if err != nil {
		logger.Sugar.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	logger.Sugar.Infof("Session storage initialized at: %s", cfg.DBPath)

	// Create bot manager
	manager := bot.NewManager()

	// Create bots from configuration
	for _, botCfg := range cfg.Bots {
		b, err := bot.New(botCfg, store)
		if err != nil {
			logger.Sugar.Fatalf("Failed to create bot %s: %v", botCfg.Name, err)
		}
		manager.AddBot(b)
		logger.Sugar.Infof("Created bot: %s (working dir: %s)", botCfg.Name, botCfg.WorkingDir)
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Sugar.Infof("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start all bots
	logger.Sugar.Info("Starting bots...")
	if err := manager.StartAll(ctx); err != nil && err != context.Canceled {
		logger.Sugar.Fatalf("Bot manager error: %v", err)
	}

	logger.Sugar.Info("Shutdown complete")
}

func printUsage() {
	fmt.Println(`tg-cc - Telegram Claude Code Integration

Usage:
  tg-cc [options]

Options:
  -env string    Path to .env file (default ".env")
  -help          Show this help message
  -version       Show version

Environment Variables (Single Bot):
  BOT_TOKEN        Telegram bot token (required)
  BOT_WORKING_DIR  Working directory for Claude Code (required)
  BOT_NAME         Bot name (optional, default: "bot")
  BOT_WHITELIST    Comma-separated list of allowed Telegram user IDs (optional)

Storage:
  DB_PATH          Path to SQLite database file (optional, default: "sessions.db")

Environment Variables (Multiple Bots):
  BOT_COUNT          Number of bots (optional, auto-detected if not set)
  BOT_1_TOKEN        First bot's Telegram token
  BOT_1_WORKING_DIR  First bot's working directory
  BOT_1_NAME         First bot's name (optional)
  BOT_1_WHITELIST    First bot's whitelist (optional)
  BOT_2_TOKEN        Second bot's Telegram token
  ... and so on

Bot Commands:
  /start   - Start the bot and show welcome message
  /new     - Start a new session (clears previous context)
  /clear   - Clear the current session
  /status  - Show current session status
  /help    - Show help message

Environment Requirements:
  - Claude CLI must be installed and configured
  - Each working directory must exist and be accessible

For more information, visit: https://github.com/eternnoir/tg-cc`)
}
