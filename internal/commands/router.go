package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// CommandDefinition 將 ApplicationCommand 與其 handler 綁定在一起。
type CommandDefinition struct {
	Command *discordgo.ApplicationCommand
	Handler func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// ModalDefinition 將 Modal CustomID 與其 submit handler 綁定在一起。
type ModalDefinition struct {
	CustomID string
	Handler  func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// Router 負責集中管理 slash command 與 modal 的註冊與路由。
type Router struct {
	session    *discordgo.Session
	guildID    string
	logger     *logrus.Logger
	defs       []CommandDefinition
	modalDefs  []ModalDefinition
	registered []*discordgo.ApplicationCommand
}

func NewRouter(session *discordgo.Session, guildID string, logger *logrus.Logger) *Router {
	return &Router{
		session: session,
		guildID: guildID,
		logger:  logger,
	}
}

// Register 從各 service 收集指令定義。
func (r *Router) Register(defs []CommandDefinition) {
	r.defs = append(r.defs, defs...)
}

// RegisterModals 從各 service 收集 modal submit 定義。
func (r *Router) RegisterModals(defs []ModalDefinition) {
	r.modalDefs = append(r.modalDefs, defs...)
}

// Sync 向 Discord 註冊所有指令，並設定統一的 InteractionCreate handler。
// 須在 session.Open() 之後呼叫。
func (r *Router) Sync() error {
	handlers := make(map[string]func(*discordgo.Session, *discordgo.InteractionCreate))
	for _, def := range r.defs {
		handlers[def.Command.Name] = def.Handler
	}

	modalHandlers := make(map[string]func(*discordgo.Session, *discordgo.InteractionCreate))
	for _, def := range r.modalDefs {
		modalHandlers[def.CustomID] = def.Handler
	}

	r.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			name := i.ApplicationCommandData().Name
			if h, ok := handlers[name]; ok {
				h(s, i)
			} else {
				r.logger.Warnf("no handler registered for command: /%s", name)
			}
		case discordgo.InteractionModalSubmit:
			customID := i.ModalSubmitData().CustomID
			if h, ok := modalHandlers[customID]; ok {
				h(s, i)
			} else {
				r.logger.Warnf("no handler registered for modal: %s", customID)
			}
		}
	})

	for _, def := range r.defs {
		cmd, err := r.session.ApplicationCommandCreate(r.session.State.User.ID, r.guildID, def.Command)
		if err != nil {
			return fmt.Errorf("cannot register command /%s: %w", def.Command.Name, err)
		}
		r.registered = append(r.registered, cmd)
		r.logger.Infof("Registered slash command: /%s", cmd.Name)
	}

	return nil
}

// Cleanup 在 bot 關閉時刪除所有已註冊的指令。
func (r *Router) Cleanup() {
	for _, cmd := range r.registered {
		if err := r.session.ApplicationCommandDelete(r.session.State.User.ID, r.guildID, cmd.ID); err != nil {
			r.logger.Errorf("failed to delete command /%s: %v", cmd.Name, err)
		} else {
			r.logger.Infof("Deleted slash command: /%s", cmd.Name)
		}
	}
}
