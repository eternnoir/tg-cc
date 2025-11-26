package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/eternnoir/tg-cc/internal/bot"
	"github.com/eternnoir/tg-cc/internal/config"
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
	if err := godotenv.Load(*envFile); err != nil {
		log.Printf("No .env file found at %s, using environment variables", *envFile)
	}

	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Loaded configuration with %d bot(s)", len(cfg.Bots))

	// Create bot manager
	manager := bot.NewManager()

	// Create bots from configuration
	for _, botCfg := range cfg.Bots {
		b, err := bot.New(botCfg)
		if err != nil {
			log.Fatalf("Failed to create bot %s: %v", botCfg.Name, err)
		}
		manager.AddBot(b)
		log.Printf("Created bot: %s (working dir: %s)", botCfg.Name, botCfg.WorkingDir)
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start all bots
	log.Println("Starting bots...")
	if err := manager.StartAll(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Bot manager error: %v", err)
	}

	log.Println("Shutdown complete")
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
