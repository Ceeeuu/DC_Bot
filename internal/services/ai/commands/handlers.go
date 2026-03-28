package commands

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// ChatClient 是所有 AI provider 共同實作的介面。
type ChatClient interface {
	Chat(ctx context.Context, userID, message string) (string, error)
	Close()
}

var chatClient ChatClient

// SetClient 由 Service 在初始化時注入選定的 AI client。
func SetClient(c ChatClient) {
	chatClient = c
}

func HandleChat(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	message := optionMap["訊息"].StringValue()
	userID := i.Member.User.ID

	reply, err := chatClient.Chat(context.Background(), userID, message)
	if err != nil {
		errMsg := fmt.Sprintf("❌ 發生錯誤：%s", err.Error())
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errMsg,
		})
		return
	}

	if len(reply) > 2000 {
		reply = reply[:1997] + "..."
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &reply,
	})
}
