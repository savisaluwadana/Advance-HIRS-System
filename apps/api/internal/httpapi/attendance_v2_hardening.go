package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

func (s *Server) assignWorkScheduleSafeV2(w http.ResponseWriter, r *http.Request) {
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
	if err := s.store.AssignWorkScheduleSafe(r.Context(), principal.OrganizationID, input.ScheduleID, input); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusBadRequest, "schedule_assignment_error", "employee or schedule does not exist in this organization")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "schedule_assignment_overlap", "employee already has an overlapping schedule assignment")
		default:
			writeError(w, http.StatusInternalServerError, "schedule_assignment_error", "could not assign work schedule")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "attendance.schedule_assigned", "work_schedule", input.ScheduleID,
		workflowAuditMetadata(map[string]any{"employee_id": input.EmployeeID, "effective_from": input.EffectiveFrom, "effective_to": input.EffectiveTo}))
	writeJSON(w, http.StatusCreated, map[string]any{"assigned": true})
}

func (s *Server) checkOutTodayV2(w http.ResponseWriter, r *http.Request) {
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
	entry, err := s.store.CheckOutTodayV2(r.Context(), principal.OrganizationID, employeeID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "not_checked_in", "employee has no open attendance entry for today")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "attendance_changed", "attendance entry changed before checkout could complete")
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
