package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func (s *Store) CalculateLeaveDays(ctx context.Context, orgID, employeeID string, start, end time.Time, partialDay string) (float64, map[int]float64, error) {
	if partialDay != "full" && !sameDate(start, end) {
		return 0, nil, fmt.Errorf("partial-day leave must start and end on the same date")
	}
	var location string
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(location, '') FROM employees
		WHERE organization_id=$1::uuid AND id=$2`, orgID, employeeID).Scan(&location); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrNotFound
		}
		return 0, nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT holiday_date
		FROM company_holidays
		WHERE organization_id=$1::uuid
		  AND holiday_date BETWEEN $2::date AND $3::date
		  AND (location IS NULL OR location='' OR lower(location)=lower($4))`,
		orgID, start.Format("2006-01-02"), end.Format("2006-01-02"), location)
	if err != nil {
		return 0, nil, err
	}
	holidays := map[string]struct{}{}
	for rows.Next() {
		var holiday time.Time
		if err := rows.Scan(&holiday); err != nil {
			rows.Close()
			return 0, nil, err
		}
		holidays[holiday.Format("2006-01-02")] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	return calculateWorkingDays(start, end, partialDay, holidays)
}

func (s *Store) CreateLeaveRequestV2(ctx context.Context, orgID, employeeID, leaveType string, start, end time.Time, partialDay, reason string) (model.LeaveWorkflowRequest, error) {
	policy, err := s.resolveLeavePolicy(ctx, orgID, employeeID, leaveType, start)
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	days, _, err := s.CalculateLeaveDays(ctx, orgID, employeeID, start, end, partialDay)
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	request, err := scanWorkflowLeaveRequest(s.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO leave_requests (
				organization_id, employee_id, leave_type, policy_id,
				start_date, end_date, days, partial_day, reason
			) VALUES ($1::uuid, $2, $3, $4::uuid, $5::date, $6::date, $7, $8, NULLIF($9, ''))
			RETURNING *
		)
		SELECT l.id::text, l.employee_id, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       l.days::float8, COALESCE(l.reason, ''), l.status,
		       COALESCE(l.approver_id, ''), '', l.decided_at,
		       COALESCE(l.decision_note, ''), l.created_at
		FROM created l
		JOIN employees e ON e.id=l.employee_id AND e.organization_id=l.organization_id`,
		orgID, employeeID, leaveType, policy.ID, start.Format("2006-01-02"),
		end.Format("2006-01-02"), days, partialDay, strings.TrimSpace(reason)))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return model.LeaveWorkflowRequest{}, ErrConflict
		}
	}
	return request, err
}

func (s *Store) DecideLeaveRequestV2(ctx context.Context, orgID, requestID, approverEmployeeID, decision, note, actorUserID string) (model.LeaveWorkflowRequest, error) {
	if decision == "approved" {
		if err := s.EnsureDefaultLeavePolicies(ctx, orgID); err != nil {
			return model.LeaveWorkflowRequest{}, err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	defer tx.Rollback(ctx)

	var employeeID, leaveType, policyID, partialDay, currentStatus string
	var start, end time.Time
	if err := tx.QueryRow(ctx, `
		SELECT employee_id, leave_type, COALESCE(policy_id::text, ''), start_date, end_date,
		       partial_day, status
		FROM leave_requests
		WHERE organization_id=$1::uuid AND id=$2::uuid
		FOR UPDATE`, orgID, requestID).Scan(
		&employeeID, &leaveType, &policyID, &start, &end, &partialDay, &currentStatus,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.LeaveWorkflowRequest{}, ErrNotFound
		}
		return model.LeaveWorkflowRequest{}, err
	}
	if currentStatus != "pending" {
		return model.LeaveWorkflowRequest{}, ErrConflict
	}

	if decision == "approved" {
		policy, err := s.policyForDecision(ctx, tx, orgID, leaveType, policyID)
		if err != nil {
			return model.LeaveWorkflowRequest{}, err
		}
		_, byYear, err := s.calculateLeaveDaysTx(ctx, tx, orgID, employeeID, start, end, partialDay)
		if err != nil {
			return model.LeaveWorkflowRequest{}, err
		}
		if policy.TrackBalance {
			for year, days := range byYear {
				if err := ensureLeaveYearTx(ctx, tx, orgID, employeeID, policy, year, actorUserID); err != nil {
					return model.LeaveWorkflowRequest{}, err
				}
				available, err := leaveAvailableTx(ctx, tx, orgID, employeeID, policy.ID, year)
				if err != nil {
					return model.LeaveWorkflowRequest{}, err
				}
				if !policy.AllowNegative && available+1e-9 < days {
					return model.LeaveWorkflowRequest{}, ErrInsufficientLeaveBalance
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO leave_balance_ledger (
						organization_id, employee_id, policy_id, leave_request_id,
						balance_year, amount, event_type, source_key, note, created_by
					) VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6, 'approved_leave', $7, 'Approved leave', NULLIF($8, '')::uuid)
					ON CONFLICT (organization_id, employee_id, policy_id, source_key) DO NOTHING`,
					orgID, employeeID, policy.ID, requestID, year, -days,
					fmt.Sprintf("request:%s:debit:%d", requestID, year), actorUserID); err != nil {
					return model.LeaveWorkflowRequest{}, err
				}
			}
		}
	}

	request, err := scanWorkflowLeaveRequest(tx.QueryRow(ctx, `
		WITH decided AS (
			UPDATE leave_requests
			SET status=$3, approver_id=NULLIF($4, ''), decided_at=now(),
			    decision_note=NULLIF($5, ''), updated_at=now()
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
		orgID, requestID, decision, approverEmployeeID, strings.TrimSpace(note)))
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	return request, nil
}

func (s *Store) CancelLeaveRequestV2(ctx context.Context, orgID, requestID, actorUserID, note string) (model.LeaveWorkflowRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM leave_requests
		WHERE organization_id=$1::uuid AND id=$2::uuid FOR UPDATE`, orgID, requestID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.LeaveWorkflowRequest{}, ErrNotFound
		}
		return model.LeaveWorkflowRequest{}, err
	}
	if currentStatus != "pending" && currentStatus != "approved" {
		return model.LeaveWorkflowRequest{}, ErrConflict
	}
	if currentStatus == "approved" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO leave_balance_ledger (
				organization_id, employee_id, policy_id, leave_request_id,
				balance_year, amount, event_type, source_key, note, created_by
			)
			SELECT organization_id, employee_id, policy_id, leave_request_id,
			       balance_year, -amount, 'cancellation',
			       'request:' || $2 || ':cancel:' || balance_year::text,
			       NULLIF($3, ''), NULLIF($4, '')::uuid
			FROM leave_balance_ledger
			WHERE organization_id=$1::uuid AND leave_request_id=$2::uuid
			  AND event_type='approved_leave'
			ON CONFLICT (organization_id, employee_id, policy_id, source_key) DO NOTHING`,
			orgID, requestID, strings.TrimSpace(note), actorUserID); err != nil {
			return model.LeaveWorkflowRequest{}, err
		}
	}
	request, err := scanWorkflowLeaveRequest(tx.QueryRow(ctx, `
		WITH cancelled AS (
			UPDATE leave_requests
			SET status='cancelled', cancelled_at=now(), cancellation_note=NULLIF($3, ''), updated_at=now()
			WHERE organization_id=$1::uuid AND id=$2::uuid
			RETURNING *
		)
		SELECT l.id::text, l.employee_id, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       l.days::float8, COALESCE(l.reason, ''), l.status,
		       COALESCE(l.approver_id, ''), COALESCE(trim(a.first_name || ' ' || a.last_name), ''),
		       l.decided_at, COALESCE(l.decision_note, ''), l.created_at
		FROM cancelled l
		JOIN employees e ON e.id=l.employee_id AND e.organization_id=l.organization_id
		LEFT JOIN employees a ON a.id=l.approver_id AND a.organization_id=l.organization_id`,
		orgID, requestID, strings.TrimSpace(note)))
	if err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.LeaveWorkflowRequest{}, err
	}
	return request, nil
}

func (s *Store) policyForDecision(ctx context.Context, tx pgx.Tx, orgID, leaveType, policyID string) (model.LeavePolicy, error) {
	if policyID != "" {
		return scanLeavePolicy(tx.QueryRow(ctx, `
			SELECT id::text, code, name, leave_type, annual_entitlement::float8,
			       carry_over_limit::float8, track_balance, allow_negative,
			       requires_approval, active, created_at, updated_at
			FROM leave_policies WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, policyID))
	}
	return scanLeavePolicy(tx.QueryRow(ctx, `
		SELECT id::text, code, name, leave_type, annual_entitlement::float8,
		       carry_over_limit::float8, track_balance, allow_negative,
		       requires_approval, active, created_at, updated_at
		FROM leave_policies
		WHERE organization_id=$1::uuid AND leave_type=$2 AND active=true
		LIMIT 1`, orgID, leaveType))
}

