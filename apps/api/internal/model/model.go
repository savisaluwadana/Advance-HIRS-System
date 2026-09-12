package model

import "time"

type Principal struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	OrganizationID string `json:"organization_id"`
	Organization   string `json:"organization"`
	Role           string `json:"role"`
}

type Employee struct {
	ID             string    `json:"id"`
	EmployeeNumber string    `json:"employee_number,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	LastName       string    `json:"last_name,omitempty"`
	Name           string    `json:"name"`
	WorkEmail      string    `json:"work_email,omitempty"`
	Role           string    `json:"role"`
	Department     string    `json:"department"`
	Location       string    `json:"location"`
	EmploymentType string    `json:"employment_type,omitempty"`
	Status         string    `json:"status"`
	ServerVersion  int64     `json:"server_version"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateEmployee struct {
	EmployeeNumber string `json:"employee_number"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	WorkEmail      string `json:"work_email"`
	Role           string `json:"role"`
	Department     string `json:"department"`
	Location       string `json:"location"`
	EmploymentType string `json:"employment_type"`
	Status         string `json:"status"`
}

type PatchEmployee struct {
	EmployeeNumber *string `json:"employee_number"`
	FirstName      *string `json:"first_name"`
	LastName       *string `json:"last_name"`
	WorkEmail      *string `json:"work_email"`
	Role           *string `json:"role"`
	Department     *string `json:"department"`
	Location       *string `json:"location"`
	EmploymentType *string `json:"employment_type"`
	Status         *string `json:"status"`
}

type LeaveRequest struct {
	ID       string `json:"id"`
	Employee string `json:"employee"`
	Type     string `json:"type"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Days     int    `json:"days"`
	Status   string `json:"status"`
}

type SyncConflict struct {
	OperationID   string   `json:"operation_id"`
	EntityID      string   `json:"entity_id"`
	ServerVersion int64    `json:"server_version"`
	ServerRecord  Employee `json:"server_record"`
}

type LoginIdentity struct {
	Principal
	PasswordHash string
}
