package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord  DiscordConfig  `yaml:"discord"`
	AI       AIConfig       `yaml:"ai"`
	Gemini   GeminiConfig   `yaml:"gemini"`
	Groq     GroqConfig     `yaml:"groq"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
}

// AIConfig 控制使用哪個 AI provider。
type AIConfig struct {
	Provider string `yaml:"provider"` // "gemini" | "groq"
}

type GroqConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type DiscordConfig struct {
	Token         string `yaml:"token"`
	GuildID       string `yaml:"guild_id"`
	ApplicationID string `yaml:"application_id"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

func Load() (*Config, error) {
	cfg := &Config{
		AI:       AIConfig{Provider: "gemini"},
		Gemini:   GeminiConfig{Model: "gemini-2.0-flash"},
		Groq:     GroqConfig{Model: "llama-3.3-70b-versatile"},
		Database: DatabaseConfig{Path: "bot.db"},
		Log:      LogConfig{Level: "info"},
	}

	data, err := os.ReadFile("config/config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
		}
	}

	// 環境變數覆蓋 config 檔設定
	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		cfg.Discord.Token = token
	}
	if guildID := os.Getenv("DISCORD_GUILD_ID"); guildID != "" {
		cfg.Discord.GuildID = guildID
	}
	if appID := os.Getenv("DISCORD_APPLICATION_ID"); appID != "" {
		cfg.Discord.ApplicationID = appID
	}

	if cfg.Discord.Token == "" {
		return nil, fmt.Errorf("discord token is required: set DISCORD_TOKEN env or config/config.yaml")
	}

	return cfg, nil
}
