package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

var (
	ErrInsufficientLeaveBalance = errors.New("insufficient leave balance")
	ErrNoWorkingDays            = errors.New("leave request contains no working days")
)

type defaultLeavePolicy struct {
	code, name, leaveType       string
	entitlement, carryOverLimit float64
	trackBalance                bool
}

var defaultLeavePolicies = []defaultLeavePolicy{
	{code: "annual", name: "Annual leave", leaveType: "annual", entitlement: 20, carryOverLimit: 5, trackBalance: true},
	{code: "sick", name: "Sick leave", leaveType: "sick", entitlement: 10, carryOverLimit: 0, trackBalance: true},
	{code: "parental", name: "Parental leave", leaveType: "parental", entitlement: 0, carryOverLimit: 0, trackBalance: false},
	{code: "unpaid", name: "Unpaid leave", leaveType: "unpaid", entitlement: 0, carryOverLimit: 0, trackBalance: false},
	{code: "remote", name: "Remote work", leaveType: "remote", entitlement: 0, carryOverLimit: 0, trackBalance: false},
	{code: "other", name: "Other leave", leaveType: "other", entitlement: 0, carryOverLimit: 0, trackBalance: false},
}

func (s *Store) EnsureDefaultLeavePolicies(ctx context.Context, orgID string) error {
	for _, policy := range defaultLeavePolicies {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO leave_policies (
				organization_id, code, name, leave_type, annual_entitlement,
				carry_over_limit, track_balance, allow_negative, requires_approval
			) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, false, true)
			ON CONFLICT (organization_id, leave_type) DO NOTHING`,
			orgID, policy.code, policy.name, policy.leaveType, policy.entitlement,
			policy.carryOverLimit, policy.trackBalance); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListLeavePolicies(ctx context.Context, orgID string) ([]model.LeavePolicy, error) {
	if err := s.EnsureDefaultLeavePolicies(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, code, name, leave_type, annual_entitlement::float8,
		       carry_over_limit::float8, track_balance, allow_negative,
		       requires_approval, active, created_at, updated_at
		FROM leave_policies
		WHERE organization_id=$1::uuid
		ORDER BY active DESC, name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var policies []model.LeavePolicy
	for rows.Next() {
		var policy model.LeavePolicy
		if err := rows.Scan(&policy.ID, &policy.Code, &policy.Name, &policy.LeaveType,
			&policy.AnnualEntitlement, &policy.CarryOverLimit, &policy.TrackBalance,
			&policy.AllowNegative, &policy.RequiresApproval, &policy.Active,
			&policy.CreatedAt, &policy.UpdatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (s *Store) CreateOrUpdateLeavePolicy(ctx context.Context, orgID string, input model.CreateLeavePolicy) (model.LeavePolicy, error) {
	var policy model.LeavePolicy
	err := s.pool.QueryRow(ctx, `
		INSERT INTO leave_policies (
			organization_id, code, name, leave_type, annual_entitlement,
			carry_over_limit, track_balance, allow_negative, requires_approval
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organization_id, leave_type) DO UPDATE SET
			code=EXCLUDED.code, name=EXCLUDED.name,
			annual_entitlement=EXCLUDED.annual_entitlement,
			carry_over_limit=EXCLUDED.carry_over_limit,
			track_balance=EXCLUDED.track_balance,
			allow_negative=EXCLUDED.allow_negative,
			requires_approval=EXCLUDED.requires_approval,
			active=true, updated_at=now()
		RETURNING id::text, code, name, leave_type, annual_entitlement::float8,
		          carry_over_limit::float8, track_balance, allow_negative,
		          requires_approval, active, created_at, updated_at`,
		orgID, input.Code, input.Name, input.LeaveType, input.AnnualEntitlement,
		input.CarryOverLimit, input.TrackBalance, input.AllowNegative, input.RequiresApproval).Scan(
		&policy.ID, &policy.Code, &policy.Name, &policy.LeaveType,
		&policy.AnnualEntitlement, &policy.CarryOverLimit, &policy.TrackBalance,
		&policy.AllowNegative, &policy.RequiresApproval, &policy.Active,
		&policy.CreatedAt, &policy.UpdatedAt,
	)
	return policy, err
}

func (s *Store) resolveLeavePolicy(ctx context.Context, orgID, employeeID, leaveType string, onDate time.Time) (model.LeavePolicy, error) {
	if err := s.EnsureDefaultLeavePolicies(ctx, orgID); err != nil {
		return model.LeavePolicy{}, err
	}
	return scanLeavePolicy(s.pool.QueryRow(ctx, `
		SELECT p.id::text, p.code, p.name, p.leave_type, p.annual_entitlement::float8,
		       p.carry_over_limit::float8, p.track_balance, p.allow_negative,
		       p.requires_approval, p.active, p.created_at, p.updated_at
		FROM leave_policies p
		LEFT JOIN leave_policy_assignments a
		  ON a.policy_id=p.id AND a.organization_id=p.organization_id
		 AND a.employee_id=$2
		 AND a.effective_from <= $4::date
		 AND (a.effective_to IS NULL OR a.effective_to >= $4::date)
		WHERE p.organization_id=$1::uuid AND p.leave_type=$3 AND p.active=true
		ORDER BY CASE WHEN a.id IS NULL THEN 1 ELSE 0 END, a.effective_from DESC NULLS LAST
		LIMIT 1`, orgID, employeeID, leaveType, onDate.Format("2006-01-02")))
}

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
	if err := rows.Close(); err != nil {
		return 0, nil, err
	}

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
	for _, days := range byYear {
		total += days
	}
	if total == 0 {
		return 0, nil, ErrNoWorkingDays
	}
	return total, byYear, nil
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
		policy, err := s.policyForDecision(ctx, tx, orgID, employeeID, leaveType, policyID, start)
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
					) VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6, 'approved_leave', $7, $8, NULLIF($9, '')::uuid)
					ON CONFLICT (organization_id, employee_id, policy_id, source_key) DO NOTHING`,
					orgID, employeeID, policy.ID, requestID, year, -days,
					fmt.Sprintf("request:%s:debit:%d", requestID, year), "Approved leave", actorUserID); err != nil {
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

	var currentStatus, employeeID string
	if err := tx.QueryRow(ctx, `
		SELECT status, employee_id FROM leave_requests
		WHERE organization_id=$1::uuid AND id=$2::uuid FOR UPDATE`, orgID, requestID).Scan(&currentStatus, &employeeID); err != nil {
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

func (s *Store) ListLeaveBalances(ctx context.Context, orgID, employeeID string, year int) ([]model.LeaveBalance, error) {
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	policies, err := s.ListLeavePolicies(ctx, orgID)
	if err != nil {
		return nil, err
	}
	balances := make([]model.LeaveBalance, 0, len(policies))
	for _, policy := range policies {
		if !policy.Active {
			continue
		}
		if policy.TrackBalance {
			if err := s.ensureLeaveYear(ctx, orgID, employeeID, policy, year, ""); err != nil {
				return nil, err
			}
		}
		var entitlement, carryOver, adjustments, consumed, available float64
		err := s.pool.QueryRow(ctx, `
			SELECT
			  COALESCE(sum(amount) FILTER (WHERE event_type='entitlement'), 0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE event_type='carry_over'), 0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE event_type='adjustment'), 0)::float8,
			  COALESCE(-sum(amount) FILTER (WHERE event_type IN ('approved_leave','cancellation')), 0)::float8,
			  COALESCE(sum(amount), 0)::float8
			FROM leave_balance_ledger
			WHERE organization_id=$1::uuid AND employee_id=$2 AND policy_id=$3::uuid AND balance_year=$4`,
			orgID, employeeID, policy.ID, year).Scan(&entitlement, &carryOver, &adjustments, &consumed, &available)
		if err != nil {
			return nil, err
		}
		balances = append(balances, model.LeaveBalance{
			EmployeeID: employeeID, PolicyID: policy.ID, PolicyCode: policy.Code, PolicyName: policy.Name,
			LeaveType: policy.LeaveType, Year: year, Entitlement: entitlement, CarryOver: carryOver,
			Adjustments: adjustments, Used: math.Max(0, consumed), Available: available,
			TrackBalance: policy.TrackBalance, AllowNegative: policy.AllowNegative,
		})
	}
	return balances, nil
}

func (s *Store) AdjustLeaveBalance(ctx context.Context, orgID, actorUserID string, input model.LeaveAdjustment) (model.LeaveBalanceEvent, error) {
	if input.Year == 0 {
		input.Year = time.Now().UTC().Year()
	}
	var policy model.LeavePolicy
	policy, err := scanLeavePolicy(s.pool.QueryRow(ctx, `
		SELECT id::text, code, name, leave_type, annual_entitlement::float8,
		       carry_over_limit::float8, track_balance, allow_negative,
		       requires_approval, active, created_at, updated_at
		FROM leave_policies WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, input.PolicyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.LeaveBalanceEvent{}, ErrNotFound
		}
		return model.LeaveBalanceEvent{}, err
	}
	if err := s.ensureLeaveYear(ctx, orgID, input.EmployeeID, policy, input.Year, actorUserID); err != nil {
		return model.LeaveBalanceEvent{}, err
	}
	var event model.LeaveBalanceEvent
	err = s.pool.QueryRow(ctx, `
		INSERT INTO leave_balance_ledger (
			organization_id, employee_id, policy_id, balance_year, amount,
			event_type, source_key, note, created_by
		) VALUES ($1::uuid, $2, $3::uuid, $4, $5, 'adjustment', gen_random_uuid()::text, NULLIF($6,''), $7::uuid)
		RETURNING id::text, employee_id, policy_id::text, '', balance_year,
		          amount::float8, event_type, COALESCE(note,''), created_at`,
		orgID, input.EmployeeID, input.PolicyID, input.Year, input.Amount,
		strings.TrimSpace(input.Note), actorUserID).Scan(
		&event.ID, &event.EmployeeID, &event.PolicyID, &event.LeaveRequestID,
		&event.Year, &event.Amount, &event.EventType, &event.Note, &event.CreatedAt,
	)
	return event, err
}

func (s *Store) ListHolidays(ctx context.Context, orgID string, year int) ([]model.CompanyHoliday, error) {
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, to_char(holiday_date,'YYYY-MM-DD'), name, COALESCE(location,''), created_at
		FROM company_holidays
		WHERE organization_id=$1::uuid AND extract(year from holiday_date)=$2
		ORDER BY holiday_date, name`, orgID, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var holidays []model.CompanyHoliday
	for rows.Next() {
		var holiday model.CompanyHoliday
		if err := rows.Scan(&holiday.ID, &holiday.Date, &holiday.Name, &holiday.Location, &holiday.CreatedAt); err != nil {
			return nil, err
		}
		holidays = append(holidays, holiday)
	}
	return holidays, rows.Err()
}

func (s *Store) CreateHoliday(ctx context.Context, orgID string, input model.CreateHoliday) (model.CompanyHoliday, error) {
	var holiday model.CompanyHoliday
	err := s.pool.QueryRow(ctx, `
		INSERT INTO company_holidays (organization_id, holiday_date, name, location)
		VALUES ($1::uuid, $2::date, $3, NULLIF($4,''))
		RETURNING id::text, to_char(holiday_date,'YYYY-MM-DD'), name, COALESCE(location,''), created_at`,
		orgID, input.Date, strings.TrimSpace(input.Name), strings.TrimSpace(input.Location)).Scan(
		&holiday.ID, &holiday.Date, &holiday.Name, &holiday.Location, &holiday.CreatedAt)
	return holiday, err
}

func (s *Store) DeleteHoliday(ctx context.Context, orgID, holidayID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM company_holidays WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, holidayID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ensureLeaveYear(ctx context.Context, orgID, employeeID string, policy model.LeavePolicy, year int, actorUserID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := ensureLeaveYearTx(ctx, tx, orgID, employeeID, policy, year, actorUserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ensureLeaveYearTx(ctx context.Context, tx pgx.Tx, orgID, employeeID string, policy model.LeavePolicy, year int, actorUserID string) error {
	if !policy.TrackBalance {
		return nil
	}
	if policy.AnnualEntitlement > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO leave_balance_ledger (
				organization_id, employee_id, policy_id, balance_year, amount,
				event_type, source_key, note, created_by
			) VALUES ($1::uuid, $2, $3::uuid, $4, $5, 'entitlement', $6, 'Annual entitlement', NULLIF($7,'')::uuid)
			ON CONFLICT (organization_id, employee_id, policy_id, source_key) DO NOTHING`,
			orgID, employeeID, policy.ID, year, policy.AnnualEntitlement,
			fmt.Sprintf("year:%d:entitlement", year), actorUserID); err != nil {
			return err
		}
	}
	if policy.CarryOverLimit > 0 {
		var previous float64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(sum(amount),0)::float8
			FROM leave_balance_ledger
			WHERE organization_id=$1::uuid AND employee_id=$2 AND policy_id=$3::uuid AND balance_year=$4`,
			orgID, employeeID, policy.ID, year-1).Scan(&previous); err != nil {
			return err
		}
		carry := math.Min(math.Max(previous, 0), policy.CarryOverLimit)
		if carry > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO leave_balance_ledger (
					organization_id, employee_id, policy_id, balance_year, amount,
					event_type, source_key, note, created_by
				) VALUES ($1::uuid, $2, $3::uuid, $4, $5, 'carry_over', $6, 'Carry over from previous year', NULLIF($7,'')::uuid)
				ON CONFLICT (organization_id, employee_id, policy_id, source_key) DO NOTHING`,
				orgID, employeeID, policy.ID, year, carry,
				fmt.Sprintf("year:%d:carryover", year), actorUserID); err != nil {
				return err
			}
		}
	}
	return nil
}

func leaveAvailableTx(ctx context.Context, tx pgx.Tx, orgID, employeeID, policyID string, year int) (float64, error) {
	var available float64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(amount),0)::float8 FROM leave_balance_ledger
		WHERE organization_id=$1::uuid AND employee_id=$2 AND policy_id=$3::uuid AND balance_year=$4`,
		orgID, employeeID, policyID, year).Scan(&available)
	return available, err
}

func (s *Store) policyForDecision(ctx context.Context, tx pgx.Tx, orgID, employeeID, leaveType, policyID string, onDate time.Time) (model.LeavePolicy, error) {
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
	if err := rows.Close(); err != nil {
		return 0, nil, err
	}
	byYear := map[int]float64{}
	for day := dateOnly(start); !day.After(dateOnly(end)); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		if _, ok := holidays[day.Format("2006-01-02")]; ok {
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

func scanLeavePolicy(row rowScanner) (model.LeavePolicy, error) {
	var policy model.LeavePolicy
	err := row.Scan(&policy.ID, &policy.Code, &policy.Name, &policy.LeaveType,
		&policy.AnnualEntitlement, &policy.CarryOverLimit, &policy.TrackBalance,
		&policy.AllowNegative, &policy.RequiresApproval, &policy.Active,
		&policy.CreatedAt, &policy.UpdatedAt)
	return policy, err
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
