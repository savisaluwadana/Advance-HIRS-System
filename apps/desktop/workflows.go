package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/domain"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/secure"
)

func (a *App) GetOperationsState() (domain.OperationsState, error) {
	if !a.authenticated {
		return domain.OperationsState{}, fmt.Errorf("sign in is required")
	}
	var leaveResponse struct {
		Data []domain.LeaveRequest `json:"data"`
	}
	if err := a.authorizedJSON(http.MethodGet, "/api/v1/leave/requests", nil, &leaveResponse); err != nil {
		return domain.OperationsState{}, err
	}
	var attendanceResponse struct {
		Data []domain.AttendanceEntry `json:"data"`
	}
	if err := a.authorizedJSON(http.MethodGet, "/api/v1/attendance", nil, &attendanceResponse); err != nil {
		return domain.OperationsState{}, err
	}
	state := domain.OperationsState{LeaveRequests: leaveResponse.Data, Attendance: attendanceResponse.Data}
	if a.user.Role == "admin" || a.user.Role == "hr" {
		var auditResponse struct {
			Data []domain.AuditEvent `json:"data"`
		}
		if err := a.authorizedJSON(http.MethodGet, "/api/v1/audit?limit=12", nil, &auditResponse); err == nil {
			state.Audit = auditResponse.Data
		}
	}
	return state, nil
}

func (a *App) DecideLeaveRequest(id, decision, note string) (domain.LeaveRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.LeaveRequest{}, fmt.Errorf("leave request id is required")
	}
	payload := map[string]string{"decision": decision, "note": note}
	var response domain.LeaveRequest
	path := "/api/v1/leave/requests/" + url.PathEscape(id) + "/decision"
	if err := a.authorizedJSON(http.MethodPost, path, payload, &response); err != nil {
		return domain.LeaveRequest{}, err
	}
	return response, nil
}

func (a *App) CheckInEmployee(employeeID, workMode string) (domain.AttendanceEntry, error) {
	payload := map[string]string{"employee_id": employeeID, "work_mode": workMode}
	var response domain.AttendanceEntry
	if err := a.authorizedJSON(http.MethodPost, "/api/v1/attendance/check-in", payload, &response); err != nil {
		return domain.AttendanceEntry{}, err
	}
	return response, nil
}

func (a *App) CheckOutEmployee(employeeID string) (domain.AttendanceEntry, error) {
	payload := map[string]string{"employee_id": employeeID}
	var response domain.AttendanceEntry
	if err := a.authorizedJSON(http.MethodPost, "/api/v1/attendance/check-out", payload, &response); err != nil {
		return domain.AttendanceEntry{}, err
	}
	return response, nil
}

func (a *App) authorizedJSON(method, path string, body any, output any) error {
	token, err := secure.LoadAccessToken()
	if err != nil {
		return fmt.Errorf("load desktop session: %w", err)
	}
	if token == "" {
		return fmt.Errorf("sign in is required")
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(a.ctx, method, a.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		a.offline = true
		return fmt.Errorf("cloud request failed: %w", err)
	}
	defer resp.Body.Close()
	a.offline = false
	if resp.StatusCode == http.StatusUnauthorized {
		_ = secure.ClearAccessToken()
		a.authenticated = false
		a.user = domain.AuthUser{}
		return fmt.Errorf("session expired; sign in again")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		message := failure.Error.Message
		if message == "" {
			message = "cloud returned status " + strconv.Itoa(resp.StatusCode)
		}
		return fmt.Errorf("%s", message)
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
