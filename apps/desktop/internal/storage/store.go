package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/domain"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/secure"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	cipher *secure.Cipher
}

type queuedOperation struct {
	ID          string
	EntityType  string
	EntityID    string
	Action      string
	Payload     string
	BaseVersion int64
	ChangedAt   string
}

func Open(path string, cipher *secure.Cipher) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}

	store := &Store{db: db, cipher: cipher}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS employees (
  id TEXT PRIMARY KEY,
  encrypted_payload TEXT NOT NULL,
  server_version INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_queue (
  id TEXT PRIMARY KEY,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  action TEXT NOT NULL,
  encrypted_payload TEXT NOT NULL,
  base_version INTEGER NOT NULL DEFAULT 0,
  changed_at TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);

CREATE TABLE IF NOT EXISTS sync_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("migrate desktop database: %w", err)
	}
	return nil
}

func (s *Store) Employees(ctx context.Context) ([]domain.Employee, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT encrypted_payload FROM employees ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make([]domain.Employee, 0)
	for rows.Next() {
		var encrypted string
		if err := rows.Scan(&encrypted); err != nil {
			return nil, err
		}
		plain, err := s.cipher.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt employee: %w", err)
		}
		var employee domain.Employee
		if err := json.Unmarshal(plain, &employee); err != nil {
			return nil, fmt.Errorf("decode employee: %w", err)
		}
		employees = append(employees, employee)
	}
	return employees, rows.Err()
}

func (s *Store) EmployeeCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM employees`).Scan(&count)
	return count, err
}

func (s *Store) ApplyRemoteEmployee(ctx context.Context, employee domain.Employee) error {
	if employee.UpdatedAt.IsZero() {
		employee.UpdatedAt = time.Now().UTC()
	}
	return s.writeEmployee(ctx, employee)
}

func (s *Store) SaveLocalEmployee(ctx context.Context, employee domain.Employee) error {
	if employee.ID == "" {
		return fmt.Errorf("employee id is required")
	}
	if employee.UpdatedAt.IsZero() {
		employee.UpdatedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(employee)
	if err != nil {
		return err
	}
	encrypted, err := s.cipher.Encrypt(payload)
	if err != nil {
		return err
	}
	operationID := fmt.Sprintf("op_%d", time.Now().UnixNano())

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO employees (id, encrypted_payload, server_version, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  encrypted_payload = excluded.encrypted_payload,
  server_version = excluded.server_version,
  updated_at = excluded.updated_at
`, employee.ID, encrypted, employee.ServerVersion, employee.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}

	// Coalesce repeated offline edits for the same employee. The newest payload
	// replaces older unsynced upserts while preserving the same cloud base
	// version, preventing a device from conflicting with its own edit history.
	if _, err := tx.ExecContext(ctx, `
DELETE FROM sync_queue
WHERE entity_type = 'employee' AND entity_id = ? AND action = 'upsert'
`, employee.ID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_queue (id, entity_type, entity_id, action, encrypted_payload, base_version, changed_at)
VALUES (?, 'employee', ?, 'upsert', ?, ?, ?)
`, operationID, employee.ID, encrypted, employee.ServerVersion, employee.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) writeEmployee(ctx context.Context, employee domain.Employee) error {
	payload, err := json.Marshal(employee)
	if err != nil {
		return err
	}
	encrypted, err := s.cipher.Encrypt(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO employees (id, encrypted_payload, server_version, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  encrypted_payload = excluded.encrypted_payload,
  server_version = excluded.server_version,
  updated_at = excluded.updated_at
`, employee.ID, encrypted, employee.ServerVersion, employee.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) PendingOperations(ctx context.Context, limit int) ([]domain.SyncOperation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, entity_type, entity_id, action, encrypted_payload, base_version, changed_at
FROM sync_queue
ORDER BY changed_at ASC
LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	operations := make([]domain.SyncOperation, 0)
	for rows.Next() {
		var row queuedOperation
		if err := rows.Scan(&row.ID, &row.EntityType, &row.EntityID, &row.Action, &row.Payload, &row.BaseVersion, &row.ChangedAt); err != nil {
			return nil, err
		}
		plain, err := s.cipher.Decrypt(row.Payload)
		if err != nil {
			return nil, err
		}
		var employee domain.Employee
		if err := json.Unmarshal(plain, &employee); err != nil {
			return nil, err
		}
		changedAt, _ := time.Parse(time.RFC3339Nano, row.ChangedAt)
		operations = append(operations, domain.SyncOperation{
			ID: row.ID, EntityType: row.EntityType, EntityID: row.EntityID,
			Action: row.Action, Payload: employee, BaseVersion: row.BaseVersion, ChangedAt: changedAt,
		})
	}
	return operations, rows.Err()
}

func (s *Store) DeleteOperations(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sync_queue WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) HasPendingEntity(ctx context.Context, entityType, entityID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sync_queue WHERE entity_type = ? AND entity_id = ?
`, entityType, entityID).Scan(&count)
	return count > 0, err
}

func (s *Store) PendingCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue`).Scan(&count)
	return count, err
}

func (s *Store) State(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM sync_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sync_state (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, key, value)
	return err
}
