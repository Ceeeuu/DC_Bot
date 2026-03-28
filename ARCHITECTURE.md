# Cee_Bot 專案架構說明（Go）

此專案為 Discord Bot，採用 Go 撰寫並以模組化方式組織功能，所有使用者指令皆透過 **Slash Commands** 輸入。

## 目錄結構

```
Cee_Bot/
├── cmd/
│   └── bot/
│       └── main.go          # 啟動點：初始化 session、service、router
├── config/
│   ├── config.go            # Config 結構與載入邏輯
│   └── config.example.yaml  # 範本設定檔（複製為 config.yaml 使用）
├── internal/
│   ├── commands/
│   │   └── router.go        # 集中管理所有 slash command 的註冊與路由
│   ├── services/            # 各功能模組
│   │   └── general/         # 範例：通用指令模組
│   │       ├── commands/
│   │       │   ├── register.go  # 此模組的指令定義（ApplicationCommand）
│   │       │   └── handlers.go  # 各指令的 handler 函式
│   │       └── service.go       # 模組入口，實作 Commands() 方法
│   └── utils/
│       └── logger.go        # logrus logger 初始化
├── docs/                    # 文件與使用說明
├── go.mod
├── .gitignore
└── README.md
```

## 核心概念

### Slash Command 流程

```
使用者輸入 /指令
    → Discord API → session InteractionCreate 事件
    → Router 依指令名稱路由
    → 對應 handler 處理並回應
```

### CommandDefinition

`internal/commands/router.go` 定義的核心型別：

```go
type CommandDefinition struct {
    Command *discordgo.ApplicationCommand  // Discord 指令定義（名稱、描述、選項）
    Handler func(s *discordgo.Session, i *discordgo.InteractionCreate)  // 處理函式
}
```

每個 service 透過 `Commands() []CommandDefinition` 方法回傳自己的指令列表，由 Router 統一向 Discord API 註冊。

### 新增功能模組的步驟

1. 在 `internal/services/` 下建立新目錄，例如 `music/`
2. 建立 `music/commands/register.go`：定義指令清單（`Definitions() []commands.CommandDefinition`）
3. 建立 `music/commands/handlers.go`：實作各指令 handler
4. 建立 `music/service.go`：實作 `Commands()` 方法
5. 在 `cmd/bot/main.go` 加入：
   ```go
   musicService := music.New(session, cfg, logger)
   router.Register(musicService.Commands())
   ```

## 程式架構準則

- **模組化**：每個功能模組獨立於 `internal/services/`，易於新增與維護
- **關注點分離**：指令定義（register.go）、處理邏輯（handlers.go）、服務生命週期（service.go）各自獨立
- **集中路由**：`internal/commands/router.go` 是唯一與 Discord 互動的指令管理層
- **集中設定**：所有設定集中於 `config/`，支援 config.yaml 與環境變數雙來源

## 建議函式庫

| 套件 | 用途 |
|------|------|
| `github.com/bwmarrin/discordgo` | Discord API client |
| `github.com/samber/lo` | 實用工具（Ternary、ToPtr、FromPtr 等）|
| `github.com/sirupsen/logrus` | 結構化日誌 |
| `gopkg.in/yaml.v3` | YAML 設定檔解析 |
| `gorm.io/gorm` | ORM（需要資料庫時加入）|
| `github.com/go-resty/resty/v2` | HTTP 請求（呼叫外部 API 時加入）|

## 特殊邏輯處理

- 使用 `lo.Ternary` 取代 if-else 設值
- string 為 pointer 時，使用 `lo.ToPtr` 與 `lo.FromPtr` 轉換
- 可使用 `lo.Associate` 將 slice 轉換為 map

## 設定管理

複製 `config/config.example.yaml` 為 `config/config.yaml` 並填入實際憑證，或設定環境變數：

```bash
export DISCORD_TOKEN="你的 Bot Token"
export DISCORD_GUILD_ID="測試用 Guild ID（留空為全域指令）"
```

## 快速啟動

```bash
cp config/config.example.yaml config/config.yaml
# 編輯 config/config.yaml 填入 Token
go mod tidy
go run ./cmd/bot
```
