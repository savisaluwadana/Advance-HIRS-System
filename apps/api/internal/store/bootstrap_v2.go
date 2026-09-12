package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

// BootstrapV2 preserves the existing organization/admin bootstrap while using
// tenant-scoped employee IDs for demo data. Internal employee IDs only need to
// be unique inside an organization because every employee query is tenant scoped.
func (s *Store) BootstrapV2(ctx context.Context, orgName, orgSlug, adminName, adminEmail, adminPassword string, seedDemo bool) (string, error) {
	orgID, err := s.Bootstrap(ctx, orgName, orgSlug, adminName, adminEmail, adminPassword, false)
	if err != nil {
		return "", err
	}
	if seedDemo {
		if err := s.seedDemoEmployeesTenantSafe(ctx, orgID); err != nil {
			return "", err
		}
	}
	return orgID, nil
}

func (s *Store) seedDemoEmployeesTenantSafe(ctx context.Context, orgID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	demo := []model.Employee{
		{ID: "emp_001", EmployeeNumber: "EMP-001", FirstName: "Ava", LastName: "Morgan", WorkEmail: "ava@northstar.local", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", Status: "active"},
		{ID: "emp_002", EmployeeNumber: "EMP-002", FirstName: "Daniel", LastName: "Ng", WorkEmail: "daniel@northstar.local", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", Status: "active"},
		{ID: "emp_003", EmployeeNumber: "EMP-003", FirstName: "Sara", LastName: "Ibrahim", WorkEmail: "sara@northstar.local", Role: "People Operations Lead", Department: "People", Location: "Dubai", Status: "active"},
		{ID: "emp_004", EmployeeNumber: "EMP-004", FirstName: "Jonas", LastName: "Lee", WorkEmail: "jonas@northstar.local", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", Status: "leave"},
	}
	for _, employee := range demo {
		if _, err := tx.Exec(ctx, `
			INSERT INTO employees (
				id, organization_id, employee_number, first_name, last_name, work_email,
				job_title, department, location, employment_type, status, server_version
			) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, 'full_time', $10, 1)
			ON CONFLICT (organization_id, id) DO NOTHING`, employee.ID, orgID, employee.EmployeeNumber,
			employee.FirstName, employee.LastName, employee.WorkEmail, employee.Role,
			employee.Department, employee.Location, employee.Status); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Keep pgx imported here so this file documents that the bootstrap remains
// transaction-compatible with the rest of the store package.
var _ pgx.Tx
