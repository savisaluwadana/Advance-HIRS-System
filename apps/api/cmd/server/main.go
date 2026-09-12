package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type Employee struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Department string `json:"department"`
	Location   string `json:"location"`
	Status     string `json:"status"`
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

var employees = []Employee{
	{ID: "emp_001", Name: "Ava Morgan", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", Status: "active"},
	{ID: "emp_002", Name: "Daniel Ng", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", Status: "active"},
	{ID: "emp_003", Name: "Sara Ibrahim", Role: "People Operations Lead", Department: "People", Location: "Dubai", Status: "active"},
	{ID: "emp_004", Name: "Jonas Lee", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", Status: "leave"},
}

var leaveRequests = []LeaveRequest{
	{ID: "leave_101", Employee: "Mia Novak", Type: "annual", Start: "2026-09-18", End: "2026-09-20", Days: 3, Status: "pending"},
	{ID: "leave_102", Employee: "Ravi Kumar", Type: "remote", Start: "2026-09-16", End: "2026-09-16", Days: 1, Status: "pending"},
	{ID: "leave_103", Employee: "Ella Thompson", Type: "annual", Start: "2026-10-02", End: "2026-10-06", Days: 3, Status: "pending"},
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "advance-hris-api", "time": time.Now().UTC()})
	})
	mux.HandleFunc("GET /api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"employees": 248,
			"present_today": 224,
			"on_leave": 14,
			"open_roles": 18,
			"pending_leave_requests": len(leaveRequests),
		})
	})
	mux.HandleFunc("GET /api/v1/employees", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": employees, "total": len(employees)})
	})
	mux.HandleFunc("GET /api/v1/leave/requests", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": leaveRequests, "total": len(leaveRequests)})
	})

	port := os.Getenv("API_PORT")
	if port == "" { port = "8080" }
	server := &http.Server{Addr: ":" + port, Handler: withMiddleware(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("advance HRIS API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil { log.Printf("encode response: %v", err) }
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
