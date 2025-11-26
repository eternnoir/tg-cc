# tg-cc

Telegram Claude Code Integration - 透過 Telegram Bot 與 Claude Code 互動的工具。

## 功能特點

- **多專案支援**: 每個目錄可以對應一個獨立的 Telegram Bot
- **Session 管理**: 支援延續、清除和重新建立對話 Session
- **白名單控制**: 可設定允許使用 Bot 的 Telegram 用戶 ID
- **並行運行**: 支援同時運行多個 Bot

## 系統需求

- Go 1.21 或更高版本
- [Claude CLI](https://github.com/anthropics/claude-code) 已安裝並設定完成
- Telegram Bot Token (從 [@BotFather](https://t.me/botfather) 取得)

## 安裝

```bash
# 複製專案
git clone https://github.com/eternnoir/tg-cc.git
cd tg-cc

# 編譯
go build -o tg-cc ./cmd/tg-cc

# 或直接安裝
go install ./cmd/tg-cc
```

## 設定

1. 複製範例設定檔:

```bash
cp config.example.yaml config.yaml
```

2. 編輯 `config.yaml`:

```yaml
bots:
  - name: "my-project-bot"
    token: "YOUR_TELEGRAM_BOT_TOKEN"
    working_dir: "/path/to/your/project"
    whitelist:
      - 123456789  # 你的 Telegram User ID
```

### 取得 Telegram User ID

可以透過以下方式取得你的 Telegram User ID:
- 傳訊息給 [@userinfobot](https://t.me/userinfobot)
- 傳訊息給 [@getmyid_bot](https://t.me/getmyid_bot)

### 建立 Telegram Bot

1. 在 Telegram 中搜尋 [@BotFather](https://t.me/botfather)
2. 發送 `/newbot` 指令
3. 按照指示設定 Bot 名稱
4. 複製取得的 Token 到設定檔

## 使用方式

```bash
# 使用預設設定檔 (config.yaml)
./tg-cc

# 指定設定檔
./tg-cc -config /path/to/config.yaml

# 顯示版本
./tg-cc -version

# 顯示說明
./tg-cc -help
```

## Bot 指令

| 指令 | 說明 |
|------|------|
| `/start` | 開始使用並顯示歡迎訊息 |
| `/new` | 開始新的對話 Session |
| `/clear` | 清除目前的 Session |
| `/status` | 顯示目前的 Session 狀態 |
| `/help` | 顯示說明訊息 |

## 使用範例

1. 設定完成後啟動程式
2. 在 Telegram 中搜尋你的 Bot
3. 發送 `/start` 開始
4. 直接輸入訊息與 Claude Code 互動

例如:
- "請列出這個專案的目錄結構"
- "幫我找出所有的 TODO 註解"
- "解釋 main.go 的功能"

## 設定範例：多個 Bot

```yaml
bots:
  # 前端專案
  - name: "frontend-bot"
    token: "BOT_TOKEN_1"
    working_dir: "/home/user/projects/frontend"
    whitelist:
      - 123456789

  # 後端專案
  - name: "backend-bot"
    token: "BOT_TOKEN_2"
    working_dir: "/home/user/projects/backend"
    whitelist:
      - 123456789
      - 987654321
```

## 注意事項

- 確保 Claude CLI 已正確安裝並可以在終端機中執行 `claude` 指令
- 白名單為空時，所有用戶都可以使用 Bot（不建議在生產環境使用）
- 每個對話都有獨立的 Session，不同的 Chat 不會互相干擾
- Claude Code 的回應可能需要一些時間，請耐心等待

## 授權

MIT License
