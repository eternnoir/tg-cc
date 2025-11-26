package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// BotConfig represents a single bot configuration
type BotConfig struct {
	Name       string  `yaml:"name"`
	Token      string  `yaml:"token"`
	WorkingDir string  `yaml:"working_dir"`
	Whitelist  []int64 `yaml:"whitelist"`
}

// Config represents the entire application configuration
type Config struct {
	Bots []BotConfig `yaml:"bots"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	for i, bot := range cfg.Bots {
		if bot.Name == "" {
			return nil, fmt.Errorf("bot %d: name is required", i)
		}
		if bot.Token == "" {
			return nil, fmt.Errorf("bot %s: token is required", bot.Name)
		}
		if bot.WorkingDir == "" {
			return nil, fmt.Errorf("bot %s: working_dir is required", bot.Name)
		}

		// Check if working directory exists
		if _, err := os.Stat(bot.WorkingDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("bot %s: working directory %s does not exist", bot.Name, bot.WorkingDir)
		}
	}

	return &cfg, nil
}
