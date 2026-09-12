package domain

import "time"

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

type SyncOperation struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Action      string    `json:"action"`
	Payload     Employee  `json:"payload"`
	BaseVersion int64     `json:"base_version"`
	ChangedAt   time.Time `json:"changed_at"`
}

type SyncResult struct {
	Pushed     int       `json:"pushed"`
	Pulled     int       `json:"pulled"`
	Pending    int       `json:"pending"`
	Conflicts  int       `json:"conflicts"`
	Cursor     string    `json:"cursor"`
	LastSyncAt time.Time `json:"last_sync_at"`
	Offline    bool      `json:"offline"`
	Message    string    `json:"message"`
}

type AuthUser struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	OrganizationID string `json:"organization_id"`
	Organization   string `json:"organization"`
	Role           string `json:"role"`
}

type AuthSession struct {
	Authenticated bool      `json:"authenticated"`
	ExpiresAt     time.Time `json:"expires_at"`
	User          AuthUser  `json:"user"`
}

type DesktopState struct {
	Employees     []Employee `json:"employees"`
	PendingSync   int        `json:"pending_sync"`
	LastSyncAt    string     `json:"last_sync_at"`
	Offline       bool       `json:"offline"`
	APIBaseURL    string     `json:"api_base_url"`
	StorageReady  bool       `json:"storage_ready"`
	Authenticated bool       `json:"authenticated"`
	User          AuthUser   `json:"user"`
}

func DemoEmployees() []Employee {
	now := time.Now().UTC()
	return []Employee{
		{ID: "emp_001", EmployeeNumber: "EMP-001", FirstName: "Ava", LastName: "Morgan", Name: "Ava Morgan", WorkEmail: "ava@northstar.local", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", EmploymentType: "full_time", Status: "active", UpdatedAt: now},
		{ID: "emp_002", EmployeeNumber: "EMP-002", FirstName: "Daniel", LastName: "Ng", Name: "Daniel Ng", WorkEmail: "daniel@northstar.local", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", EmploymentType: "full_time", Status: "active", UpdatedAt: now},
		{ID: "emp_003", EmployeeNumber: "EMP-003", FirstName: "Sara", LastName: "Ibrahim", Name: "Sara Ibrahim", WorkEmail: "sara@northstar.local", Role: "People Operations Lead", Department: "People", Location: "Dubai", EmploymentType: "full_time", Status: "active", UpdatedAt: now},
		{ID: "emp_004", EmployeeNumber: "EMP-004", FirstName: "Jonas", LastName: "Lee", Name: "Jonas Lee", WorkEmail: "jonas@northstar.local", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", EmploymentType: "full_time", Status: "leave", UpdatedAt: now},
	}
}
