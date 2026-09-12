package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Bootstrap(ctx context.Context, orgName, orgSlug, adminName, adminEmail, adminPassword string, seedDemo bool) (string, error) {
	if orgName == "" || orgSlug == "" {
		return "", errors.New("bootstrap organization name and slug are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var orgID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
		RETURNING id::text`, orgName, orgSlug).Scan(&orgID); err != nil {
		return "", err
	}

	if adminEmail != "" && adminPassword != "" {
		hash, err := auth.HashPassword(adminPassword)
		if err != nil {
			return "", fmt.Errorf("bootstrap password: %w", err)
		}
		var userID string
		err = tx.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, display_name)
			VALUES (lower($1), $2, $3)
			ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = now()
			RETURNING id::text`, adminEmail, hash, adminName).Scan(&userID)
		if err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_memberships (organization_id, user_id, role)
			VALUES ($1::uuid, $2::uuid, 'admin')
			ON CONFLICT (organization_id, user_id) DO UPDATE SET role = 'admin', updated_at = now()`, orgID, userID); err != nil {
			return "", err
		}
	}

	if seedDemo {
		if err := seedDemoEmployees(ctx, tx, orgID); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgID, nil
}

func seedDemoEmployees(ctx context.Context, tx pgx.Tx, orgID string) error {
	demo := []model.Employee{
		{ID: "emp_001", EmployeeNumber: "EMP-001", FirstName: "Ava", LastName: "Morgan", WorkEmail: "ava@northstar.local", Role: "Senior Product Designer", Department: "Product", Location: "Colombo", Status: "active"},
		{ID: "emp_002", EmployeeNumber: "EMP-002", FirstName: "Daniel", LastName: "Ng", WorkEmail: "daniel@northstar.local", Role: "Platform Engineer", Department: "Engineering", Location: "Singapore", Status: "active"},
		{ID: "emp_003", EmployeeNumber: "EMP-003", FirstName: "Sara", LastName: "Ibrahim", WorkEmail: "sara@northstar.local", Role: "People Operations Lead", Department: "People", Location: "Dubai", Status: "active"},
		{ID: "emp_004", EmployeeNumber: "EMP-004", FirstName: "Jonas", LastName: "Lee", WorkEmail: "jonas@northstar.local", Role: "Account Executive", Department: "Revenue", Location: "Melbourne", Status: "leave"},
	}
	for _, employee := range demo {
		_, err := tx.Exec(ctx, `
			INSERT INTO employees (
				id, organization_id, employee_number, first_name, last_name, work_email,
				job_title, department, location, employment_type, status, server_version
			) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, 'full_time', $10, 1)
			ON CONFLICT (id) DO NOTHING`, employee.ID, orgID, employee.EmployeeNumber, employee.FirstName, employee.LastName,
			employee.WorkEmail, employee.Role, employee.Department, employee.Location, employee.Status)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LoginIdentity(ctx context.Context, email, orgSlug string) (model.LoginIdentity, error) {
	var identity model.LoginIdentity
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.display_name, u.password_hash,
		       o.id::text, o.name, m.role
		FROM users u
		JOIN organization_memberships m ON m.user_id = u.id
		JOIN organizations o ON o.id = m.organization_id
		WHERE u.email = lower($1)
		  AND u.status = 'active'
		  AND m.status = 'active'
		  AND ($2 = '' OR o.slug = $2)
		ORDER BY m.created_at ASC
		LIMIT 1`, email, orgSlug).Scan(
		&identity.UserID, &identity.Email, &identity.DisplayName, &identity.PasswordHash,
		&identity.OrganizationID, &identity.Organization, &identity.Role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity, ErrNotFound
	}
	return identity, err
}

