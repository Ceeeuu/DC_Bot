package general

import (
	"cee_bot/config"
	routercmds "cee_bot/internal/commands"
	generalcmds "cee_bot/internal/services/general/commands"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Service 負責管理 general/utility 類別的指令。
type Service struct {
	session *discordgo.Session
	cfg     *config.Config
	logger  *logrus.Logger
}

func New(session *discordgo.Session, cfg *config.Config, logger *logrus.Logger) *Service {
	return &Service{
		session: session,
		cfg:     cfg,
		logger:  logger,
	}
}

// Commands 回傳此 service 的所有 slash command 定義，供 Router 統一註冊。
func (s *Service) Commands() []routercmds.CommandDefinition {
	return generalcmds.Definitions()
}
