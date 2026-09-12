package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

func (s *Server) workSchedulesV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	schedules, err := s.store.ListWorkSchedules(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "schedule_list_error", "could not load work schedules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": schedules, "total": len(schedules)})
}

func (s *Server) upsertWorkScheduleV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.UpsertWorkSchedule
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Code = strings.TrimSpace(strings.ToLower(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if input.Code == "" || input.Name == "" || strings.ContainsAny(input.Code, " /\\") {
		writeError(w, http.StatusBadRequest, "validation_error", "code and name are required; code may not contain spaces or slashes")
		return
	}
	start, startErr := time.Parse("15:04", input.StartTime)
	end, endErr := time.Parse("15:04", input.EndTime)
	if startErr != nil || endErr != nil || !end.After(start) {
		writeError(w, http.StatusBadRequest, "validation_error", "start_time and end_time must use HH:MM and end_time must be later")
		return
	}
	if input.BreakMinutes < 0 || input.BreakMinutes > 480 || input.GraceMinutes < 0 || input.GraceMinutes > 240 || input.OvertimeThresholdMinutes < 0 || input.OvertimeThresholdMinutes > 480 {
		writeError(w, http.StatusBadRequest, "validation_error", "schedule minute values are outside supported limits")
		return
	}
	if len(input.WorkDays) == 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "at least one work day is required")
		return
	}
	seen := map[int]bool{}
	for _, day := range input.WorkDays {
		if day < 0 || day > 6 || seen[day] {
			writeError(w, http.StatusBadRequest, "validation_error", "work_days must contain unique values from 0 (Sunday) through 6 (Saturday)")
			return
		}
		seen[day] = true
	}
	schedule, err := s.store.UpsertWorkSchedule(r.Context(), principal.OrganizationID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "schedule_save_error", err.Error())
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.schedule_saved", "work_schedule", schedule.ID,
		workflowAuditMetadata(map[string]any{"code": schedule.Code, "default": schedule.IsDefault}))
	writeJSON(w, http.StatusOK, schedule)
}

func (s *Server) assignWorkScheduleV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.WorkScheduleAssignment
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.EmployeeID = strings.TrimSpace(input.EmployeeID)
	input.ScheduleID = r.PathValue("id")
	if input.EmployeeID == "" || strings.TrimSpace(input.EffectiveFrom) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "employee_id and effective_from are required")
		return
	}
	from, err := time.Parse("2006-01-02", input.EffectiveFrom)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "effective_from must use YYYY-MM-DD")
		return
	}
	if input.EffectiveTo != "" {
		to, err := time.Parse("2006-01-02", input.EffectiveTo)
		if err != nil || to.Before(from) {
			writeError(w, http.StatusBadRequest, "validation_error", "effective_to must be on or after effective_from")
			return
		}
	}
	if err := s.store.AssignWorkSchedule(r.Context(), principal.OrganizationID, input.ScheduleID, input); err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "schedule_assignment_error", "employee or schedule does not exist in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "schedule_assignment_error", "could not assign work schedule")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.schedule_assigned", "work_schedule", input.ScheduleID,
		workflowAuditMetadata(map[string]any{"employee_id": input.EmployeeID, "effective_from": input.EffectiveFrom, "effective_to": input.EffectiveTo}))
	writeJSON(w, http.StatusCreated, map[string]any{"assigned": true})
}

func (s *Server) attendanceV2(w http.ResponseWriter, r *http.Request) {
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
	entries, err := s.store.ListAttendanceV2(r.Context(), principal.OrganizationID, workDate, scopeID, includeReports)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attendance_list_error", "could not load attendance")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": entries, "total": len(entries), "date": workDate.Format("2006-01-02")})
}

func (s *Server) checkInV2(w http.ResponseWriter, r *http.Request) {
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
	source := "self_service"
	if principal.Role == "admin" || principal.Role == "hr" {
		source = "hr_override"
	}
	entry, err := s.store.CheckInV2(r.Context(), principal.OrganizationID, employeeID, input.WorkMode, source)
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "attendance_employee_error", "employee does not exist in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "attendance_checkin_error", "could not check in")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.checked_in", "attendance", entry.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "work_mode": input.WorkMode, "late_minutes": entry.LateMinutes}))
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) checkOutV2(w http.ResponseWriter, r *http.Request) {
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
	entry, err := s.store.CheckOutV2(r.Context(), principal.OrganizationID, employeeID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "not_checked_in", "employee has no open attendance entry")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "attendance_checkout_error", "could not check out")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.checked_out", "attendance", entry.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "worked_minutes": entry.WorkedMinutes, "overtime_minutes": entry.OvertimeMinutes}))
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) attendanceCorrectionsV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	scopeID, includeReports, ok := s.selfServiceScope(w, r, principal)
	if !ok {
		return
	}
	requests, err := s.store.ListAttendanceCorrections(r.Context(), principal.OrganizationID, scopeID, includeReports)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "correction_list_error", "could not load attendance corrections")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": requests, "total": len(requests)})
}

