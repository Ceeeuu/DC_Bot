package commands

import (
	"cee_bot/internal/credstore"
	"cee_bot/internal/services/tku/client"
	"cee_bot/internal/services/tku/models"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

// ─── /credits ────────────────────────────────────────────────────────────────

func HandleCredits(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: CreditsModalID,
			Title:    "查詢淡江大學畢業學分",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "username", Label: "學號",
						Style: discordgo.TextInputShort, Placeholder: "請輸入學號", Required: true,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "password", Label: "密碼",
						Style: discordgo.TextInputShort, Placeholder: "請輸入校務系統密碼", Required: true,
					},
				}},
			},
		},
	})
}

func HandleCreditsModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	username, password := extractModalCreds(i)

	report, err := client.FetchCreditReport(username, password)
	if err != nil {
		errMsg := fmt.Sprintf("❌ 無法取得資料：%s", err.Error())
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &errMsg})
		return
	}

	// 登入成功後儲存帳密，供 /chat 自動使用
	credstore.Default.Set(i.Member.User.ID, credstore.Credentials{
		Username: username,
		Password: password,
	})

	embed := buildEmbed(report)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// ─── /login ──────────────────────────────────────────────────────────────────

func HandleLogin(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: LoginModalID,
			Title:    "儲存淡江大學帳號密碼",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "username", Label: "學號",
						Style: discordgo.TextInputShort, Placeholder: "請輸入學號", Required: true,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						CustomID: "password", Label: "密碼",
						Style: discordgo.TextInputShort, Placeholder: "請輸入校務系統密碼", Required: true,
					},
				}},
			},
		},
	})
}

func HandleLoginModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	username, password := extractModalCreds(i)

	credstore.Default.Set(i.Member.User.ID, credstore.Credentials{
		Username: username,
		Password: password,
	})

	msg := "✅ 帳號密碼已儲存！現在可以使用 `/chat` 詢問學分相關問題了。"
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// ─── /schedule ───────────────────────────────────────────────────────────────

func HandleSchedule(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creds, ok := credstore.Default.Get(i.Member.User.ID)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ 尚未儲存帳號密碼，請先使用 `/login` 指令登入。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	report, err := client.FetchCreditReport(creds.Username, creds.Password)
	if err != nil {
		errMsg := fmt.Sprintf("❌ 無法取得資料：%s", err.Error())
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &errMsg})
		return
	}

	embed := buildScheduleEmbed(report)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// ─── /history ────────────────────────────────────────────────────────────────

func HandleHistory(s *discordgo.Session, i *discordgo.InteractionCreate) {
	creds, ok := credstore.Default.Get(i.Member.User.ID)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ 尚未儲存帳號密碼，請先使用 `/login` 指令登入。",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	report, err := client.FetchCreditReport(creds.Username, creds.Password)
	if err != nil {
		errMsg := fmt.Sprintf("❌ 無法取得資料：%s", err.Error())
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &errMsg})
		return
	}

	embed := buildHistoryEmbed(report)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
}

// ─── 共用工具 ─────────────────────────────────────────────────────────────────

// extractModalCreds 從 modal submit 取出學號與密碼。
func extractModalCreds(i *discordgo.InteractionCreate) (username, password string) {
	for _, row := range i.ModalSubmitData().Components {
		for _, comp := range row.(*discordgo.ActionsRow).Components {
			input := comp.(*discordgo.TextInput)
			switch input.CustomID {
			case "username":
				username = input.Value
			case "password":
				password = input.Value
			}
		}
	}
	return
}

func buildScheduleEmbed(r *models.CreditReport) *discordgo.MessageEmbed {
	sem := r.CurrentSemester
	var lines []string
	for _, c := range sem.Courses {
		typeTag := "選"
		if c.Type == "A" {
			typeTag = "必"
		}
		timeStr := client.FormatTimeSlots(c.TimeSlots)
		line := fmt.Sprintf("`[%s]` **%s** `%s` %d學分\n%s", typeTag, c.CourseName, c.CourseCode, c.Credits, timeStr)
		if c.ExcludeGrad {
			line += " ⚠不計畢業學分"
		}
		lines = append(lines, line)
	}
	value := strings.Join(lines, "\n\n")
	if len(value) > 4090 {
		value = value[:4090] + "..."
	}
	if value == "" {
		value = "（無課程資料）"
	}

	return &discordgo.MessageEmbed{
		Title: fmt.Sprintf("📅 %s（%s）%s 課程表", r.Name, r.StudentID, sem.Semester),
		Color: 0x003087,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "課程列表", Value: value, Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "資料來源：淡江大學校務行政資訊系統",
		},
	}
}

