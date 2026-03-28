package client

import (
	"cee_bot/internal/services/tku/models"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	baseURL          = "https://sso.tku.edu.tw"
	loginURL         = baseURL + "/NEAI/login2.do?action=EAI"
	imageValidateURL = baseURL + "/NEAI/ImageValidate"
	loginTargetURL   = baseURL + "/aissinfo/emis/tmw0012.aspx"
	menuURL          = baseURL + "/aissinfo/emis/TMW0040.aspx"
	c120URL          = baseURL + "/aissinfo/emis/TMWC120.aspx"
	c020URL          = baseURL + "/aissinfo/emis/TMWC020.aspx"
	s100URL          = baseURL + "/aissinfo/emis/TMWS100.aspx"
)

// FetchCreditReport 以帳號密碼登入並取得完整學分報告。
func FetchCreditReport(username, password string) (*models.CreditReport, error) {
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{Jar: jar}

	if err := login(httpClient, username, password); err != nil {
		return nil, err
	}
	return fetchCreditReport(httpClient)
}

// ─── 登入 ─────────────────────────────────────────────────────────────────────

func login(client *http.Client, username, password string) error {
	httpGet(client, loginURL, "")
	httpGet(client, imageValidateURL, loginURL)

	req, _ := http.NewRequest("POST", imageValidateURL, strings.NewReader("outType=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", loginURL)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	vidcode := strings.TrimSpace(string(b))

	form := url.Values{
		"myurl":     {loginTargetURL},
		"ln":        {"zh_TW"},
		"embed":     {"No"},
		"vkb":       {"No"},
		"logintype": {"logineb"},
		"username":  {username},
		"password":  {password},
		"vidcode":   {vidcode},
		"loginbtn":  {"登入"},
	}
	loginBody := httpPost(client, loginURL, form.Encode(), loginURL)
	m := regexp.MustCompile(`location\.href="([^"]+)"`).FindStringSubmatch(loginBody)
	if len(m) < 2 {
		return fmt.Errorf("帳號或密碼錯誤，無法登入")
	}
	httpGet(client, m[1], loginURL)
	return nil
}

// ─── 學分報告 ─────────────────────────────────────────────────────────────────

func fetchCreditReport(client *http.Client) (*models.CreditReport, error) {
	report := &models.CreditReport{}

	c120 := httpGet(client, c120URL, menuURL)
	parseGradRequirements(c120, report)

	s100 := httpGet(client, s100URL, menuURL)
	parseEarnedCredits(s100, report)

	c020page := httpGet(client, c020URL, menuURL)
	semester, err := fetchCurrentSemester(client, c020page)
	if err != nil {
		return nil, err
	}
	report.CurrentSemester = semester

	return report, nil
}

// ─── 解析函式 ─────────────────────────────────────────────────────────────────

func parseGradRequirements(html string, report *models.CreditReport) {
	if m := regexp.MustCompile(`學號：<font[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		report.StudentID = m[1]
	}
	if m := regexp.MustCompile(`姓名：<font[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		report.Name = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`畢業學分數：(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		report.GradReq.TotalRequired, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`必修學分數：(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		report.GradReq.MandatoryCredit, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`最低學分[：:]\s*(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		report.GradReq.MinElective, _ = strconv.Atoi(m[1])
	}
}

func parseEarnedCredits(html string, report *models.CreditReport) {
	if m := regexp.MustCompile(`累計實得學分[：:]\s*(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		report.EarnedCredits, _ = strconv.Atoi(m[1])
	}

	tableRe := regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	rowRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	seen := map[string]bool{}

	for _, t := range tableRe.FindAllStringSubmatch(html, -1) {
		if !strings.Contains(t[1], "科號") {
			continue
		}
		for i, row := range rowRe.FindAllStringSubmatch(t[1], -1) {
			if i < 4 {
				continue
			}
			cells := parseTdCells(row[1])
			if len(cells) < 11 {
				continue
			}
			code := cells[4]
			semSeq := cells[6]
			name := extractChineseName(cells[5])
			credits, _ := strconv.Atoi(cells[9])
			if credits == 0 {
				continue
			}

			excl, note := isExcludedFromGradHistory(code, name)
			key := code + "_" + semSeq
			if !excl && seen[key] {
				excl = true
				note = "重複修習"
			}
			if !excl {
				seen[key] = true
			}

			courseType := cells[8]
			score := cells[10]
			if strings.Contains(courseType, "必修") {
				courseType = "必修"
			} else {
				courseType = "選修"
			}

			report.HistoryCourses = append(report.HistoryCourses, models.HistoryCourse{
				Year:        cells[0],
				Semester:    cells[1],
				Code:        code,
				Name:        name,
				Type:        courseType,
				Credits:     credits,
				Score:       score,
				ExcludeGrad: excl,
				ExcludeNote: note,
			})
			if !excl {
				report.EarnedGradCredits += credits
			}
		}
		break
	}
}

func isExcludedFromGradHistory(code, name string) (bool, string) {
	if strings.HasPrefix(code, "X") {
		return true, "外語能力檢定替代"
	}
	if regexp.MustCompile(`^T9[89]\d+`).MatchString(code) {
		return true, "體育課程"
	}
	if strings.Contains(name, "體育") {
		return true, "體育課程"
	}
	if regexp.MustCompile(`^T97\d+`).MatchString(code) {
		return true, "全民國防/軍事訓練"
	}
	if strings.Contains(name, "全民國防") || strings.Contains(name, "軍事訓練") {
		return true, "全民國防/軍事訓練"
	}
	return false, ""
}

func fetchCurrentSemester(client *http.Client, page string) (models.SemesterSummary, error) {
	vs := extractHidden(page, "__VIEWSTATE")
	evgen := extractHidden(page, "__VIEWSTATEGENERATOR")
	evval := extractHidden(page, "__EVENTVALIDATION")

	semRe := regexp.MustCompile(`<option[^>]+value="(\d{4})"[^>]*>([^<]+)`)
	semMatches := semRe.FindAllStringSubmatch(page, -1)
	if len(semMatches) == 0 {
		return models.SemesterSummary{}, fmt.Errorf("找不到學期資料")
	}
	semCode := semMatches[0][1]
	semName := strings.TrimSpace(semMatches[0][2])

	form := url.Values{
		"__EVENTTARGET":        {""},
		"__EVENTARGUMENT":      {""},
		"__LASTFOCUS":          {""},
		"__VIEWSTATE":          {vs},
		"__VIEWSTATEGENERATOR": {evgen},
		"__EVENTVALIDATION":    {evval},
		"DropDownList1":        {semCode},
		"Button1":              {"開始查詢"},
	}
	req, _ := http.NewRequest("POST", c020URL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", c020URL)
	resp, err := client.Do(req)
	if err != nil {
		return models.SemesterSummary{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	return parseCoursePage(string(body), semName), nil
}

func parseCoursePage(html, semName string) models.SemesterSummary {
	summary := models.SemesterSummary{Semester: semName}

	tableRe := regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	for _, t := range tableRe.FindAllStringSubmatch(html, -1) {
		if !strings.Contains(t[1], "開課") || !strings.Contains(t[1], "科目") {
			continue
		}
		rowRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
		for i, row := range rowRe.FindAllStringSubmatch(t[1], -1) {
			if i == 0 {
				continue
			}
			cells := parseTdCells(row[1])
			if len(cells) < 12 {
				continue
			}

			isMainRow := regexp.MustCompile(`^\d+$`).MatchString(cells[0])
			isContinuationRow := cells[0] == "" && strings.Contains(cells[11], "/")

			if isMainRow {
				nameCode := strings.TrimSpace(cells[3])
				codeRe := regexp.MustCompile(`([A-Z]\d{4,})$`)
				courseCode := ""
				courseName := nameCode
				if cm := codeRe.FindStringSubmatch(nameCode); len(cm) > 1 {
					courseCode = cm[1]
					courseName = strings.TrimSpace(strings.TrimSuffix(nameCode, courseCode))
				}

				courseType := strings.TrimSpace(cells[7])
				credits, _ := strconv.Atoi(strings.TrimSpace(cells[8]))
				if credits == 0 {
					continue
				}

				course := models.SemesterCourse{
					CourseCode:  courseCode,
					CourseName:  courseName,
					Type:        courseType,
					Credits:     credits,
					ExcludeGrad: isExcludedFromGrad(courseCode, courseName),
				}
				if ts := parseTimeSlot(cells[11]); ts != nil {
					course.TimeSlots = append(course.TimeSlots, *ts)
				}
				summary.Courses = append(summary.Courses, course)

			} else if isContinuationRow && len(summary.Courses) > 0 {
				if ts := parseTimeSlot(cells[11]); ts != nil {
					last := &summary.Courses[len(summary.Courses)-1]
					last.TimeSlots = append(last.TimeSlots, *ts)
				}
			}
		}
		break
	}

	sumTableRe := regexp.MustCompile(`(?s)科目數.*?總修學分.*?<tr[^>]*>(.*?)</tr>`)
	if sm := sumTableRe.FindStringSubmatch(html); len(sm) > 1 {
		nums := regexp.MustCompile(`\d+`).FindAllString(sm[1], -1)
		if len(nums) >= 4 {
			summary.Required, _ = strconv.Atoi(nums[1])
			summary.Elective, _ = strconv.Atoi(nums[2])
			summary.Total, _ = strconv.Atoi(nums[3])
		}
	}

	excluded := 0
	for _, c := range summary.Courses {
		if c.ExcludeGrad {
			excluded += c.Credits
		}
	}
	summary.GradValid = summary.Total - excluded
	return summary
}

func isExcludedFromGrad(code, name string) bool {
	if strings.HasPrefix(code, "X") {
		return true
	}
	if regexp.MustCompile(`^T9[89]\d+`).MatchString(code) {
		return true
	}
	if strings.Contains(name, "全民國防") || strings.Contains(name, "軍事訓練") {
		return true
	}
	return false
}

// extractChineseName 從「中文名稱ENGLISH NAME」格式中提取中文部分
func extractChineseName(raw string) string {
	raw = strings.TrimSpace(raw)
	runes := []rune(raw)
	for i := 1; i < len(runes); i++ {
		cur := runes[i]
		prev := runes[i-1]
		isCurUpper := cur >= 'A' && cur <= 'Z'
		isPrevCJK := (prev >= 0x4E00 && prev <= 0x9FFF) ||
			(prev >= 0xFF00 && prev <= 0xFFEF)
		if isCurUpper && isPrevCJK {
			return strings.TrimSpace(string(runes[:i]))
		}
	}
	return raw
}

// parseTimeSlot 解析「三 / 11,12 / B 204」格式的時間欄位
func parseTimeSlot(raw string) *models.TimeSlot {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.SplitN(raw, "/", 3)
	if len(parts) < 2 {
		return nil
	}
	day := strings.TrimSpace(parts[0])
	if day == "" {
		return nil
	}
	periodStrs := strings.Split(strings.TrimSpace(parts[1]), ",")
	var periods []int
	for _, p := range periodStrs {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil {
			periods = append(periods, n)
		}
	}
	room := ""
	if len(parts) == 3 {
		room = strings.TrimSpace(parts[2])
	}
	return &models.TimeSlot{Day: day, Periods: periods, Room: room}
}

// FormatTimeSlots 將時段列表格式化為易讀字串（供 handlers 使用）
func FormatTimeSlots(slots []models.TimeSlot) string {
	if len(slots) == 0 {
		return "（無時間資料）"
	}
	startTimes := map[int]string{
		1: "08:10", 2: "09:10", 3: "10:10", 4: "11:10",
		5: "12:10", 6: "13:10", 7: "14:10", 8: "15:10",
		9: "16:10", 10: "17:10", 11: "18:10", 12: "19:10",
		13: "20:10", 14: "21:10",
	}
	endTimes := map[int]string{
		1: "09:00", 2: "10:00", 3: "11:00", 4: "12:00",
		5: "13:00", 6: "14:00", 7: "15:00", 8: "16:00",
		9: "17:00", 10: "18:00", 11: "19:00", 12: "20:00",
		13: "21:00", 14: "22:00",
	}
	var parts []string
	for _, s := range slots {
		if len(s.Periods) == 0 {
			continue
		}
		first := s.Periods[0]
		last := s.Periods[len(s.Periods)-1]
		timeStr := startTimes[first] + "～" + endTimes[last]
		periodNums := make([]string, len(s.Periods))
		for i, p := range s.Periods {
			periodNums[i] = strconv.Itoa(p)
		}
		room := s.Room
		if room == "" {
			room = "（未指定教室）"
		}
		parts = append(parts, fmt.Sprintf("週%s 第%s節 %s  教室：%s",
			s.Day, strings.Join(periodNums, ","), timeStr, room))
	}
	return strings.Join(parts, "\n")
}

// ─── 工具函式 ─────────────────────────────────────────────────────────────────

func httpGet(client *http.Client, u, referer string) string {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func httpPost(client *http.Client, u, body, referer string) string {
	req, _ := http.NewRequest("POST", u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func extractHidden(html, name string) string {
	re := regexp.MustCompile(`<input[^>]+name="` + name + `"[^>]+value="([^"]*)"`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseTdCells(trHTML string) []string {
	tdRe := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	var cells []string
	for _, m := range tdRe.FindAllStringSubmatch(trHTML, -1) {
		content := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(m[1], "")
		content = strings.ReplaceAll(content, "&nbsp;", " ")
		content = strings.TrimSpace(content)
		cells = append(cells, content)
	}
	return cells
}
