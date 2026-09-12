package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

func (s *Store) CreateAccessInvitation(ctx context.Context, orgID, createdBy, email, role, employeeID, tokenHash string, expiresAt time.Time) (model.AccessInvitation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.AccessInvitation{}, err
	}
	defer tx.Rollback(ctx)

	email = strings.ToLower(strings.TrimSpace(email))
	employeeID = strings.TrimSpace(employeeID)
	if employeeID != "" {
		var workEmail string
		if err := tx.QueryRow(ctx, `SELECT lower(work_email) FROM employees WHERE organization_id=$1::uuid AND id=$2`, orgID, employeeID).Scan(&workEmail); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return model.AccessInvitation{}, ErrNotFound
			}
			return model.AccessInvitation{}, err
		}
		if workEmail != email {
			return model.AccessInvitation{}, ErrConflict
		}
	}

	// Reissuing an invitation invalidates any older pending link for this email.
	if _, err := tx.Exec(ctx, `
		UPDATE access_invitations SET revoked_at=now()
		WHERE organization_id=$1::uuid AND lower(email)=lower($2)
		  AND accepted_at IS NULL AND revoked_at IS NULL`, orgID, email); err != nil {
		return model.AccessInvitation{}, err
	}

	invitation, err := scanAccessInvitation(tx.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO access_invitations (
				organization_id, email, role, employee_id, token_hash, expires_at, created_by
			) VALUES ($1::uuid, lower($2), $3, NULLIF($4, ''), $5, $6, NULLIF($7, '')::uuid)
			RETURNING *
		)
		SELECT i.id::text, i.organization_id::text, o.name, i.email, i.role,
		       COALESCE(i.employee_id, ''), COALESCE(trim(e.first_name || ' ' || e.last_name), ''),
		       i.expires_at, i.accepted_at, i.revoked_at, COALESCE(i.created_by::text, ''), i.created_at
		FROM created i
		JOIN organizations o ON o.id=i.organization_id
		LEFT JOIN employees e ON e.organization_id=i.organization_id AND e.id=i.employee_id`,
		orgID, email, role, employeeID, tokenHash, expiresAt, createdBy))
	if err != nil {
		return model.AccessInvitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.AccessInvitation{}, err
	}
	return invitation, nil
}

func (s *Store) ListAccessInvitations(ctx context.Context, orgID string, limit int) ([]model.AccessInvitation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT i.id::text, i.organization_id::text, o.name, i.email, i.role,
		       COALESCE(i.employee_id, ''), COALESCE(trim(e.first_name || ' ' || e.last_name), ''),
		       i.expires_at, i.accepted_at, i.revoked_at, COALESCE(i.created_by::text, ''), i.created_at
		FROM access_invitations i
		JOIN organizations o ON o.id=i.organization_id
		LEFT JOIN employees e ON e.organization_id=i.organization_id AND e.id=i.employee_id
		WHERE i.organization_id=$1::uuid
		ORDER BY i.created_at DESC
		LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invitations := make([]model.AccessInvitation, 0)
	for rows.Next() {
		invitation, err := scanAccessInvitation(rows)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *Store) RevokeAccessInvitation(ctx context.Context, orgID, invitationID string) error {
	command, err := s.pool.Exec(ctx, `
		UPDATE access_invitations SET revoked_at=now()
		WHERE organization_id=$1::uuid AND id=$2::uuid
		  AND accepted_at IS NULL AND revoked_at IS NULL`, orgID, invitationID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PreviewAccessInvitation(ctx context.Context, tokenHash string) (model.InvitationPreview, error) {
	var preview model.InvitationPreview
	var employeeID, employee string
	err := s.pool.QueryRow(ctx, `
		SELECT i.email, i.role, COALESCE(i.employee_id, ''),
		       COALESCE(trim(e.first_name || ' ' || e.last_name), ''), o.name, i.expires_at,
		       EXISTS(SELECT 1 FROM users u WHERE u.email=lower(i.email))
		FROM access_invitations i
		JOIN organizations o ON o.id=i.organization_id
		LEFT JOIN employees e ON e.organization_id=i.organization_id AND e.id=i.employee_id
		WHERE i.token_hash=$1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()`, tokenHash).Scan(
		&preview.Email, &preview.Role, &employeeID, &employee, &preview.Organization, &preview.ExpiresAt, &preview.ExistingUser,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return preview, ErrNotFound
	}
	preview.EmployeeID = employeeID
	preview.Employee = employee
	return preview, err
}

func (s *Store) AcceptAccessInvitation(ctx context.Context, tokenHash, displayName, password string) (model.LoginIdentity, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.LoginIdentity{}, false, err
	}
	defer tx.Rollback(ctx)

	var invitationID, orgID, orgName, email, role, employeeID, employeeName string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT i.id::text, i.organization_id::text, o.name, i.email, i.role,
		       COALESCE(i.employee_id, ''), COALESCE(trim(e.first_name || ' ' || e.last_name), ''), i.expires_at
		FROM access_invitations i
		JOIN organizations o ON o.id=i.organization_id
		LEFT JOIN employees e ON e.organization_id=i.organization_id AND e.id=i.employee_id
		WHERE i.token_hash=$1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL
		FOR UPDATE OF i`, tokenHash).Scan(&invitationID, &orgID, &orgName, &email, &role, &employeeID, &employeeName, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.LoginIdentity{}, false, ErrNotFound
	}
	if err != nil {
		return model.LoginIdentity{}, false, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		return model.LoginIdentity{}, false, ErrConflict
	}

	var userID, passwordHash, existingName, userStatus string
	err = tx.QueryRow(ctx, `SELECT id::text, password_hash, display_name, status FROM users WHERE email=lower($1)`, email).Scan(&userID, &passwordHash, &existingName, &userStatus)
	createdUser := false
	if errors.Is(err, pgx.ErrNoRows) {
		name := strings.TrimSpace(displayName)
		if name == "" {
			name = strings.TrimSpace(employeeName)
		}
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		hash, hashErr := auth.HashPassword(password)
		if hashErr != nil {
			return model.LoginIdentity{}, false, hashErr
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, display_name)
			VALUES (lower($1), $2, $3)
			RETURNING id::text, password_hash, display_name, status`, email, hash, name).Scan(&userID, &passwordHash, &existingName, &userStatus); err != nil {
			return model.LoginIdentity{}, false, err
		}
		createdUser = true
	} else if err != nil {
		return model.LoginIdentity{}, false, err
	}
	if userStatus != "active" {
		return model.LoginIdentity{}, false, ErrConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, employee_id, role, status)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, ''), $4, 'active')
		ON CONFLICT (organization_id, user_id) DO UPDATE SET
			employee_id=EXCLUDED.employee_id, role=EXCLUDED.role, status='active', updated_at=now()`,
		orgID, userID, employeeID, role); err != nil {
		return model.LoginIdentity{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE access_invitations SET accepted_at=now() WHERE id=$1::uuid`, invitationID); err != nil {
		return model.LoginIdentity{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.LoginIdentity{}, false, err
	}

	return model.LoginIdentity{
		Principal: model.Principal{
			UserID: userID, Email: strings.ToLower(email), DisplayName: existingName,
			OrganizationID: orgID, Organization: orgName, Role: role,
		},
		PasswordHash: passwordHash,
	}, createdUser, nil
}

func scanAccessInvitation(row rowScanner) (model.AccessInvitation, error) {
	var invitation model.AccessInvitation
	err := row.Scan(
		&invitation.ID, &invitation.OrganizationID, &invitation.Organization,
		&invitation.Email, &invitation.Role, &invitation.EmployeeID, &invitation.Employee,
		&invitation.ExpiresAt, &invitation.AcceptedAt, &invitation.RevokedAt,
		&invitation.CreatedBy, &invitation.CreatedAt,
	)
	return invitation, err
}
