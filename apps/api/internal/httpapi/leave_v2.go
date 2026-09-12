package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

func (s *Server) leavePoliciesV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	policies, err := s.store.ListLeavePolicies(r.Context(), principal.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leave_policy_list_error", "could not load leave policies")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": policies, "total": len(policies)})
}

func (s *Server) upsertLeavePolicyV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateLeavePolicy
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.LeaveType = strings.ToLower(strings.TrimSpace(input.LeaveType))
	if input.Code == "" || input.Name == "" || !workflowOneOf(input.LeaveType, "annual", "sick", "unpaid", "remote", "parental", "other") {
		writeError(w, http.StatusBadRequest, "validation_error", "code, name and a supported leave_type are required")
		return
	}
	if input.AnnualEntitlement < 0 || input.CarryOverLimit < 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "entitlement and carry-over values cannot be negative")
		return
	}
	if !input.RequiresApproval {
		writeError(w, http.StatusBadRequest, "validation_error", "automatic approval is not enabled yet; requires_approval must be true")
		return
	}
	policy, err := s.store.CreateOrUpdateLeavePolicy(r.Context(), principal.OrganizationID, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leave_policy_save_error", "could not save leave policy")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.policy_saved", "leave_policy", policy.ID,
		workflowAuditMetadata(map[string]any{"leave_type": policy.LeaveType, "code": policy.Code}))
	writeJSON(w, http.StatusOK, policy)
}

func (s *Server) assignLeavePolicyV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.AssignLeavePolicy
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.EmployeeID = strings.TrimSpace(input.EmployeeID)
	if input.EmployeeID == "" {
		writeError(w, http.StatusBadRequest, "employee_required", "employee_id is required")
		return
	}
	if err := s.store.AssignLeavePolicy(r.Context(), principal.OrganizationID, r.PathValue("id"), input); err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "leave_assignment_error", "employee or policy does not exist in this organization")
			return
		}
		writeError(w, http.StatusBadRequest, "leave_assignment_error", err.Error())
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.policy_assigned", "leave_policy", r.PathValue("id"),
		workflowAuditMetadata(map[string]any{"employee_id": input.EmployeeID}))
	writeJSON(w, http.StatusCreated, map[string]any{"assigned": true})
}

func (s *Server) leaveBalancesV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	employeeID, ok := s.balanceScopeEmployeeID(w, r, principal)
	if !ok {
		return
	}
	year, ok := parseYearQuery(w, r)
	if !ok {
		return
	}
	balances, err := s.store.ListLeaveBalances(r.Context(), principal.OrganizationID, employeeID, year)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || isConstraintError(err) {
			writeError(w, http.StatusNotFound, "employee_not_found", "employee does not exist in this organization")
			return
		}
		writeError(w, http.StatusInternalServerError, "leave_balance_error", "could not load leave balances")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"employee_id": employeeID, "year": year, "data": balances})
}

func (s *Server) leaveBalanceLedgerV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	employeeID, ok := s.balanceScopeEmployeeID(w, r, principal)
	if !ok {
		return
	}
	year, ok := parseYearQuery(w, r)
	if !ok {
		return
	}
	events, err := s.store.LeaveBalanceLedger(r.Context(), principal.OrganizationID, employeeID, strings.TrimSpace(r.URL.Query().Get("policy_id")), year)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leave_ledger_error", "could not load leave balance history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"employee_id": employeeID, "year": year, "data": events})
}

func (s *Server) adjustLeaveBalanceV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.LeaveAdjustment
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.EmployeeID = strings.TrimSpace(input.EmployeeID)
	input.PolicyID = strings.TrimSpace(input.PolicyID)
	input.Note = strings.TrimSpace(input.Note)
	if input.EmployeeID == "" || input.PolicyID == "" || input.Amount == 0 || input.Note == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "employee_id, policy_id, non-zero amount and note are required")
		return
	}
	event, err := s.store.AdjustLeaveBalance(r.Context(), principal.OrganizationID, principal.UserID, input)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || isConstraintError(err) {
			writeError(w, http.StatusBadRequest, "leave_adjustment_error", "employee or policy is invalid")
			return
		}
		writeError(w, http.StatusInternalServerError, "leave_adjustment_error", "could not adjust leave balance")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.balance_adjusted", "leave_balance", event.ID,
		workflowAuditMetadata(map[string]any{"employee_id": input.EmployeeID, "policy_id": input.PolicyID, "amount": input.Amount}))
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) holidaysV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	year, ok := parseYearQuery(w, r)
	if !ok {
		return
	}
	holidays, err := s.store.ListHolidays(r.Context(), principal.OrganizationID, year)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "holiday_list_error", "could not load company holidays")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"year": year, "data": holidays})
}

func (s *Server) createHolidayV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateHoliday
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil || strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "date must use YYYY-MM-DD and name is required")
		return
	}
	holiday, err := s.store.CreateHoliday(r.Context(), principal.OrganizationID, input)
	if err != nil {
		if isConstraintError(err) {
			writeError(w, http.StatusConflict, "holiday_conflict", "that holiday already exists for this location")
			return
		}
		writeError(w, http.StatusInternalServerError, "holiday_create_error", "could not create company holiday")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.holiday_created", "company_holiday", holiday.ID,
		workflowAuditMetadata(map[string]any{"date": holiday.Date, "location": holiday.Location}))
	writeJSON(w, http.StatusCreated, holiday)
}

