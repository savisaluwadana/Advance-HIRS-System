package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

// NewHandlerV2 extends the foundation handler with leave, attendance and audit
// workflows while keeping all authorization checks server-side.
func NewHandlerV2(dataStore *store.Store, authManager *auth.Manager, allowedOrigin string) http.Handler {
	server := &Server{store: dataStore, auth: authManager, allowedOrigin: allowedOrigin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("POST /api/v1/auth/login", server.login)
	mux.HandleFunc("GET /api/v1/auth/me", server.authorize(server.me, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("GET /api/v1/dashboard", server.authorize(server.dashboard, "admin", "hr", "manager"))
	mux.HandleFunc("GET /api/v1/employees", server.authorize(server.listEmployees, "admin", "hr", "manager"))
	mux.HandleFunc("POST /api/v1/employees", server.authorize(server.createEmployee, "admin", "hr"))
	mux.HandleFunc("PATCH /api/v1/employees/{id}", server.authorize(server.patchEmployee, "admin", "hr"))
	mux.HandleFunc("DELETE /api/v1/employees/{id}", server.authorize(server.archiveEmployee, "admin", "hr"))

	mux.HandleFunc("GET /api/v1/leave/requests", server.authorize(server.workflowLeaveRequests, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/requests", server.authorize(server.createLeaveRequest, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/leave/requests/{id}/decision", server.authorize(server.decideLeaveRequest, "admin", "hr", "manager"))

	mux.HandleFunc("GET /api/v1/attendance", server.authorize(server.attendance, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/attendance/check-in", server.authorize(server.checkIn, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("POST /api/v1/attendance/check-out", server.authorize(server.checkOut, "admin", "hr", "manager", "employee"))
	mux.HandleFunc("GET /api/v1/audit", server.authorize(server.auditEvents, "admin", "hr"))

	mux.HandleFunc("POST /api/v1/sync/push", server.authorize(server.syncPush, "admin", "hr"))
	mux.HandleFunc("GET /api/v1/sync/pull", server.authorize(server.syncPull, "admin", "hr"))
	return server.withMiddleware(mux)
}

func (s *Server) workflowLeaveRequests(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	scopeID, includeReports, ok := s.selfServiceScope(w, r, principal)
	if !ok {
		return
	}
	requests, err := s.store.ListLeaveRequestsScoped(r.Context(), principal.OrganizationID, scopeID, includeReports)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leave_list_error", "could not load leave requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": requests, "total": len(requests)})
}

func (s *Server) createLeaveRequest(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateLeaveRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employeeID, ok := s.mutationEmployeeID(w, r, principal, input.EmployeeID)
	if !ok {
		return
	}
	input.Type = strings.TrimSpace(strings.ToLower(input.Type))
	if !workflowOneOf(input.Type, "annual", "sick", "unpaid", "remote", "parental", "other") {
		writeError(w, http.StatusBadRequest, "validation_error", "unsupported leave_type")
		return
	}
	start, err := time.Parse("2006-01-02", input.Start)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "start_date must use YYYY-MM-DD")
		return
	}
	end, err := time.Parse("2006-01-02", input.End)
	if err != nil || end.Before(start) {
		writeError(w, http.StatusBadRequest, "validation_error", "end_date must use YYYY-MM-DD and be on or after start_date")
		return
	}
	days := end.Sub(start).Hours()/24 + 1
	request, err := s.store.CreateLeaveRequest(r.Context(), principal.OrganizationID, employeeID, input.Type, start, end, days, strings.TrimSpace(input.Reason))
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "leave_employee_error", "employee does not exist in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "leave_create_error", "could not create leave request")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.requested", "leave_request", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "leave_type": input.Type}))
	writeJSON(w, http.StatusCreated, request)
}

func (s *Server) decideLeaveRequest(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.LeaveDecision
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Decision = strings.TrimSpace(strings.ToLower(input.Decision))
	if input.Decision != "approved" && input.Decision != "rejected" {
		writeError(w, http.StatusBadRequest, "validation_error", "decision must be approved or rejected")
		return
	}

	approverEmployeeID, _ := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if principal.Role == "manager" {
		if approverEmployeeID == "" {
			writeError(w, http.StatusForbidden, "employee_link_required", "manager account is not linked to an employee profile")
			return
		}
		owns, err := s.store.ManagerOwnsLeaveRequest(r.Context(), principal.OrganizationID, approverEmployeeID, r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "leave_permission_error", "could not validate manager scope")
			return
		}
		if !owns {
			writeError(w, http.StatusForbidden, "forbidden", "managers may decide leave only for direct reports")
			return
		}
	}

	request, err := s.store.DecideLeaveRequest(r.Context(), principal.OrganizationID, r.PathValue("id"), approverEmployeeID, input.Decision, strings.TrimSpace(input.Note))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "leave_not_found", "leave request does not exist")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "leave_already_decided", "leave request is no longer pending")
		default:
			writeError(w, http.StatusInternalServerError, "leave_decision_error", "could not decide leave request")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave."+input.Decision, "leave_request", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": request.EmployeeID, "note": input.Note}))
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) attendance(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	scopeID, includeReports, ok := s.selfServiceScope(w, r, principal)
	if !ok {
		return
	}
	workDate := time.Now().UTC()
	if raw := strings.TrimSpace(r.URL.Query().Get("date")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "date must use YYYY-MM-DD")
			return
		}
		workDate = parsed
	}
	entries, err := s.store.ListAttendance(r.Context(), principal.OrganizationID, workDate, scopeID, includeReports)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attendance_list_error", "could not load attendance")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": entries, "total": len(entries), "date": workDate.Format("2006-01-02")})
}

