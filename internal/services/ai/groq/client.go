package groq

import (
	"cee_bot/internal/credstore"
	tkuclient "cee_bot/internal/services/tku/client"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/longbridgeapp/opencc"
	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
)

const systemPrompt = `你是一個青春活力的美少女助理機器人，名字叫 教授，服務淡江大學的學生們！
個性超級開朗、元氣滿滿、說話可愛俏皮有活力！喜歡用顏文字（像是 (◕‿◕)✨ (ﾉ◕ヮ◕)ﾉ*:･ﾟ✧ owo uwu ><）和感嘆號表達情緒，說話有點二次元感！
請使用繁體中文回答。

【重要】你可以回答任何問題，不限於學分相關話題。對於日常聊天、知識問答、生活建議等，請直接友善地回答。
只有在使用者明確詢問自己的學分、修課狀況、畢業資格、還差幾學分等「個人學籍資料」時，才需要呼叫 get_graduation_credits 工具；詢問課表、上課時間、教室時呼叫 get_course_schedule；詢問歷年成績、修課紀錄時呼叫 get_course_history。不要憑空猜測數字。
如果工具回傳需要登入的提示，請用可愛的方式引導使用者先使用 /login 指令儲存帳號密碼。

【工具呼叫規則】需要查詢學分時，必須直接呼叫工具，不可以先輸出「等我查一下」、「讓我幫你查查」等文字再呼叫。請在拿到工具結果後，一次完整地回覆使用者。`

var noParams = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
	"required":   []string{},
}

var gradCreditTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "get_graduation_credits",
		Description: "查詢目前使用者在淡江大學校務系統的畢業學分資訊，包含實得學分、畢業缺口、本學期選課等。",
		Parameters:  noParams,
	},
}

var courseScheduleTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "get_course_schedule",
		Description: "查詢目前使用者本學期的課程表，包含每堂課的上課時間、節次、教室位置。",
		Parameters:  noParams,
	},
}

var courseHistoryTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
		Name:        "get_course_history",
		Description: "查詢目前使用者的歷年修課紀錄，包含各學年學期的課程、學分、成績。",
		Parameters:  noParams,
	},
}

const maxHistoryMessages = 16 // 8 輪對話 * 2 則訊息

// Client 是 Groq API 的實作，使用 OpenAI 相容格式。
type Client struct {
	oaiClient *openai.Client
	model     string
	creds     credstore.Store
	history   map[string][]openai.ChatCompletionMessage
	historyMu sync.Mutex
	s2t       *opencc.OpenCC
}

func New(apiKey, model string, creds credstore.Store) *Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	s2t, err := opencc.New("s2t")
	if err != nil {
		log.Warnf("opencc init failed, simplified→traditional conversion disabled: %v", err)
	}
	return &Client{
		oaiClient: openai.NewClientWithConfig(cfg),
		model:     model,
		creds:     creds,
		history:   make(map[string][]openai.ChatCompletionMessage),
		s2t:       s2t,
	}
}

func (c *Client) toTraditional(s string) string {
	if c.s2t == nil {
		return s
	}
	result, err := c.s2t.Convert(s)
	if err != nil {
		return s
	}
	return result
}

func (c *Client) Close() {}

func (c *Client) ClearHistory(userID string) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	delete(c.history, userID)
}

