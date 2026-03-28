package ai

import (
	"cee_bot/config"
	routercmds "cee_bot/internal/commands"
	"cee_bot/internal/credstore"
	aicmds "cee_bot/internal/services/ai/commands"
	"cee_bot/internal/services/ai/gemini"
	"cee_bot/internal/services/ai/groq"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type Service struct {
	client aicmds.ChatClient
	logger *logrus.Logger
}

func New(session *discordgo.Session, cfg *config.Config, logger *logrus.Logger) (*Service, error) {
	var c aicmds.ChatClient

	switch cfg.AI.Provider {
	case "groq":
		if cfg.Groq.APIKey == "" {
			return nil, fmt.Errorf("groq api_key is not set in config")
		}
		c = groq.New(cfg.Groq.APIKey, cfg.Groq.Model, credstore.Default)
		logger.Infof("AI provider: Groq (%s)", cfg.Groq.Model)

	default: // gemini
		if cfg.Gemini.APIKey == "" {
			return nil, fmt.Errorf("gemini api_key is not set in config")
		}
		gc, err := gemini.New(cfg.Gemini.APIKey, cfg.Gemini.Model, credstore.Default)
		if err != nil {
			return nil, err
		}
		c = gc
		logger.Infof("AI provider: Gemini (%s)", cfg.Gemini.Model)
	}

	aicmds.SetClient(c)
	session.AddHandler(makeMessageHandler(c, logger))

	return &Service{client: c, logger: logger}, nil
}

func (s *Service) Commands() []routercmds.CommandDefinition {
	return aicmds.Definitions()
}

func (s *Service) Close() {
	s.client.Close()
}

func makeMessageHandler(c aicmds.ChatClient, logger *logrus.Logger) func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.Bot {
			return
		}

		triggered := false
		var replyContext string

		for _, user := range m.Mentions {
			if user.ID == s.State.User.ID {
				triggered = true
				break
			}
		}

		// 獨立檢查 reply（即使 mention 已觸發也要設 replyContext）
		if m.MessageReference != nil {
			ref, err := s.ChannelMessage(m.ChannelID, m.MessageReference.MessageID)
			if err == nil && ref.Author.ID == s.State.User.ID {
				triggered = true
				if ref.Content != "" {
					replyContext = ref.Content
				}
			}
		}
		if !triggered {
			return
		}

		content := strings.TrimSpace(strings.NewReplacer(
			"<@"+s.State.User.ID+">", "",
			"<@!"+s.State.User.ID+">", "",
		).Replace(m.Content))
		if content == "" {
			content = "你好！"
		}
		if replyContext != "" {
			if len([]rune(replyContext)) > 300 {
				runes := []rune(replyContext)
				replyContext = string(runes[:300]) + "..."
			}
			content = "[使用者正在回覆你說的這段話：\n" + replyContext + "]\n\n" + content
		}

		logger.WithFields(logrus.Fields{
			"user":          m.Author.Username,
			"reply_context": replyContext,
			"content":       content,
		}).Debug("chat input")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					s.ChannelTyping(m.ChannelID)
					select {
					case <-ctx.Done():
						return
					case <-time.After(8 * time.Second):
					}
				}
			}
		}()

		reply, err := c.Chat(ctx, m.Author.ID, content)
		if err != nil {
			logger.Errorf("ai chat error: %v", err)
			s.ChannelMessageSendReply(m.ChannelID, "❌ 出錯了，請稍後再試！", m.Reference())
			return
		}

		if len(reply) > 2000 {
			reply = reply[:1997] + "..."
		}
		s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference())
	}
}
