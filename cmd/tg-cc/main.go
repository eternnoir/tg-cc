package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eternnoir/tg-cc/internal/bot"
	"github.com/eternnoir/tg-cc/internal/config"
)

var (
	version    = "dev"
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	showHelp   = flag.Bool("help", false, "Show help message")
	showVer    = flag.Bool("version", false, "Show version")
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

	// Load configuration
	cfg, err := config.Load(*configPath)
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
  -config string    Path to configuration file (default "config.yaml")
  -help             Show this help message
  -version          Show version

Configuration File Format (YAML):
  bots:
    - name: "my-project-bot"
      token: "YOUR_TELEGRAM_BOT_TOKEN"
      working_dir: "/path/to/your/project"
      whitelist:
        - 123456789  # Telegram user ID
        - 987654321

Bot Commands:
  /start   - Start the bot and show welcome message
  /new     - Start a new session (clears previous context)
  /clear   - Clear the current session
  /status  - Show current session status
  /help    - Show help message

Environment Requirements:
  - Claude CLI must be installed and configured
  - Each working_dir must exist and be accessible

For more information, visit: https://github.com/eternnoir/tg-cc`)
}