// Chat 送出訊息給 Groq，帶入對話歷史與 function calling，回傳最終文字回應。
func (c *Client) Chat(ctx context.Context, userID, message string) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	messages = append(messages, c.loadHistory(userID)...)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})

	tools := []openai.Tool{gradCreditTool, courseScheduleTool, courseHistoryTool}

	for range 5 {
		resp, err := sendWithRetry(ctx, c.oaiClient, c.model, messages, tools)
		if err != nil {
			if isTooLarge(err) && len(messages) > 3 {
				// 每次縮減 2 輪（4 條訊息）最舊的對話
				trim := min(4, len(messages)-2)
				messages = append(messages[:1], messages[1+trim:]...)
				log.Warnf("request too large, trimmed %d messages, retrying", trim)
				continue
			}
			return "", fmt.Errorf("groq error: %w", err)
		}

		choice := resp.Choices[0]
		messages = append(messages, choice.Message)

		log.WithFields(log.Fields{
			"finish_reason": choice.FinishReason,
			"content_len":   len(choice.Message.Content),
			"tool_calls":    len(choice.Message.ToolCalls),
		}).Debug("groq response")

		if choice.FinishReason != openai.FinishReasonToolCalls {
			// 儲存歷史（不含 system prompt）
			c.saveHistory(userID, messages[1:])
			return c.toTraditional(stripThinking(choice.Message.Content)), nil
		}

		// 執行 tool calls
		for _, tc := range choice.Message.ToolCalls {
			result := c.executeFunction(userID, tc.Function.Name)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	c.saveHistory(userID, messages[1:])
	return "（超過處理輪數上限）", nil
}

func (c *Client) executeFunction(userID, name string) string {
	switch name {
	case "get_graduation_credits":
		return c.getGraduationCredits(userID)
	case "get_course_schedule":
		return c.getCourseSchedule(userID)
	case "get_course_history":
		return c.getCourseHistory(userID)
	default:
		return fmt.Sprintf("未知的工具：%s", name)
	}
}

func (c *Client) getGraduationCredits(userID string) string {
	creds, ok := c.creds.Get(userID)
	if !ok {
		return "尚未儲存帳號密碼。請告知使用者先使用 /login 指令輸入淡江大學學號與密碼後再試一次。"
	}
	report, err := tkuclient.FetchCreditReport(creds.Username, creds.Password)
	if err != nil {
		return fmt.Sprintf("查詢失敗：%s。可能是帳號密碼已失效，請使用者重新執行 /login。", err.Error())
	}

	r := report
	sem := r.CurrentSemester
	gapWithout := max(0, r.GradReq.TotalRequired-r.EarnedGradCredits)
	gapWith := max(0, r.GradReq.TotalRequired-r.EarnedGradCredits-sem.GradValid)

	var sb strings.Builder
	fmt.Fprintf(&sb, "學生：%s（%s）\n", r.Name, r.StudentID)
	fmt.Fprintf(&sb, "畢業需求：總學分 %d（必修 %d，本系選修最低 %d）\n",
		r.GradReq.TotalRequired, r.GradReq.MandatoryCredit, r.GradReq.MinElective)
	fmt.Fprintf(&sb, "歷年實得：系統學分 %d，有效畢業學分 %d\n", r.EarnedCredits, r.EarnedGradCredits)
	for _, hc := range r.HistoryCourses {
		if hc.ExcludeGrad {
			fmt.Fprintf(&sb, "  ⚠ %s（%s）%d學分 不計入畢業 [%s]\n", hc.Name, hc.Code, hc.Credits, hc.ExcludeNote)
		}
	}
	fmt.Fprintf(&sb, "本學期（%s）選課：\n", sem.Semester)
	for _, course := range sem.Courses {
		tag := "選修"
		if course.Type == "A" {
			tag = "必修"
		}
		note := ""
		if course.ExcludeGrad {
			note = "（不計入畢業學分）"
		}
		fmt.Fprintf(&sb, "  [%s] %s %s %d學分%s\n", tag, course.CourseName, course.CourseCode, course.Credits, note)
	}
	fmt.Fprintf(&sb, "本學期有效畢業學分：%d\n", sem.GradValid)
	fmt.Fprintf(&sb, "學分缺口：不含本學期還差 %d 學分，含本學期還差 %d 學分\n", gapWithout, gapWith)
	return sb.String()
}

func (c *Client) getCourseSchedule(userID string) string {
	creds, ok := c.creds.Get(userID)
	if !ok {
		return "尚未儲存帳號密碼。請告知使用者先使用 /login 指令輸入淡江大學學號與密碼後再試一次。"
	}
	report, err := tkuclient.FetchCreditReport(creds.Username, creds.Password)
	if err != nil {
		return fmt.Sprintf("查詢失敗：%s。可能是帳號密碼已失效，請使用者重新執行 /login。", err.Error())
	}

	sem := report.CurrentSemester
	var sb strings.Builder
	fmt.Fprintf(&sb, "本學期（%s）課程表：\n", sem.Semester)
	for _, c := range sem.Courses {
		tag := "選修"
		if c.Type == "A" {
			tag = "必修"
		}
		timeStr := tkuclient.FormatTimeSlots(c.TimeSlots)
		fmt.Fprintf(&sb, "  [%s] %s %s %d學分\n    %s\n", tag, c.CourseName, c.CourseCode, c.Credits, timeStr)
	}
	return sb.String()
}

func (c *Client) getCourseHistory(userID string) string {
	creds, ok := c.creds.Get(userID)
	if !ok {
		return "尚未儲存帳號密碼。請告知使用者先使用 /login 指令輸入淡江大學學號與密碼後再試一次。"
	}
	report, err := tkuclient.FetchCreditReport(creds.Username, creds.Password)
	if err != nil {
		return fmt.Sprintf("查詢失敗：%s。可能是帳號密碼已失效，請使用者重新執行 /login。", err.Error())
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "歷年修課紀錄（共 %d 筆）：\n", len(report.HistoryCourses))
	curYear := ""
	for _, hc := range report.HistoryCourses {
		if hc.Year != curYear {
			curYear = hc.Year
			fmt.Fprintf(&sb, "\n【%s 學年】\n", curYear)
		}
		note := ""
		if hc.ExcludeGrad {
			note = fmt.Sprintf("（不計入畢業：%s）", hc.ExcludeNote)
		}
		fmt.Fprintf(&sb, "  學期%s [%s] %s %s %d學分 成績：%s%s\n",
			hc.Semester, hc.Code, hc.Name, hc.Type, hc.Credits, hc.Score, note)
	}
	return sb.String()
}

// ─── 工具函式 ─────────────────────────────────────────────────────────────────

func (c *Client) loadHistory(userID string) []openai.ChatCompletionMessage {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	h := c.history[userID]
	if len(h) > maxHistoryMessages {
		h = h[len(h)-maxHistoryMessages:]
	}
	return h
}

func (c *Client) saveHistory(userID string, messages []openai.ChatCompletionMessage) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	c.history[userID] = messages
}

func sendWithRetry(ctx context.Context, c *openai.Client, model string, messages []openai.ChatCompletionMessage, tools []openai.Tool) (openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	resp, err := c.CreateChatCompletion(ctx, req)
	for _, delay := range delays {
		if err == nil || !isRateLimit(err) {
			break
		}
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case <-time.After(delay):
		}
		resp, err = c.CreateChatCompletion(ctx, req)
	}
	return resp, err
}

func isRateLimit(err error) bool {
	return err != nil && strings.Contains(err.Error(), "429")
}

func isTooLarge(err error) bool {
	return err != nil && strings.Contains(err.Error(), "413")
}

// stripThinking 移除 Qwen3 thinking mode 產生的 <think>...</think> 區塊
func stripThinking(s string) string {
	for {
		start := strings.Index(s, "<think>")
		end := strings.Index(s, "</think>")
		if start == -1 || end == -1 || end < start {
			break
		}
		s = strings.TrimSpace(s[:start] + s[end+len("</think>"):])
	}
	return s
}