func (s *Store) calculateLeaveDaysTx(ctx context.Context, tx pgx.Tx, orgID, employeeID string, start, end time.Time, partialDay string) (float64, map[int]float64, error) {
	if partialDay != "full" && !sameDate(start, end) {
		return 0, nil, fmt.Errorf("partial-day leave must start and end on the same date")
	}
	var location string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(location,'') FROM employees WHERE organization_id=$1::uuid AND id=$2`, orgID, employeeID).Scan(&location); err != nil {
		return 0, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT holiday_date FROM company_holidays
		WHERE organization_id=$1::uuid AND holiday_date BETWEEN $2::date AND $3::date
		  AND (location IS NULL OR location='' OR lower(location)=lower($4))`,
		orgID, start.Format("2006-01-02"), end.Format("2006-01-02"), location)
	if err != nil {
		return 0, nil, err
	}
	holidays := map[string]struct{}{}
	for rows.Next() {
		var holiday time.Time
		if err := rows.Scan(&holiday); err != nil {
			rows.Close()
			return 0, nil, err
		}
		holidays[holiday.Format("2006-01-02")] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	return calculateWorkingDays(start, end, partialDay, holidays)
}

func calculateWorkingDays(start, end time.Time, partialDay string, holidays map[string]struct{}) (float64, map[int]float64, error) {
	byYear := map[int]float64{}
	for day := dateOnly(start); !day.After(dateOnly(end)); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		if _, holiday := holidays[day.Format("2006-01-02")]; holiday {
			continue
		}
		amount := 1.0
		if partialDay != "full" {
			amount = 0.5
		}
		byYear[day.Year()] += amount
	}
	var total float64
	for _, amount := range byYear {
		total += amount
	}
	if total == 0 {
		return 0, nil, ErrNoWorkingDays
	}
	return total, byYear, nil
}

func sameDate(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func dateOnly(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
