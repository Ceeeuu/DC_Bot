package main

import (
	"os"
	"os/signal"
	"syscall"

	"cee_bot/config"
	"cee_bot/internal/commands"
	"cee_bot/internal/credstore"
	"cee_bot/internal/services/ai"
	"cee_bot/internal/services/general"
	"cee_bot/internal/services/tku"
	"cee_bot/internal/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := utils.NewLogger(cfg.Log.Level)
	logger.Info("Starting Cee_Bot...")

	// 初始化 SQLite 資料庫
	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{})
	if err != nil {
		logger.Fatalf("failed to open database: %v", err)
	}
	db.AutoMigrate(&credstore.UserCredential{})

	// 將 credstore 切換為 DB 實作
	credstore.Default = credstore.NewGormStore(db)
	logger.Infof("Database initialized: %s", cfg.Database.Path)

	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		logger.Fatalf("failed to create discord session: %v", err)
	}
	// IntentsGuildMessages + IntentMessageContent 用於接收並讀取訊息內容
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentMessageContent

	// 初始化各 service
	generalService := general.New(session, cfg, logger)
	tkuService := tku.New(session, cfg, logger)

	aiService, err := ai.New(session, cfg, logger)
	if err != nil {
		logger.Fatalf("failed to init AI service: %v", err)
	}
	defer aiService.Close()

	// 建立指令路由器，集中管理所有 slash commands
	router := commands.NewRouter(session, cfg.Discord.GuildID, logger)
	router.Register(generalService.Commands())
	router.Register(tkuService.Commands())
	router.Register(aiService.Commands())
	router.RegisterModals(tkuService.Modals())

	if err := session.Open(); err != nil {
		logger.Fatalf("failed to open discord session: %v", err)
	}
	defer session.Close()

	if err := router.Sync(); err != nil {
		logger.Fatalf("failed to sync slash commands: %v", err)
	}

	logger.Info("Cee_Bot is running. Press CTRL+C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logger.Info("Shutting down Cee_Bot...")
	router.Cleanup()
}