func buildHistoryEmbed(r *models.CreditReport) *discordgo.MessageEmbed {
	// 按學年分組
	grouped := map[string][]models.HistoryCourse{}
	var yearOrder []string
	for _, c := range r.HistoryCourses {
		if _, exists := grouped[c.Year]; !exists {
			yearOrder = append(yearOrder, c.Year)
		}
		grouped[c.Year] = append(grouped[c.Year], c)
	}

	var fields []*discordgo.MessageEmbedField
	for _, year := range yearOrder {
		courses := grouped[year]
		var lines []string
		for _, c := range courses {
			note := ""
			if c.ExcludeGrad {
				note = " ⚠"
			}
			lines = append(lines, fmt.Sprintf("學期%s `%s` %s %d學分 **%s**%s", c.Semester, c.Code, c.Name, c.Credits, c.Score, note))
		}
		value := strings.Join(lines, "\n")
		if len(value) > 1020 {
			value = value[:1020] + "..."
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s 學年", year),
			Value:  value,
			Inline: false,
		})
		if len(fields) >= 25 {
			break
		}
	}

	return &discordgo.MessageEmbed{
		Title: fmt.Sprintf("📋 %s（%s）歷年修課紀錄", r.Name, r.StudentID),
		Color: 0x003087,
		Fields: fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("共 %d 筆　有效畢業學分：%d 學分　⚠ 標記不計入畢業", len(r.HistoryCourses), r.EarnedGradCredits),
		},
	}
}

func buildEmbed(r *models.CreditReport) *discordgo.MessageEmbed {
	gapWithout := max(0, r.GradReq.TotalRequired-r.EarnedGradCredits)
	gapWith := max(0, r.GradReq.TotalRequired-r.EarnedGradCredits-r.CurrentSemester.GradValid)

	gradReqValue := fmt.Sprintf(
		"畢業總學分：**%d**　｜　必修：**%d**　｜　本系選修：**%d**",
		r.GradReq.TotalRequired, r.GradReq.MandatoryCredit, r.GradReq.MinElective,
	)

	excludedCredits := r.EarnedCredits - r.EarnedGradCredits
	earnedValue := fmt.Sprintf("系統實得：**%d** 學分　｜　有效畢業學分：**%d** 學分", r.EarnedCredits, r.EarnedGradCredits)
	if excludedCredits > 0 {
		var excludeNames []string
		for _, c := range r.HistoryCourses {
			if c.ExcludeGrad {
				excludeNames = append(excludeNames, fmt.Sprintf("%s（%s）", c.Name, c.ExcludeNote))
			}
		}
		earnedValue += fmt.Sprintf("\n⚠ 排除 **%d** 學分：%s", excludedCredits, strings.Join(excludeNames, "、"))
	}

	sem := r.CurrentSemester
	var courseLines []string
	for _, c := range sem.Courses {
		typeTag := lo.Ternary(c.Type == "A", "必", "選")
		line := fmt.Sprintf("`[%s]` %-20s `%s` %d學分", typeTag, c.CourseName, c.CourseCode, c.Credits)
		if c.ExcludeGrad {
			line += " ⚠"
		}
		courseLines = append(courseLines, line)
	}
	courseLines = append(courseLines, fmt.Sprintf("\n系統統計：必修 **%d** ＋ 選修 **%d** ＝ 總計 **%d** 學分", sem.Required, sem.Elective, sem.Total))
	courseLines = append(courseLines, fmt.Sprintf("有效畢業學分：**%d** 學分", sem.GradValid))
	semValue := strings.Join(courseLines, "\n")
	if len(semValue) > 1020 {
		semValue = semValue[:1020] + "..."
	}

	gapValue := fmt.Sprintf("不含本學期：還差 **%d** 學分\n含本學期中：還差 **%d** 學分", gapWithout, gapWith)
	if gapWith <= 0 {
		gapValue += "\n\n✅ 含本學期已達畢業學分門檻！"
	}

	return &discordgo.MessageEmbed{
		Title: fmt.Sprintf("📊 %s（%s）的畢業學分報告", r.Name, r.StudentID),
		Color: 0x003087,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "🎓 畢業要求", Value: gradReqValue, Inline: false},
			{Name: "📚 累計實得學分", Value: earnedValue, Inline: false},
			{Name: fmt.Sprintf("📅 %s", sem.Semester), Value: semValue, Inline: false},
			{Name: "🏁 學分缺口", Value: gapValue, Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "⚠ 標記課程不計入畢業學分　｜　資料來源：淡江大學校務行政資訊系統",
		},
	}
}
