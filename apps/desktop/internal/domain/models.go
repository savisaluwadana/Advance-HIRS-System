package domain

import "time"

type Employee struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	Department    string    `json:"department"`
	Location      string    `json:"location"`
	Status        string    `json:"status"`
	ServerVersion int64     `json:"server_version"`
	UpdatedAt     time.Time `json:"updated_at"`
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

type DesktopState struct {
	Employees    []Employee `json:"employees"`
	PendingSync  int        `json:"pending_sync"`
	LastSyncAt   string     `json:"last_sync_at"`
	Offline      bool       `json:"offline"`
	APIBaseURL   string     `json:"api_base_url"`
	StorageReady bool       `json:"storage_ready"`
}

func DemoEmployees() []Employee {
	now := time.Now().UTC()
	return []Employee{
		{ID: "emp_001", Name: "Ava Morgan", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", Status: "active", UpdatedAt: now},
		{ID: "emp_002", Name: "Daniel Ng", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", Status: "active", UpdatedAt: now},
		{ID: "emp_003", Name: "Sara Ibrahim", Role: "People Operations Lead", Department: "People", Location: "Dubai", Status: "active", UpdatedAt: now},
		{ID: "emp_004", Name: "Jonas Lee", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", Status: "leave", UpdatedAt: now},
	}
}
