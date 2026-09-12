package model

type CreateLeaveRequestV2 struct {
	EmployeeID string `json:"employee_id"`
	Type       string `json:"leave_type"`
	Start      string `json:"start_date"`
	End        string `json:"end_date"`
	PartialDay string `json:"partial_day"`
	Reason     string `json:"reason"`
}

type AssignLeavePolicy struct {
	EmployeeID    string `json:"employee_id"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
}
