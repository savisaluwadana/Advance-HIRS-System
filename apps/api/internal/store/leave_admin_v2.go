package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func (s *Store) AssignLeavePolicy(ctx context.Context, orgID, policyID string, input model.AssignLeavePolicy) error {
	from := time.Now().UTC()
	if input.EffectiveFrom != "" {
		parsed, err := time.Parse("2006-01-02", input.EffectiveFrom)
		if err != nil {
			return err
		}
		from = parsed
	}
	var to any
	if input.EffectiveTo != "" {
		parsed, err := time.Parse("2006-01-02", input.EffectiveTo)
		if err != nil {
			return err
		}
		if parsed.Before(from) {
			return errors.New("effective_to must be on or after effective_from")
		}
		to = parsed.Format("2006-01-02")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO leave_policy_assignments (
			organization_id, employee_id, policy_id, effective_from, effective_to
		) VALUES ($1::uuid, $2, $3::uuid, $4::date, $5::date)
		ON CONFLICT (organization_id, employee_id, policy_id, effective_from)
		DO UPDATE SET effective_to=EXCLUDED.effective_to`,
		orgID, input.EmployeeID, policyID, from.Format("2006-01-02"), to)
	return err
}

func (s *Store) LeaveRequestOwner(ctx context.Context, orgID, requestID string) (string, error) {
	var employeeID string
	err := s.pool.QueryRow(ctx, `
		SELECT employee_id FROM leave_requests
		WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, requestID).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return employeeID, err
}

func (s *Store) ManagerOwnsEmployee(ctx context.Context, orgID, managerEmployeeID, employeeID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM employees
			WHERE organization_id=$1::uuid AND id=$2 AND manager_id=$3
		)`, orgID, employeeID, managerEmployeeID).Scan(&ok)
	return ok, err
}

func (s *Store) LeaveBalanceLedger(ctx context.Context, orgID, employeeID, policyID string, year int) ([]model.LeaveBalanceEvent, error) {
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, employee_id, policy_id::text, COALESCE(leave_request_id::text,''),
		       balance_year, amount::float8, event_type, COALESCE(note,''), created_at
		FROM leave_balance_ledger
		WHERE organization_id=$1::uuid AND employee_id=$2 AND balance_year=$3
		  AND ($4='' OR policy_id=$4::uuid)
		ORDER BY created_at DESC`, orgID, employeeID, year, policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []model.LeaveBalanceEvent
	for rows.Next() {
		var event model.LeaveBalanceEvent
		if err := rows.Scan(&event.ID, &event.EmployeeID, &event.PolicyID, &event.LeaveRequestID,
			&event.Year, &event.Amount, &event.EventType, &event.Note, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
