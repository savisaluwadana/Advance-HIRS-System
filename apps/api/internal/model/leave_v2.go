package model

import "time"

type LeavePolicy struct {
	ID                string    `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	LeaveType         string    `json:"leave_type"`
	AnnualEntitlement float64   `json:"annual_entitlement"`
	CarryOverLimit    float64   `json:"carry_over_limit"`
	TrackBalance      bool      `json:"track_balance"`
	AllowNegative     bool      `json:"allow_negative"`
	RequiresApproval  bool      `json:"requires_approval"`
	IsDefault         bool      `json:"is_default"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type LeaveBalance struct {
	EmployeeID    string  `json:"employee_id"`
	PolicyID      string  `json:"policy_id"`
	PolicyCode    string  `json:"policy_code"`
	PolicyName    string  `json:"policy_name"`
	LeaveType     string  `json:"leave_type"`
	Year          int     `json:"year"`
	Entitlement   float64 `json:"entitlement"`
	CarryOver     float64 `json:"carry_over"`
	Adjustments   float64 `json:"adjustments"`
	Used          float64 `json:"used"`
	Available     float64 `json:"available"`
	TrackBalance  bool    `json:"track_balance"`
	AllowNegative bool    `json:"allow_negative"`
}

type LeaveBalanceEvent struct {
	ID             string    `json:"id"`
	EmployeeID     string    `json:"employee_id"`
	PolicyID       string    `json:"policy_id"`
	LeaveRequestID string    `json:"leave_request_id,omitempty"`
	Year           int       `json:"year"`
	Amount         float64   `json:"amount"`
	EventType      string    `json:"event_type"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type CompanyHoliday struct {
	ID        string    `json:"id"`
	Date      string    `json:"date"`
	Name      string    `json:"name"`
	Location  string    `json:"location,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateLeavePolicy struct {
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	LeaveType         string  `json:"leave_type"`
	AnnualEntitlement float64 `json:"annual_entitlement"`
	CarryOverLimit    float64 `json:"carry_over_limit"`
	TrackBalance      bool    `json:"track_balance"`
	AllowNegative     bool    `json:"allow_negative"`
	RequiresApproval  bool    `json:"requires_approval"`
}

type LeaveAdjustment struct {
	EmployeeID string  `json:"employee_id"`
	PolicyID   string  `json:"policy_id"`
	Year       int     `json:"year"`
	Amount     float64 `json:"amount"`
	Note       string  `json:"note"`
}

type CreateHoliday struct {
	Date     string `json:"date"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

type LeaveCancellation struct {
	Note string `json:"note"`
}
