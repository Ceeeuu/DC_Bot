package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandlePing(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong! 🏓",
		},
	})
}

func HandleHello(s *discordgo.Session, i *discordgo.InteractionCreate) {
	name := i.Member.User.Username
	if i.Member.Nick != "" {
		name = i.Member.Nick
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("你好，%s！有什麼我可以幫你的嗎？👋", name),
		},
	})
}
