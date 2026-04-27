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

### 一般指令

| 指令 | 說明 |
|------|------|
| `/ping` | 確認機器人是否正常運作 |
| `/hello [name]` | 向機器人打招呼 |

### 淡江大學校務指令

使用這些指令前，需先透過 `/login` 或 `/credits` 儲存校務系統帳號密碼（僅保存於記憶體，重啟後清除）。

| 指令 | 說明 |
|------|------|
| `/login` | 開啟 Modal 輸入學號與密碼，儲存帳號密碼以供後續指令使用 |
| `/credits` | 查詢畢業學分達成狀況（必修、選修、總學分等），輸入帳密後同時儲存供後續使用 |
| `/schedule` | 查詢本學期課程表（科目、時間、教室），需先完成登入 |
| `/history` | 查詢歷年修課紀錄，需先完成登入 |

> **提示：** `/credits` 與 `/login` 皆會彈出 Modal 表單要求輸入學號及校務系統密碼。完成後即可直接使用 `/schedule`、`/history` 與 `/chat` 而無需重複輸入。

### AI 聊天功能

Cee_Bot 整合了 AI 模型（支援 Gemini 與 Groq），可透過以下兩種方式觸發：

**Slash Command：**

| 指令 | 說明 |
|------|------|
| `/chat 訊息:<內容>` | 向 AI 提問，若已登入校務系統，AI 可自動取得學分資料並回答學分相關問題 |

**自然語言觸發（訊息聊天）：**

- **@ 提及機器人**：在任意頻道 `@Cee_Bot <問題>` 即可觸發 AI 回應。
- **回覆機器人訊息**：直接回覆機器人的訊息，也會觸發 AI 並自動帶入上一則回覆內容作為脈絡。

> **學分問答：** 若已透過 `/login` 或 `/credits` 儲存帳號密碼，AI 在回答學分相關問題時會自動查詢校務系統，提供個人化的答覆。

## 架構說明

請參閱 [ARCHITECTURE.md](ARCHITECTURE.md)。