func (s *Server) deleteHolidayV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	if err := s.store.DeleteHoliday(r.Context(), principal.OrganizationID, r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "holiday_not_found", "holiday does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "holiday_delete_error", "could not delete company holiday")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.holiday_deleted", "company_holiday", r.PathValue("id"), `{}`)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createLeaveRequestV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.CreateLeaveRequestV2
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	employeeID, ok := s.mutationEmployeeID(w, r, principal, input.EmployeeID)
	if !ok {
		return
	}
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	if !workflowOneOf(input.Type, "annual", "sick", "unpaid", "remote", "parental", "other") {
		writeError(w, http.StatusBadRequest, "validation_error", "unsupported leave_type")
		return
	}
	if input.PartialDay == "" {
		input.PartialDay = "full"
	}
	if !workflowOneOf(input.PartialDay, "full", "first_half", "second_half") {
		writeError(w, http.StatusBadRequest, "validation_error", "partial_day must be full, first_half or second_half")
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
	request, err := s.store.CreateLeaveRequestV2(r.Context(), principal.OrganizationID, employeeID, input.Type, start, end, input.PartialDay, strings.TrimSpace(input.Reason))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "leave_overlap", "this employee already has pending or approved leave in that date range")
		case errors.Is(err, store.ErrNoWorkingDays):
			writeError(w, http.StatusUnprocessableEntity, "no_working_days", "the selected dates contain no working days after weekends and company holidays")
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "employee_not_found", "employee does not exist in this organization")
		default:
			writeError(w, http.StatusInternalServerError, "leave_create_error", "could not create leave request")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.requested", "leave_request", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": employeeID, "leave_type": input.Type, "days": request.Days, "partial_day": input.PartialDay}))
	writeJSON(w, http.StatusCreated, request)
}

func (s *Server) decideLeaveRequestV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.LeaveDecision
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Decision = strings.ToLower(strings.TrimSpace(input.Decision))
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
	request, err := s.store.DecideLeaveRequestV2(r.Context(), principal.OrganizationID, r.PathValue("id"), approverEmployeeID, input.Decision, strings.TrimSpace(input.Note), principal.UserID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "leave_not_found", "leave request does not exist")
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "leave_already_decided", "leave request is no longer pending")
		case errors.Is(err, store.ErrInsufficientLeaveBalance):
			writeError(w, http.StatusConflict, "insufficient_leave_balance", "the employee does not have enough available balance for this approval")
		default:
			writeError(w, http.StatusInternalServerError, "leave_decision_error", "could not decide leave request")
		}
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave."+input.Decision, "leave_request", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": request.EmployeeID, "days": request.Days}))
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) cancelLeaveRequestV2(w http.ResponseWriter, r *http.Request) {
	principal := principalFromContext(r.Context())
	var input model.LeaveCancellation
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	owner, err := s.store.LeaveRequestOwner(r.Context(), principal.OrganizationID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "leave_not_found", "leave request does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "leave_cancel_error", "could not validate leave owner")
		return
	}
	if principal.Role != "admin" && principal.Role != "hr" {
		selfID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
		if err != nil || selfID == "" || selfID != owner {
			writeError(w, http.StatusForbidden, "forbidden", "you may cancel only your own leave requests")
			return
		}
	}
	request, err := s.store.CancelLeaveRequestV2(r.Context(), principal.OrganizationID, r.PathValue("id"), principal.UserID, strings.TrimSpace(input.Note))
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "leave_not_cancellable", "only pending or approved leave can be cancelled")
			return
		}
		writeError(w, http.StatusInternalServerError, "leave_cancel_error", "could not cancel leave request")
		return
	}
	s.store.RecordAudit(r.Context(), principal.OrganizationID, principal.UserID, "leave.cancelled", "leave_request", request.ID,
		workflowAuditMetadata(map[string]any{"employee_id": request.EmployeeID, "days": request.Days}))
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) balanceScopeEmployeeID(w http.ResponseWriter, r *http.Request, principal model.Principal) (string, bool) {
	requested := strings.TrimSpace(r.URL.Query().Get("employee_id"))
	if principal.Role == "admin" || principal.Role == "hr" {
		if requested == "" {
			writeError(w, http.StatusBadRequest, "employee_required", "employee_id query parameter is required")
			return "", false
		}
		return requested, true
	}
	selfID, err := s.store.EmployeeIDForUser(r.Context(), principal.OrganizationID, principal.UserID)
	if err != nil || selfID == "" {
		writeError(w, http.StatusForbidden, "employee_link_required", "account is not linked to an employee profile")
		return "", false
	}
	if requested == "" || requested == selfID {
		return selfID, true
	}
	if principal.Role == "manager" {
		owns, err := s.store.ManagerOwnsEmployee(r.Context(), principal.OrganizationID, selfID, requested)
		if err == nil && owns {
			return requested, true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "you cannot view this employee's leave balance")
	return "", false
}

func parseYearQuery(w http.ResponseWriter, r *http.Request) (int, bool) {
	year := time.Now().UTC().Year()
	if raw := strings.TrimSpace(r.URL.Query().Get("year")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 2000 || parsed > 2200 {
			writeError(w, http.StatusBadRequest, "validation_error", "year must be between 2000 and 2200")
			return 0, false
		}
		year = parsed
	}
	return year, true
}
