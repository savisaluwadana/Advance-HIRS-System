package model

import "time"

type LeaveWorkflowRequest struct {
	ID           string     `json:"id"`
	EmployeeID   string     `json:"employee_id"`
	Employee     string     `json:"employee"`
	Type         string     `json:"type"`
	Start        string     `json:"start"`
	End          string     `json:"end"`
	Days         float64    `json:"days"`
	Reason       string     `json:"reason,omitempty"`
	Status       string     `json:"status"`
	ApproverID   string     `json:"approver_id,omitempty"`
	Approver     string     `json:"approver,omitempty"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	DecisionNote string     `json:"decision_note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CreateLeaveRequest struct {
	EmployeeID string `json:"employee_id"`
	Type       string `json:"leave_type"`
	Start      string `json:"start_date"`
	End        string `json:"end_date"`
	Reason     string `json:"reason"`
}

type LeaveDecision struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

type AttendanceEntry struct {
	ID                       string     `json:"id"`
	EmployeeID               string     `json:"employee_id"`
	Employee                 string     `json:"employee"`
	WorkDate                 string     `json:"work_date"`
	CheckIn                  *time.Time `json:"check_in,omitempty"`
	CheckOut                 *time.Time `json:"check_out,omitempty"`
	WorkMode                 string     `json:"work_mode"`
	Status                   string     `json:"status"`
	ScheduleID               string     `json:"schedule_id,omitempty"`
	ScheduleName             string     `json:"schedule_name,omitempty"`
	ScheduledStartAt         *time.Time `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt           *time.Time `json:"scheduled_end_at,omitempty"`
	BreakMinutes             int        `json:"break_minutes"`
	LateMinutes              int        `json:"late_minutes"`
	EarlyLeaveMinutes        int        `json:"early_leave_minutes"`
	WorkedMinutes            int        `json:"worked_minutes"`
	OvertimeMinutes          int        `json:"overtime_minutes"`
	OvertimeThresholdMinutes int        `json:"overtime_threshold_minutes"`
	Source                   string     `json:"source"`
}

type AttendanceAction struct {
	EmployeeID string `json:"employee_id"`
	WorkMode   string `json:"work_mode"`
}

type AuditEvent struct {
	ID           string         `json:"id"`
	ActorUserID  string         `json:"actor_user_id,omitempty"`
	Actor        string         `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}
