package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func (s *Store) EmployeeIDForUser(ctx context.Context, orgID, userID string) (string, error) {
	var employeeID string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(employee_id, '')
		FROM organization_memberships
		WHERE organization_id=$1::uuid AND user_id=$2::uuid AND status='active'`, orgID, userID).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return employeeID, err
}

func (s *Store) ListLeaveRequestsScoped(ctx context.Context, orgID, scopeEmployeeID string, includeReports bool) ([]model.LeaveRequest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id::text, l.employee_id, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       l.days::float8, COALESCE(l.reason, ''), l.status,
		       COALESCE(l.approver_id, ''), COALESCE(trim(a.first_name || ' ' || a.last_name), ''),
		       l.decided_at, COALESCE(l.decision_note, ''), l.created_at
		FROM leave_requests l
		JOIN employees e ON e.id=l.employee_id AND e.organization_id=l.organization_id
		LEFT JOIN employees a ON a.id=l.approver_id AND a.organization_id=l.organization_id
		WHERE l.organization_id=$1::uuid
		  AND ($2='' OR e.id=$2 OR ($3::boolean AND e.manager_id=$2))
		ORDER BY CASE WHEN l.status='pending' THEN 0 ELSE 1 END, l.created_at DESC`, orgID, scopeEmployeeID, includeReports)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []model.LeaveRequest
	for rows.Next() {
		request, err := scanWorkflowLeaveRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *Store) CreateLeaveRequest(ctx context.Context, orgID, employeeID, leaveType string, start, end time.Time, days float64, reason string) (model.LeaveRequest, error) {
	return scanWorkflowLeaveRequest(s.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO leave_requests (organization_id, employee_id, leave_type, start_date, end_date, days, reason)
			VALUES ($1::uuid, $2, $3, $4::date, $5::date, $6, NULLIF($7, ''))
			RETURNING *
		)
		SELECT l.id::text, l.employee_id, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       l.days::float8, COALESCE(l.reason, ''), l.status,
		       COALESCE(l.approver_id, ''), '', l.decided_at, COALESCE(l.decision_note, ''), l.created_at
		FROM created l
		JOIN employees e ON e.id=l.employee_id AND e.organization_id=l.organization_id`,
		orgID, employeeID, leaveType, start.Format("2006-01-02"), end.Format("2006-01-02"), days, reason))
}

func (s *Store) DecideLeaveRequest(ctx context.Context, orgID, requestID, approverEmployeeID, decision, note string) (model.LeaveRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.LeaveRequest{}, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM leave_requests WHERE organization_id=$1::uuid AND id=$2::uuid FOR UPDATE`, orgID, requestID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.LeaveRequest{}, ErrNotFound
		}
		return model.LeaveRequest{}, err
	}
	if currentStatus != "pending" {
		return model.LeaveRequest{}, ErrConflict
	}

	request, err := scanWorkflowLeaveRequest(tx.QueryRow(ctx, `
		WITH decided AS (
			UPDATE leave_requests
			SET status=$3, approver_id=NULLIF($4, ''), decided_at=now(), decision_note=NULLIF($5, ''), updated_at=now()
			WHERE organization_id=$1::uuid AND id=$2::uuid
			RETURNING *
		)
		SELECT l.id::text, l.employee_id, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       l.days::float8, COALESCE(l.reason, ''), l.status,
		       COALESCE(l.approver_id, ''), COALESCE(trim(a.first_name || ' ' || a.last_name), ''),
		       l.decided_at, COALESCE(l.decision_note, ''), l.created_at
		FROM decided l
		JOIN employees e ON e.id=l.employee_id AND e.organization_id=l.organization_id
		LEFT JOIN employees a ON a.id=l.approver_id AND a.organization_id=l.organization_id`,
		orgID, requestID, decision, approverEmployeeID, note))
	if err != nil {
		return model.LeaveRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.LeaveRequest{}, err
	}
	return request, nil
}