func (s *Store) ListEmployees(ctx context.Context, orgID string) ([]model.Employee, error) {
	rows, err := s.pool.Query(ctx, employeeSelect+`
		WHERE organization_id = $1::uuid
		ORDER BY first_name, last_name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var employees []model.Employee
	for rows.Next() {
		employee, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, rows.Err()
}

func (s *Store) GetEmployee(ctx context.Context, orgID, id string) (model.Employee, error) {
	employee, err := scanEmployee(s.pool.QueryRow(ctx, employeeSelect+`
		WHERE organization_id = $1::uuid AND id = $2`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Employee{}, ErrNotFound
	}
	return employee, err
}

func (s *Store) CreateEmployee(ctx context.Context, orgID string, input model.CreateEmployee) (model.Employee, error) {
	id, err := newEmployeeID()
	if err != nil {
		return model.Employee{}, err
	}
	if input.EmployeeNumber == "" {
		input.EmployeeNumber = strings.ToUpper(strings.Replace(id, "emp_", "EMP-", 1))
	}
	if input.EmploymentType == "" {
		input.EmploymentType = "full_time"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	employee, err := scanEmployee(s.pool.QueryRow(ctx, `
		INSERT INTO employees (
			id, organization_id, employee_number, first_name, last_name, work_email,
			job_title, department, location, employment_type, status, server_version
		) VALUES ($1, $2::uuid, $3, $4, $5, lower($6), $7, $8, $9, $10, $11, 1)
		RETURNING id, employee_number, first_name, last_name, work_email,
		          COALESCE(job_title, ''), COALESCE(department, ''), COALESCE(location, ''),
		          employment_type, status, server_version, updated_at`,
		id, orgID, input.EmployeeNumber, input.FirstName, input.LastName, input.WorkEmail,
		input.Role, input.Department, input.Location, input.EmploymentType, input.Status))
	return employee, err
}

func (s *Store) PatchEmployee(ctx context.Context, orgID, id string, patch model.PatchEmployee) (model.Employee, error) {
	employee, err := scanEmployee(s.pool.QueryRow(ctx, `
		UPDATE employees SET
			employee_number = COALESCE($3, employee_number),
			first_name = COALESCE($4, first_name),
			last_name = COALESCE($5, last_name),
			work_email = lower(COALESCE($6, work_email)),
			job_title = COALESCE($7, job_title),
			department = COALESCE($8, department),
			location = COALESCE($9, location),
			employment_type = COALESCE($10, employment_type),
			status = COALESCE($11, status),
			server_version = server_version + 1,
			updated_at = now()
		WHERE organization_id = $1::uuid AND id = $2
		RETURNING id, employee_number, first_name, last_name, work_email,
		          COALESCE(job_title, ''), COALESCE(department, ''), COALESCE(location, ''),
		          employment_type, status, server_version, updated_at`,
		orgID, id, patch.EmployeeNumber, patch.FirstName, patch.LastName, patch.WorkEmail,
		patch.Role, patch.Department, patch.Location, patch.EmploymentType, patch.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Employee{}, ErrNotFound
	}
	return employee, err
}

func (s *Store) ArchiveEmployee(ctx context.Context, orgID, id string) (model.Employee, error) {
	status := "inactive"
	return s.PatchEmployee(ctx, orgID, id, model.PatchEmployee{Status: &status})
}

func (s *Store) Dashboard(ctx context.Context, orgID string) (map[string]int, error) {
	var employees, presentToday, onLeave, openRoles, pendingLeave int
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::int FROM employees WHERE organization_id = $1::uuid AND status <> 'inactive'),
			(SELECT count(*)::int FROM attendance_entries WHERE organization_id = $1::uuid AND work_date = current_date AND status = 'present'),
			(SELECT count(*)::int FROM employees WHERE organization_id = $1::uuid AND status = 'leave'),
			(SELECT COALESCE(sum(openings), 0)::int FROM jobs WHERE organization_id = $1::uuid AND status = 'open'),
			(SELECT count(*)::int FROM leave_requests WHERE organization_id = $1::uuid AND status = 'pending')`, orgID).Scan(
		&employees, &presentToday, &onLeave, &openRoles, &pendingLeave,
	)
	if err != nil {
		return nil, err
	}
	return map[string]int{
		"employees":              employees,
		"present_today":          presentToday,
		"on_leave":               onLeave,
		"open_roles":             openRoles,
		"pending_leave_requests": pendingLeave,
	}, nil
}

