package model

import "time"

type AccessInvitation struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Organization   string     `json:"organization"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	EmployeeID     string     `json:"employee_id,omitempty"`
	Employee       string     `json:"employee,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedBy      string     `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreateAccessInvitation struct {
	Email      string `json:"email"`
	Role       string `json:"role"`
	EmployeeID string `json:"employee_id"`
}

type InvitationPreview struct {
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	EmployeeID   string    `json:"employee_id,omitempty"`
	Employee     string    `json:"employee,omitempty"`
	Organization string    `json:"organization"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExistingUser bool      `json:"existing_user"`
}

type AcceptInvitation struct {
	Token       string `json:"token"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}
