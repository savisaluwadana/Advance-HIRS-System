package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
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
		policy, err := scanLeavePolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (s *Store) CreateOrUpdateLeavePolicy(ctx context.Context, orgID string, input model.CreateLeavePolicy) (model.LeavePolicy, error) {
	return scanLeavePolicy(s.pool.QueryRow(ctx, `
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
		input.CarryOverLimit, input.TrackBalance, input.AllowNegative, input.RequiresApproval))
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
		if err := s.pool.QueryRow(ctx, `
			SELECT
			  COALESCE(sum(amount) FILTER (WHERE event_type='entitlement'), 0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE event_type='carry_over'), 0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE event_type='adjustment'), 0)::float8,
			  COALESCE(-sum(amount) FILTER (WHERE event_type IN ('approved_leave','cancellation')), 0)::float8,
			  COALESCE(sum(amount), 0)::float8
			FROM leave_balance_ledger
			WHERE organization_id=$1::uuid AND employee_id=$2 AND policy_id=$3::uuid AND balance_year=$4`,
			orgID, employeeID, policy.ID, year).Scan(&entitlement, &carryOver, &adjustments, &consumed, &available); err != nil {
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
		input.Note, actorUserID).Scan(
		&event.ID, &event.EmployeeID, &event.PolicyID, &event.LeaveRequestID,
		&event.Year, &event.Amount, &event.EventType, &event.Note, &event.CreatedAt,
	)
	return event, err
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

func scanLeavePolicy(row rowScanner) (model.LeavePolicy, error) {
	var policy model.LeavePolicy
	err := row.Scan(&policy.ID, &policy.Code, &policy.Name, &policy.LeaveType,
		&policy.AnnualEntitlement, &policy.CarryOverLimit, &policy.TrackBalance,
		&policy.AllowNegative, &policy.RequiresApproval, &policy.Active,
		&policy.CreatedAt, &policy.UpdatedAt)
	return policy, err
}