func (s *Store) LeaveRequests(ctx context.Context, orgID string) ([]model.LeaveRequest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id::text, trim(e.first_name || ' ' || e.last_name), l.leave_type,
		       to_char(l.start_date, 'YYYY-MM-DD'), to_char(l.end_date, 'YYYY-MM-DD'),
		       ceil(l.days)::int, l.status
		FROM leave_requests l
		JOIN employees e ON e.id = l.employee_id AND e.organization_id = l.organization_id
		WHERE l.organization_id = $1::uuid
		ORDER BY l.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []model.LeaveRequest
	for rows.Next() {
		var request model.LeaveRequest
		if err := rows.Scan(&request.ID, &request.Employee, &request.Type, &request.Start, &request.End, &request.Days, &request.Status); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *Store) SyncUpsertEmployee(ctx context.Context, orgID string, incoming model.Employee, baseVersion int64) (model.Employee, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Employee{}, false, err
	}
	defer tx.Rollback(ctx)

	current, err := scanEmployee(tx.QueryRow(ctx, employeeSelect+`
		WHERE organization_id = $1::uuid AND id = $2 FOR UPDATE`, orgID, incoming.ID))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.Employee{}, false, err
	}

	incoming = normalizeSyncedEmployee(incoming)
	if errors.Is(err, pgx.ErrNoRows) {
		created, err := scanEmployee(tx.QueryRow(ctx, `
			INSERT INTO employees (
				id, organization_id, employee_number, first_name, last_name, work_email,
				job_title, department, location, employment_type, status, server_version
			) VALUES ($1, $2::uuid, $3, $4, $5, lower($6), $7, $8, $9, $10, $11, 1)
			RETURNING id, employee_number, first_name, last_name, work_email,
			          COALESCE(job_title, ''), COALESCE(department, ''), COALESCE(location, ''),
			          employment_type, status, server_version, updated_at`,
			incoming.ID, orgID, incoming.EmployeeNumber, incoming.FirstName, incoming.LastName,
			incoming.WorkEmail, incoming.Role, incoming.Department, incoming.Location,
			incoming.EmploymentType, incoming.Status))
		if err != nil {
			return model.Employee{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return model.Employee{}, false, err
		}
		return created, false, nil
	}

	if baseVersion != 0 && baseVersion < current.ServerVersion {
		return current, true, nil
	}
	updated, err := scanEmployee(tx.QueryRow(ctx, `
		UPDATE employees SET first_name=$3, last_name=$4, job_title=$5, department=$6,
		       location=$7, status=$8, server_version=server_version+1, updated_at=now()
		WHERE organization_id=$1::uuid AND id=$2
		RETURNING id, employee_number, first_name, last_name, work_email,
		          COALESCE(job_title, ''), COALESCE(department, ''), COALESCE(location, ''),
		          employment_type, status, server_version, updated_at`,
		orgID, incoming.ID, incoming.FirstName, incoming.LastName, incoming.Role,
		incoming.Department, incoming.Location, incoming.Status))
	if err != nil {
		return model.Employee{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Employee{}, false, err
	}
	return updated, false, nil
}

func (s *Store) RecordAudit(ctx context.Context, orgID, userID, action, resourceType, resourceID string, metadata string) {
	if metadata == "" {
		metadata = `{}`
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO audit_events (organization_id, actor_user_id, action, resource_type, resource_id, metadata)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5, $6::jsonb)`,
		orgID, userID, action, resourceType, resourceID, metadata)
}

const employeeSelect = `
	SELECT id, employee_number, first_name, last_name, work_email,
	       COALESCE(job_title, ''), COALESCE(department, ''), COALESCE(location, ''),
	       employment_type, status, server_version, updated_at
	FROM employees `

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEmployee(row rowScanner) (model.Employee, error) {
	var employee model.Employee
	err := row.Scan(
		&employee.ID, &employee.EmployeeNumber, &employee.FirstName, &employee.LastName,
		&employee.WorkEmail, &employee.Role, &employee.Department, &employee.Location,
		&employee.EmploymentType, &employee.Status, &employee.ServerVersion, &employee.UpdatedAt,
	)
	employee.Name = strings.TrimSpace(employee.FirstName + " " + employee.LastName)
	return employee, err
}

func normalizeSyncedEmployee(employee model.Employee) model.Employee {
	if employee.ID == "" {
		employee.ID = "emp_unknown"
	}
	if employee.FirstName == "" && employee.LastName == "" {
		parts := strings.Fields(employee.Name)
		if len(parts) > 0 {
			employee.FirstName = parts[0]
			if len(parts) > 1 {
				employee.LastName = strings.Join(parts[1:], " ")
			}
		}
	}
	if employee.FirstName == "" {
		employee.FirstName = "Unknown"
	}
	if employee.EmployeeNumber == "" {
		employee.EmployeeNumber = strings.ToUpper(strings.Replace(employee.ID, "emp_", "EMP-", 1))
	}
	if employee.WorkEmail == "" {
		employee.WorkEmail = employee.ID + "@desktop.local.invalid"
	}
	if employee.EmploymentType == "" {
		employee.EmploymentType = "full_time"
	}
	if employee.Status == "" {
		employee.Status = "active"
	}
	return employee
}

func newEmployeeID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "emp_" + hex.EncodeToString(buf), nil
}

func (s *Store) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(ctx)
}