func (s *Server) createAttendanceCorrectionV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateAttendanceCorrection
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employeeID, ok := s.mutationEmployeeID(w, r, principal, input.EmployeeID)
	if !ok {
		return
	}
	if _, err := time.Parse("2006-01-02", input.WorkDate); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "work_date must use YYYY-MM-DD")
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
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "reason is required")
		return
	}
	var checkIn, checkOut *time.Time
	if strings.TrimSpace(input.RequestedCheckIn) != "" {
		parsed, err := time.Parse(time.RFC3339, input.RequestedCheckIn)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "requested_check_in must use RFC3339")
			return
		}
		checkIn = &parsed
	}
	if strings.TrimSpace(input.RequestedCheckOut) != "" {
		parsed, err := time.Parse(time.RFC3339, input.RequestedCheckOut)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "requested_check_out must use RFC3339")
			return
		}
		checkOut = &parsed
	}
	if checkIn == nil && checkOut == nil {
		writeError(w, http.StatusBadRequest, "validation_error", "at least one corrected timestamp is required")
		return
	}
	if checkIn != nil && checkOut != nil && checkOut.Before(*checkIn) {
		writeError(w, http.StatusBadRequest, "validation_error", "requested_check_out may not be before requested_check_in")
		return
	}
	request, err := s.store.CreateAttendanceCorrection(r.Context(), principal.OrganizationID, employeeID, principal.UserID, input, checkIn, checkOut)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "correction_pending", "a pending correction already exists for this employee and date")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "correction_create_error", "could not create attendance correction")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.correction_requested", "attendance_correction", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "work_date": input.WorkDate}))
	writeJSON(w, http.StatusCreated, request)
}

func (s *Server) decideAttendanceCorrectionV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.AttendanceCorrectionDecision
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Decision = strings.TrimSpace(strings.ToLower(input.Decision))
	if !workflowOneOf(input.Decision, "approved", "rejected") {
		writeError(w, http.StatusBadRequest, "validation_error", "decision must be approved or rejected")
		return
	}
	if principal.Role == "manager" {
		managerID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
		if err != nil || managerID == "" {
			writeError(w, http.StatusForbidden, "employee_link_required", "manager account is not linked to an employee profile")
			return
		}
		ownerID, err := s.store.AttendanceCorrectionOwner(r.Context(), principal.OrganizationID, r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "correction_not_found", "attendance correction does not exist")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "correction_permission_error", "could not validate correction scope")
			return
		}
		owns, err := s.store.ManagerOwnsEmployee(r.Context(), principal.OrganizationID, managerID, ownerID)
		if err != nil || !owns {
			writeError(w, http.StatusForbidden, "forbidden", "managers may review corrections only for direct reports")
			return
		}
	}
	request, err := s.store.DecideAttendanceCorrection(r.Context(), principal.OrganizationID, r.PathValue("id"), principal.UserID, input.Decision, strings.TrimSpace(input.Note))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "correction_not_found", "attendance correction does not exist")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "correction_decided", "attendance correction is no longer pending")
		return
	}
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "correction_invalid", "corrected timestamps conflict with attendance constraints")
			return
		}
		writeError(w, http.StatusInternalServerError, "correction_decision_error", "could not decide attendance correction")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.correction_"+input.Decision, "attendance_correction", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": request.EmployeeID, "work_date": request.WorkDate, "note": input.Note}))
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) attendanceTimesheetV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	from, err := time.Parse("2006-01-02", strings.TrimSpace(r.URL.Query().Get("from")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "from must use YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(r.URL.Query().Get("to")))
	if err != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		writeError(w, http.StatusBadRequest, "validation_error", "to must be on or after from and the range may not exceed 366 days")
		return
	}
	employeeID, ok := s.attendanceReadEmployeeID(w, r, principal, strings.TrimSpace(r.URL.Query().Get("employee_id")))
	if !ok {
		return
	}
	summary, err := s.store.AttendanceTimesheet(r.Context(), principal.OrganizationID, employeeID, from, to)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "employee_not_found", "employee does not exist")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "timesheet_error", "could not build attendance timesheet")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) attendanceReadEmployeeID(w http.ResponseWriter, r *http.Request, principal model.Principal, requested string) (string, bool) {
	if principal.Role == "admin" || principal.Role == "hr" {
		if requested == "" {
			writeError(w, http.StatusBadRequest, "employee_required", "employee_id is required")
			return "", false
		}
		return requested, true
	}
	ownID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil || ownID == "" {
		writeError(w, http.StatusForbidden, "employee_link_required", "account is not linked to an employee profile")
		return "", false
	}
	if requested == "" || requested == ownID {
		return ownID, true
	}
	if principal.Role == "manager" {
		owns, err := s.store.ManagerOwnsEmployee(r.Context(), principal.OrganizationID, ownID, requested)
		if err == nil && owns {
			return requested, true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "requested employee is outside your attendance scope")
	return "", false
}
