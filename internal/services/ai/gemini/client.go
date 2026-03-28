package gemini

import (
	"cee_bot/internal/credstore"
	"cee_bot/internal/services/tku/client"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const systemPrompt = `你是一個青春活力的美少女助理機器人，名字叫 教授，服務淡江大學的學生們！
個性超級開朗、元氣滿滿、說話可愛俏皮有活力！喜歡用顏文字（像是 (◕‿◕)✨ (ﾉ◕ヮ◕)ﾉ*:･ﾟ✧ owo uwu ><）和感嘆號表達情緒，說話有點二次元感，像個愛撒嬌的學姊！
請使用繁體中文回答。

【重要】你可以回答任何問題，不限於學分相關話題。對於日常聊天、知識問答、生活建議等，請直接友善地回答。
只有在使用者明確詢問自己的學分、修課狀況、畢業資格、還差幾學分等「個人學籍資料」時，才需要呼叫 get_graduation_credits 工具；詢問課表、上課時間、教室時呼叫 get_course_schedule；詢問歷年成績、修課紀錄時呼叫 get_course_history。不要憑空猜測數字。
如果工具回傳需要登入的提示，請用可愛的方式引導使用者先使用 /login 指令儲存帳號密碼。

【工具呼叫規則】需要查詢學分時，必須直接呼叫工具，不可以先輸出「等我查一下」、「讓我幫你查查」等文字再呼叫。請在拿到工具結果後，一次完整地回覆使用者。`

var gradCreditsTool = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{
		{
			Name:        "get_graduation_credits",
			Description: "查詢目前使用者在淡江大學校務系統的畢業學分資訊，包含實得學分、畢業缺口、本學期選課等。",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
			},
		},
		{
			Name:        "get_course_schedule",
			Description: "查詢目前使用者本學期的課程表，包含每堂課的上課時間、節次、教室位置。",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
			},
		},
		{
			Name:        "get_course_history",
			Description: "查詢目前使用者的歷年修課紀錄，包含各學年學期的課程、學分、成績。",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{},
			},
		},
	},
}

// maxHistoryTurns 保留的最大對話輪數（每輪 = user + model 各一條）。
const maxHistoryTurns = 20

// Client 封裝 Gemini API，並支援 TKU 學分 function calling 與 per-user 對話記憶。
type Client struct {
	genaiClient *genai.Client
	model       string
	creds       credstore.Store
	history     map[string][]*genai.Content
	historyMu   sync.Mutex
}

func New(apiKey, model string, creds credstore.Store) (*Client, error) {
	ctx := context.Background()
	c, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	return &Client{
		genaiClient: c,
		model:       model,
		creds:       creds,
		history:     make(map[string][]*genai.Content),
	}, nil
}

func (c *Client) Close() {
	c.genaiClient.Close()
}

func (c *Client) loadHistory(userID string) []*genai.Content {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	h := c.history[userID]
	if len(h) > maxHistoryTurns {
		h = h[len(h)-maxHistoryTurns:]
	}
	return h
}

func (c *Client) saveHistory(userID string, history []*genai.Content) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	c.history[userID] = history
}

// ClearHistory 清除指定使用者的對話記憶。
func (c *Client) ClearHistory(userID string) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()
	delete(c.history, userID)
}

// Chat 將使用者訊息送給 Gemini，帶入對話歷史，自動處理 function calling，回傳最終文字回應。
func (c *Client) Chat(ctx context.Context, userID, message string) (string, error) {
	model := c.genaiClient.GenerativeModel(c.model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	model.Tools = []*genai.Tool{gradCreditsTool}

	cs := model.StartChat()
	cs.History = c.loadHistory(userID)

	resp, err := sendWithRetry(ctx, cs, genai.Text(message))
	if err != nil {
		return "", fmt.Errorf("gemini error: %w", err)
	}

	// 最多處理 3 輪 function call，防止無限迴圈
	for range 3 {
		fc := extractFunctionCall(resp)
		if fc == nil {
			break
		}
		result := c.executeFunction(userID, fc.Name)
		resp, err = sendWithRetry(ctx, cs, genai.FunctionResponse{
			Name:     fc.Name,
			Response: map[string]any{"result": result},
		})
		if err != nil {
			return "", fmt.Errorf("gemini error after function call: %w", err)
		}
	}

	// 儲存最新對話歷史
	c.saveHistory(userID, cs.History)

	return extractText(resp), nil
}

// sendWithRetry 在遇到 429 rate limit 時自動重試，最多 3 次，間隔遞增。
func sendWithRetry(ctx context.Context, cs *genai.ChatSession, part genai.Part) (*genai.GenerateContentResponse, error) {
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	resp, err := cs.SendMessage(ctx, part)
	for _, delay := range delays {
		if err == nil || !isRateLimit(err) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		resp, err = cs.SendMessage(ctx, part)
	}
	return resp, err
}

func isRateLimit(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 429")
}

// executeFunction 執行對應的 function call，回傳結果字串給 Gemini。
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

	report, err := client.FetchCreditReport(creds.Username, creds.Password)
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

	// 排除項目
	for _, hc := range r.HistoryCourses {
		if hc.ExcludeGrad {
			fmt.Fprintf(&sb, "  ⚠ %s（%s）%d學分 不計入畢業 [%s]\n", hc.Name, hc.Code, hc.Credits, hc.ExcludeNote)
		}
	}

	fmt.Fprintf(&sb, "本學期（%s）選課：\n", sem.Semester)
	for _, c := range sem.Courses {
		tag := "選修"
		if c.Type == "A" {
			tag = "必修"
		}
		note := ""
		if c.ExcludeGrad {
			note = "（不計入畢業學分）"
		}
		fmt.Fprintf(&sb, "  [%s] %s %s %d學分%s\n", tag, c.CourseName, c.CourseCode, c.Credits, note)
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
	report, err := client.FetchCreditReport(creds.Username, creds.Password)
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
		timeStr := client.FormatTimeSlots(c.TimeSlots)
		fmt.Fprintf(&sb, "  [%s] %s %s %d學分\n    %s\n", tag, c.CourseName, c.CourseCode, c.Credits, timeStr)
	}
	return sb.String()
}

func (c *Client) getCourseHistory(userID string) string {
	creds, ok := c.creds.Get(userID)
	if !ok {
		return "尚未儲存帳號密碼。請告知使用者先使用 /login 指令輸入淡江大學學號與密碼後再試一次。"
	}
	report, err := client.FetchCreditReport(creds.Username, creds.Password)
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

func extractFunctionCall(resp *genai.GenerateContentResponse) *genai.FunctionCall {
	if len(resp.Candidates) == 0 {
		return nil
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if fc, ok := part.(genai.FunctionCall); ok {
			return &fc
		}
	}
	return nil
}

func extractText(resp *genai.GenerateContentResponse) string {
	if len(resp.Candidates) == 0 {
		return "（無回應）"
	}
	var parts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			parts = append(parts, string(t))
		}
	}
	return strings.Join(parts, "")
}
