# Cee_Bot

Discord 機器人，使用 Go 撰寫，所有指令皆透過 Slash Commands 輸入。

## 快速啟動

**1. 設定**

```bash
cp config/config.example.yaml config/config.yaml
```

編輯 `config/config.yaml`，填入 Bot Token 與 Application ID。

**2. 安裝依賴**

```bash
go mod tidy
```

**3. 執行**

```bash
go run ./cmd/bot
```

## 環境變數

可用環境變數取代 config.yaml：

```bash
DISCORD_TOKEN=your_token
DISCORD_GUILD_ID=your_guild_id   # 留空為全域指令
DISCORD_APPLICATION_ID=your_app_id
```

## 可用指令

| 指令 | 說明 |
|------|------|
| `/ping` | 確認機器人是否正常運作 |
| `/hello [name]` | 向機器人打招呼 |

## 架構說明

請參閱 [ARCHITECTURE.md](ARCHITECTURE.md)。
