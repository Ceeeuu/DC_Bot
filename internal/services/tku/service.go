package tku

import (
	"cee_bot/config"
	routercmds "cee_bot/internal/commands"
	tkucmds "cee_bot/internal/services/tku/commands"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

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

func (s *Service) Commands() []routercmds.CommandDefinition {
	return tkucmds.Definitions()
}

func (s *Service) Modals() []routercmds.ModalDefinition {
	return tkucmds.ModalDefinitions()
}
