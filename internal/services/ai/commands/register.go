package commands

import (
	"cee_bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

func Definitions() []commands.CommandDefinition {
	return []commands.CommandDefinition{
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "chat",
				Description: "用自然語言與 教授 聊天，可詢問學分等相關問題",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "訊息",
						Description: "你想問的問題或說的話",
						Required:    true,
					},
				},
			},
			Handler: HandleChat,
		},
	}
}