func (s *Store) ManagerOwnsLeaveRequest(ctx context.Context, orgID, managerEmployeeID, requestID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM leave_requests l
			JOIN employees e ON e.organization_id=l.organization_id AND e.id=l.employee_id
			WHERE l.organization_id=$1::uuid AND l.id=$2::uuid AND e.manager_id=$3
		)`, orgID, requestID, managerEmployeeID).Scan(&ok)
	return ok, err
}

func (s *Store) ListAttendance(ctx context.Context, orgID string, workDate time.Time, scopeEmployeeID string, includeReports bool) ([]model.AttendanceEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date, 'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status
		FROM attendance_entries a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		WHERE a.organization_id=$1::uuid AND a.work_date=$2::date
		  AND ($3='' OR e.id=$3 OR ($4::boolean AND e.manager_id=$3))
		ORDER BY e.first_name, e.last_name`, orgID, workDate.Format("2006-01-02"), scopeEmployeeID, includeReports)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.AttendanceEntry
	for rows.Next() {
		entry, err := scanAttendanceEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) CheckIn(ctx context.Context, orgID, employeeID, workMode string) (model.AttendanceEntry, error) {
	return scanAttendanceEntry(s.pool.QueryRow(ctx, `
		WITH changed AS (
			INSERT INTO attendance_entries (organization_id, employee_id, work_date, check_in, work_mode, status)
			VALUES ($1::uuid, $2, current_date, now(), $3, 'present')
			ON CONFLICT (organization_id, employee_id, work_date) DO UPDATE SET
				check_in=COALESCE(attendance_entries.check_in, EXCLUDED.check_in),
				work_mode=EXCLUDED.work_mode, status='present', updated_at=now()
			RETURNING *
		)
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date, 'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status
		FROM changed a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id`, orgID, employeeID, workMode))
}

func (s *Store) CheckOut(ctx context.Context, orgID, employeeID string) (model.AttendanceEntry, error) {
	entry, err := scanAttendanceEntry(s.pool.QueryRow(ctx, `
		WITH changed AS (
			UPDATE attendance_entries SET check_out=COALESCE(check_out, now()), updated_at=now()
			WHERE organization_id=$1::uuid AND employee_id=$2 AND work_date=current_date AND check_in IS NOT NULL
			RETURNING *
		)
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date, 'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status
		FROM changed a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id`, orgID, employeeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AttendanceEntry{}, ErrNotFound
	}
	return entry, err
}

func (s *Store) AuditEvents(ctx context.Context, orgID string, limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id::text, COALESCE(a.actor_user_id::text, ''), COALESCE(u.display_name, ''),
		       a.action, a.resource_type, COALESCE(a.resource_id, ''), a.metadata::text, a.created_at
		FROM audit_events a
		LEFT JOIN users u ON u.id=a.actor_user_id
		WHERE a.organization_id=$1::uuid
		ORDER BY a.created_at DESC
		LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.AuditEvent
	for rows.Next() {
		var event model.AuditEvent
		var metadata string
		if err := rows.Scan(&event.ID, &event.ActorUserID, &event.Actor, &event.Action, &event.ResourceType, &event.ResourceID, &metadata, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Metadata = map[string]any{}
		_ = json.Unmarshal([]byte(metadata), &event.Metadata)
		events = append(events, event)
	}
	return events, rows.Err()
}

func scanWorkflowLeaveRequest(row rowScanner) (model.LeaveRequest, error) {
	var request model.LeaveRequest
	err := row.Scan(
		&request.ID, &request.EmployeeID, &request.Employee, &request.Type, &request.Start, &request.End,
		&request.Days, &request.Reason, &request.Status, &request.ApproverID, &request.Approver,
		&request.DecidedAt, &request.DecisionNote, &request.CreatedAt,
	)
	return request, err
}

func scanAttendanceEntry(row rowScanner) (model.AttendanceEntry, error) {
	var entry model.AttendanceEntry
	err := row.Scan(&entry.ID, &entry.EmployeeID, &entry.Employee, &entry.WorkDate, &entry.CheckIn, &entry.CheckOut, &entry.WorkMode, &entry.Status)
	return entry, err
}
