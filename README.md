# tg-cc

Telegram Claude Code Integration - A tool for interacting with Claude Code through Telegram Bot.

## Features

- **Multi-project Support**: Each directory can correspond to an independent Telegram Bot
- **Session Management**: Support for continuing, clearing, and recreating conversation sessions
- **Whitelist Control**: Configure allowed Telegram user IDs
- **Parallel Execution**: Support running multiple bots simultaneously
- **Environment Variable Configuration**: Use `.env` files or environment variables for configuration

## Requirements

- Go 1.21 or higher
- [Claude CLI](https://github.com/anthropics/claude-code) installed and configured
- Telegram Bot Token (obtain from [@BotFather](https://t.me/botfather))

## Installation

```bash
# Clone the project
git clone https://github.com/eternnoir/tg-cc.git
cd tg-cc

# Build
go build -o tg-cc ./cmd/tg-cc

# Or install directly
go install ./cmd/tg-cc
```

## Configuration

1. Copy the example configuration file:

```bash
cp .env.example .env
```

2. Edit the `.env` file:

### Single Bot Configuration

```env
BOT_TOKEN=your_telegram_bot_token_here
BOT_WORKING_DIR=/path/to/your/project
BOT_NAME=my-project-bot
BOT_WHITELIST=123456789,987654321
```

### Multiple Bot Configuration

```env
BOT_1_TOKEN=first_bot_token
BOT_1_WORKING_DIR=/path/to/first/project
BOT_1_NAME=frontend-bot
BOT_1_WHITELIST=123456789

BOT_2_TOKEN=second_bot_token
BOT_2_WORKING_DIR=/path/to/second/project
BOT_2_NAME=backend-bot
BOT_2_WHITELIST=123456789,987654321
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `BOT_TOKEN` | Telegram Bot Token | Yes |
| `BOT_WORKING_DIR` | Claude Code working directory | Yes |
| `BOT_NAME` | Bot name | No |
| `BOT_WHITELIST` | Allowed user IDs (comma-separated) | No |

For multiple bots, use `BOT_1_*`, `BOT_2_*` prefixes.

### Getting Your Telegram User ID

You can get your Telegram User ID through:
- Send a message to [@userinfobot](https://t.me/userinfobot)
- Send a message to [@getmyid_bot](https://t.me/getmyid_bot)

### Creating a Telegram Bot

1. Search for [@BotFather](https://t.me/botfather) in Telegram
2. Send the `/newbot` command
3. Follow the instructions to set up your bot name
4. Copy the token to your `.env` file

## Usage

```bash
# Use default .env file
./tg-cc

# Specify .env file path
./tg-cc -env /path/to/.env

# Show version
./tg-cc -version

# Show help
./tg-cc -help
```

You can also set environment variables directly without using a `.env` file:

```bash
BOT_TOKEN=xxx BOT_WORKING_DIR=/path/to/project ./tg-cc
```

## Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Start the bot and show welcome message |
| `/new` | Start a new conversation session |
| `/clear` | Clear the current session |
| `/status` | Show current session status |
| `/help` | Show help message |

## Example Usage

1. Start the program after configuration
2. Search for your bot in Telegram
3. Send `/start` to begin
4. Type messages directly to interact with Claude Code

Examples:
- "List the directory structure of this project"
- "Find all TODO comments"
- "Explain what main.go does"

## Notes

- Ensure Claude CLI is properly installed and the `claude` command is accessible from the terminal
- When the whitelist is empty, all users can use the bot (not recommended for production)
- Each conversation has its own session; different chats do not interfere with each other
- Claude Code responses may take some time, please be patient

## License

MIT License
