package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

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

type LeaveRequest struct {
	ID       string `json:"id"`
	Employee string `json:"employee"`
	Type     string `json:"type"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Days     int    `json:"days"`
	Status   string `json:"status"`
}

type SyncOperation struct {
	ID          string          `json:"id"`
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	Action      string          `json:"action"`
	Payload     json.RawMessage `json:"payload"`
	BaseVersion int64           `json:"base_version"`
	ChangedAt   time.Time       `json:"changed_at"`
}

type SyncPushRequest struct {
	Operations []SyncOperation `json:"operations"`
}

type SyncConflict struct {
	OperationID   string   `json:"operation_id"`
	EntityID      string   `json:"entity_id"`
	ServerVersion int64    `json:"server_version"`
	ServerRecord  Employee `json:"server_record"`
}

var (
	employeesMu sync.RWMutex
	employees   = []Employee{
		{ID: "emp_001", Name: "Ava Morgan", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", Status: "active", ServerVersion: 1, UpdatedAt: time.Now().UTC()},
		{ID: "emp_002", Name: "Daniel Ng", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", Status: "active", ServerVersion: 1, UpdatedAt: time.Now().UTC()},
		{ID: "emp_003", Name: "Sara Ibrahim", Role: "People Operations Lead", Department: "People", Location: "Dubai", Status: "active", ServerVersion: 1, UpdatedAt: time.Now().UTC()},
		{ID: "emp_004", Name: "Jonas Lee", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", Status: "leave", ServerVersion: 1, UpdatedAt: time.Now().UTC()},
	}
	leaveRequests = []LeaveRequest{
		{ID: "leave_101", Employee: "Mia Novak", Type: "annual", Start: "2026-09-18", End: "2026-09-20", Days: 3, Status: "pending"},
		{ID: "leave_102", Employee: "Ravi Kumar", Type: "remote", Start: "2026-09-16", End: "2026-09-16", Days: 1, Status: "pending"},
		{ID: "leave_103", Employee: "Ella Thompson", Type: "annual", Start: "2026-10-02", End: "2026-10-06", Days: 3, Status: "pending"},
	}
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/v1/dashboard", dashboardHandler)
	mux.HandleFunc("GET /api/v1/employees", employeesHandler)
	mux.HandleFunc("GET /api/v1/leave/requests", leaveRequestsHandler)
	mux.HandleFunc("POST /api/v1/sync/push", syncPushHandler)
	mux.HandleFunc("GET /api/v1/sync/pull", syncPullHandler)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           withMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("advance HRIS API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "service": "advance-hris-api", "time": time.Now().UTC(),
	})
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	employeesMu.RLock()
	count := len(employees)
	employeesMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"employees":              count,
		"present_today":          224,
		"on_leave":               14,
		"open_roles":             18,
		"pending_leave_requests": len(leaveRequests),
	})
}

func employeesHandler(w http.ResponseWriter, r *http.Request) {
	employeesMu.RLock()
	snapshot := append([]Employee(nil), employees...)
	employeesMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"data": snapshot, "total": len(snapshot)})
}

func leaveRequestsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": leaveRequests, "total": len(leaveRequests)})
}

func syncPushHandler(w http.ResponseWriter, r *http.Request) {
	var request SyncPushRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid sync request"})
		return
	}

	accepted := make([]string, 0, len(request.Operations))
	conflicts := make([]SyncConflict, 0)

	employeesMu.Lock()
	defer employeesMu.Unlock()

	for _, operation := range request.Operations {
		if operation.EntityType != "employee" || operation.Action != "upsert" {
			continue
		}

		var incoming Employee
		if err := json.Unmarshal(operation.Payload, &incoming); err != nil {
			continue
		}
		if incoming.ID == "" {
			incoming.ID = operation.EntityID
		}
		if incoming.ID == "" {
			continue
		}

		index := -1
		for i := range employees {
			if employees[i].ID == incoming.ID {
				index = i
				break
			}
		}

		if index >= 0 {
			current := employees[index]
			if operation.BaseVersion != 0 && operation.BaseVersion < current.ServerVersion {
				conflicts = append(conflicts, SyncConflict{
					OperationID: operation.ID, EntityID: incoming.ID,
					ServerVersion: current.ServerVersion, ServerRecord: current,
				})
				continue
			}
			incoming.ServerVersion = current.ServerVersion + 1
			incoming.UpdatedAt = time.Now().UTC()
			employees[index] = incoming
		} else {
			incoming.ServerVersion = 1
			incoming.UpdatedAt = time.Now().UTC()
			employees = append(employees, incoming)
		}
		accepted = append(accepted, operation.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accepted":    accepted,
		"conflicts":   conflicts,
		"server_time": time.Now().UTC(),
	})
}

func syncPullHandler(w http.ResponseWriter, r *http.Request) {
	employeesMu.RLock()
	snapshot := append([]Employee(nil), employees...)
	employeesMu.RUnlock()

	// v1 intentionally returns a complete authoritative snapshot. The cursor
	// exists now so the desktop contract can evolve to delta sync without
	// changing the client-facing endpoint.
	writeJSON(w, http.StatusOK, map[string]any{
		"employees": snapshot,
		"cursor":    time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
