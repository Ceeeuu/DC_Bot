package commands

import (
	"cee_bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

// Definitions 回傳 general service 的所有 slash command 定義。
func Definitions() []commands.CommandDefinition {
	return []commands.CommandDefinition{
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "ping",
				Description: "檢查機器人是否正常運作",
			},
			Handler: HandlePing,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "hello",
				Description: "跟機器人打招呼",
			},
			Handler: HandleHello,
		},
	}
}
