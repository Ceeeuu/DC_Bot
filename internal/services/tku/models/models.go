package models

// GradRequirements 畢業學分需求
type GradRequirements struct {
	TotalRequired   int
	MandatoryCredit int
	MinElective     int
}

// TimeSlot 單一上課時段
type TimeSlot struct {
	Day     string // 一二三四五六日
	Periods []int  // 節次列表，如 [7, 8]
	Room    string // 教室，如 "B 204"
}

// SemesterCourse 單一選課資料
type SemesterCourse struct {
	CourseCode  string
	CourseName  string
	Type        string // A=必修 C=選修/其他
	Credits     int
	ExcludeGrad bool
	TimeSlots   []TimeSlot // 上課時段（可能有多個）
}

// SemesterSummary 當學期選課摘要
type SemesterSummary struct {
	Semester  string
	Courses   []SemesterCourse
	Total     int
	Required  int
	Elective  int
	GradValid int
}

// HistoryCourse 歷年成績中的單一課程
type HistoryCourse struct {
	Year        string // 學年
	Semester    string // 學期
	Code        string
	Name        string
	Type        string // 必修/選修
	Credits     int
	Score       string // 成績（數字或「通過」）
	ExcludeGrad bool
	ExcludeNote string
}

// CreditReport 完整學分報告
type CreditReport struct {
	Name              string
	StudentID         string
	GradReq           GradRequirements
	EarnedCredits     int
	EarnedGradCredits int
	HistoryCourses    []HistoryCourse
	CurrentSemester   SemesterSummary
}
