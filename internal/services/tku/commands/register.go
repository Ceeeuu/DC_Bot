package commands

import (
	"cee_bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

const (
	CreditsModalID = "credits_modal"
	LoginModalID   = "login_modal"
)

func Definitions() []commands.CommandDefinition {
	return []commands.CommandDefinition{
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "credits",
				Description: "查詢淡江大學畢業學分狀況（同時儲存帳密供 /chat 使用）",
			},
			Handler: HandleCredits,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "login",
				Description: "儲存淡江大學帳號密碼，讓 /chat 可以自動查詢學分",
			},
			Handler: HandleLogin,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "schedule",
				Description: "查詢本學期課程表（時間、教室）",
			},
			Handler: HandleSchedule,
		},
		{
			Command: &discordgo.ApplicationCommand{
				Name:        "history",
				Description: "查詢歷年修課紀錄",
			},
			Handler: HandleHistory,
		},
	}
}

func ModalDefinitions() []commands.ModalDefinition {
	return []commands.ModalDefinition{
		{CustomID: CreditsModalID, Handler: HandleCreditsModal},
		{CustomID: LoginModalID, Handler: HandleLoginModal},
	}
}
