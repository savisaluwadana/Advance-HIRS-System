package domain

import "time"

type LeaveRequest struct {
	ID           string     `json:"id"`
	EmployeeID   string     `json:"employee_id"`
	Employee     string     `json:"employee"`
	Type         string     `json:"type"`
	Start        string     `json:"start"`
	End          string     `json:"end"`
	Days         float64    `json:"days"`
	Reason       string     `json:"reason,omitempty"`
	Status       string     `json:"status"`
	Approver     string     `json:"approver,omitempty"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	DecisionNote string     `json:"decision_note,omitempty"`
}

type AttendanceEntry struct {
	ID         string     `json:"id"`
	EmployeeID string     `json:"employee_id"`
	Employee   string     `json:"employee"`
	WorkDate   string     `json:"work_date"`
	CheckIn    *time.Time `json:"check_in,omitempty"`
	CheckOut   *time.Time `json:"check_out,omitempty"`
	WorkMode   string     `json:"work_mode"`
	Status     string     `json:"status"`
}

type AuditEvent struct {
	ID           string         `json:"id"`
	Actor        string         `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type OperationsState struct {
	LeaveRequests []LeaveRequest    `json:"leave_requests"`
	Attendance    []AttendanceEntry `json:"attendance"`
	Audit         []AuditEvent      `json:"audit"`
}
