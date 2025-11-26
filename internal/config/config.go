package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// BotConfig represents a single bot configuration
type BotConfig struct {
	Name       string
	Token      string
	WorkingDir string
	Whitelist  []int64
}

// Config represents the entire application configuration
type Config struct {
	Bots []BotConfig
}

// LoadFromEnv reads configuration from environment variables
// Environment variable format:
//
//	BOT_COUNT=2 (optional, defaults to 1)
//	BOT_1_NAME=my-bot
//	BOT_1_TOKEN=your-telegram-token
//	BOT_1_WORKING_DIR=/path/to/project
//	BOT_1_WHITELIST=123456789,987654321
//
// For single bot, you can also use:
//
//	BOT_NAME=my-bot
//	BOT_TOKEN=your-telegram-token
//	BOT_WORKING_DIR=/path/to/project
//	BOT_WHITELIST=123456789,987654321
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Bots: make([]BotConfig, 0),
	}

	// Check if using single bot format (no BOT_COUNT, just BOT_*)
	if os.Getenv("BOT_TOKEN") != "" {
		bot, err := loadBotFromEnv("BOT_")
		if err != nil {
			return nil, err
		}
		cfg.Bots = append(cfg.Bots, bot)
		return cfg, nil
	}

	// Multi-bot format with BOT_COUNT
	countStr := os.Getenv("BOT_COUNT")
	if countStr == "" {
		// Try to auto-detect bots by looking for BOT_1_TOKEN, BOT_2_TOKEN, etc.
		for i := 1; ; i++ {
			prefix := fmt.Sprintf("BOT_%d_", i)
			if os.Getenv(prefix+"TOKEN") == "" {
				break
			}
			bot, err := loadBotFromEnv(prefix)
			if err != nil {
				return nil, err
			}
			cfg.Bots = append(cfg.Bots, bot)
		}
	} else {
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return nil, fmt.Errorf("invalid BOT_COUNT: %w", err)
		}

		for i := 1; i <= count; i++ {
			prefix := fmt.Sprintf("BOT_%d_", i)
			bot, err := loadBotFromEnv(prefix)
			if err != nil {
				return nil, err
			}
			cfg.Bots = append(cfg.Bots, bot)
		}
	}

	if len(cfg.Bots) == 0 {
		return nil, fmt.Errorf("no bots configured. Set BOT_TOKEN for single bot or BOT_1_TOKEN for multiple bots")
	}

	return cfg, nil
}

// loadBotFromEnv loads a single bot configuration from environment variables with given prefix
func loadBotFromEnv(prefix string) (BotConfig, error) {
	bot := BotConfig{}

	// Get required fields
	bot.Token = os.Getenv(prefix + "TOKEN")
	if bot.Token == "" {
		return bot, fmt.Errorf("%sTOKEN is required", prefix)
	}

	bot.WorkingDir = os.Getenv(prefix + "WORKING_DIR")
	if bot.WorkingDir == "" {
		return bot, fmt.Errorf("%sWORKING_DIR is required", prefix)
	}

	// Check if working directory exists
	if _, err := os.Stat(bot.WorkingDir); os.IsNotExist(err) {
		return bot, fmt.Errorf("working directory %s does not exist", bot.WorkingDir)
	}

	// Get optional fields
	bot.Name = os.Getenv(prefix + "NAME")
	if bot.Name == "" {
		// Generate default name from prefix or working dir
		if prefix != "" {
			bot.Name = strings.TrimSuffix(prefix, "_")
		} else {
			bot.Name = "bot"
		}
	}

	// Parse whitelist
	whitelistStr := os.Getenv(prefix + "WHITELIST")
	if whitelistStr != "" {
		ids := strings.Split(whitelistStr, ",")
		for _, idStr := range ids {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return bot, fmt.Errorf("invalid user ID in %sWHITELIST: %s", prefix, idStr)
			}
			bot.Whitelist = append(bot.Whitelist, id)
		}
	}

	return bot, nil
}
