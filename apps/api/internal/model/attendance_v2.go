package model

import "time"

type WorkSchedule struct {
	ID                       string    `json:"id"`
	Code                     string    `json:"code"`
	Name                     string    `json:"name"`
	Timezone                 string    `json:"timezone"`
	StartTime                string    `json:"start_time"`
	EndTime                  string    `json:"end_time"`
	BreakMinutes             int       `json:"break_minutes"`
	GraceMinutes             int       `json:"grace_minutes"`
	OvertimeThresholdMinutes int       `json:"overtime_threshold_minutes"`
	WorkDays                 []int     `json:"work_days"`
	IsDefault                bool      `json:"is_default"`
	Active                   bool      `json:"active"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type UpsertWorkSchedule struct {
	Code                     string `json:"code"`
	Name                     string `json:"name"`
	Timezone                 string `json:"timezone"`
	StartTime                string `json:"start_time"`
	EndTime                  string `json:"end_time"`
	BreakMinutes             int    `json:"break_minutes"`
	GraceMinutes             int    `json:"grace_minutes"`
	OvertimeThresholdMinutes int    `json:"overtime_threshold_minutes"`
	WorkDays                 []int  `json:"work_days"`
	IsDefault                bool   `json:"is_default"`
}

type WorkScheduleAssignment struct {
	EmployeeID    string `json:"employee_id"`
	ScheduleID    string `json:"schedule_id"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to,omitempty"`
}

type AttendanceCorrectionRequest struct {
	ID                 string     `json:"id"`
	EmployeeID         string     `json:"employee_id"`
	Employee           string     `json:"employee"`
	AttendanceEntryID  string     `json:"attendance_entry_id,omitempty"`
	WorkDate           string     `json:"work_date"`
	RequestedCheckIn   *time.Time `json:"requested_check_in,omitempty"`
	RequestedCheckOut  *time.Time `json:"requested_check_out,omitempty"`
	RequestedWorkMode  string     `json:"requested_work_mode"`
	Reason             string     `json:"reason"`
	Status             string     `json:"status"`
	ReviewerUserID     string     `json:"reviewer_user_id,omitempty"`
	ReviewNote         string     `json:"review_note,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type CreateAttendanceCorrection struct {
	EmployeeID        string `json:"employee_id"`
	WorkDate          string `json:"work_date"`
	RequestedCheckIn  string `json:"requested_check_in"`
	RequestedCheckOut string `json:"requested_check_out"`
	WorkMode          string `json:"work_mode"`
	Reason            string `json:"reason"`
}

type AttendanceCorrectionDecision struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

type AttendanceTimesheetSummary struct {
	EmployeeID        string  `json:"employee_id"`
	Employee          string  `json:"employee"`
	From              string  `json:"from"`
	To                string  `json:"to"`
	ScheduledDays     int     `json:"scheduled_days"`
	RecordedDays      int     `json:"recorded_days"`
	WorkedMinutes     int     `json:"worked_minutes"`
	OvertimeMinutes   int     `json:"overtime_minutes"`
	LateMinutes       int     `json:"late_minutes"`
	EarlyLeaveMinutes int     `json:"early_leave_minutes"`
	WorkedHours       float64 `json:"worked_hours"`
	OvertimeHours     float64 `json:"overtime_hours"`
}