func (s *Server) checkIn(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.AttendanceAction
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employeeID, ok := s.mutationEmployeeID(w, r, principal, input.EmployeeID)
	if !ok {
		return
	}
	input.WorkMode = strings.TrimSpace(strings.ToLower(input.WorkMode))
	if input.WorkMode == "" {
		input.WorkMode = "office"
	}
	if !workflowOneOf(input.WorkMode, "office", "remote", "hybrid", "field") {
		writeError(w, http.StatusBadRequest, "validation_error", "unsupported work_mode")
		return
	}
	entry, err := s.store.CheckIn(r.Context(), principal.OrganizationID, employeeID, input.WorkMode)
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "attendance_employee_error", "employee does not exist in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "attendance_checkin_error", "could not check in")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.checked_in", "attendance", entry.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "work_mode": input.WorkMode}))
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) checkOut(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.AttendanceAction
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employeeID, ok := s.mutationEmployeeID(w, r, principal, input.EmployeeID)
	if !ok {
		return
	}
	entry, err := s.store.CheckOut(r.Context(), principal.OrganizationID, employeeID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "not_checked_in", "employee has not checked in today")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attendance_checkout_error", "could not check out")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.checked_out", "attendance", entry.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID}))
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) auditEvents(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	events, err := s.store.AuditEvents(r.Context(), principal.OrganizationID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "audit_list_error", "could not load audit history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": events, "total": len(events)})
}

func (s *Server) selfServiceScope(w http.ResponseWriter, r *http.Request, principal model.Principal) (string, bool, bool) {
	if principal.Role == "admin" || principal.Role == "hr" {
		return "", false, true
	}
	employeeID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil || employeeID == "" {
		writeError(w, http.StatusForbidden, "employee_link_required", "account is not linked to an employee profile")
		return "", false, false
	}
	if principal.Role == "manager" {
		return employeeID, true, true
	}
	return employeeID, false, true
}

func (s *Server) mutationEmployeeID(w http.ResponseWriter, r *http.Request, principal model.Principal, requested string) (string, bool) {
	requested = strings.TrimSpace(requested)
	if principal.Role == "admin" || principal.Role == "hr" {
		if requested == "" {
			writeError(w, http.StatusBadRequest, "employee_required", "employee_id is required")
			return "", false
		}
		return requested, true
	}
	employeeID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil || employeeID == "" {
		writeError(w, http.StatusForbidden, "employee_link_required", "account is not linked to an employee profile")
		return "", false
	}
	if requested != "" && requested != employeeID {
		writeError(w, http.StatusForbidden, "forbidden", "you may perform this action only for your own employee profile")
		return "", false
	}
	return employeeID, true
}

func workflowOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func workflowAuditMetadata(value map[string]any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}
