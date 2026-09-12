package store

import (
	"context"
	"fmt"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
)

// SeedDemoAccess provisions local/demo-only manager and employee accounts and
// a simple reporting line. Production environments should leave
// SEED_DEMO_PASSWORD unset and provision users through the real access flow.
func (s *Store) SeedDemoAccess(ctx context.Context, orgID, password string) error {
	if password == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE employees SET manager_id='emp_003', updated_at=now()
		WHERE organization_id=$1::uuid AND id IN ('emp_001','emp_002')`, orgID); err != nil {
		return err
	}

	accounts := []struct {
		email      string
		display    string
		role       string
		employeeID string
	}{
		{email: "sara@northstar.local", display: "Sara Ibrahim", role: "manager", employeeID: "emp_003"},
		{email: "ava@northstar.local", display: "Ava Morgan", role: "employee", employeeID: "emp_001"},
	}

	for _, account := range accounts {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash demo password: %w", err)
		}
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, display_name)
			VALUES (lower($1), $2, $3)
			ON CONFLICT (email) DO UPDATE SET password_hash=EXCLUDED.password_hash,
				display_name=EXCLUDED.display_name, status='active', updated_at=now()
			RETURNING id::text`, account.email, hash, account.display).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_memberships (organization_id, user_id, employee_id, role)
			VALUES ($1::uuid, $2::uuid, $3, $4)
			ON CONFLICT (organization_id, user_id) DO UPDATE SET
				employee_id=EXCLUDED.employee_id, role=EXCLUDED.role, status='active', updated_at=now()`,
			orgID, userID, account.employeeID, account.role); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
