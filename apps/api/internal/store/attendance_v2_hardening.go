package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

// AssignWorkScheduleSafe prevents overlapping effective-dated assignments for
// an employee. The table lock intentionally serializes this low-frequency HR
// administration path so two concurrent assignment requests cannot both pass
// the overlap check and create ambiguous schedule resolution.
func (s *Store) AssignWorkScheduleSafe(ctx context.Context, orgID, scheduleID string, input model.WorkScheduleAssignment) error {
	from, err := time.Parse("2006-01-02", input.EffectiveFrom)
	if err != nil {
		return err
	}
	var to *time.Time
	if input.EffectiveTo != "" {
		parsed, err := time.Parse("2006-01-02", input.EffectiveTo)
		if err != nil {
			return err
		}
		if parsed.Before(from) {
			return errors.New("effective_to must be on or after effective_from")
		}
		to = &parsed
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `LOCK TABLE work_schedule_assignments IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM work_schedules
			WHERE organization_id=$1::uuid AND id=$2::uuid AND active=true
		)`, orgID, scheduleID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM employees
			WHERE organization_id=$1::uuid AND id=$2 AND status <> 'inactive'
		)`, orgID, input.EmployeeID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	var overlap bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM work_schedule_assignments
			WHERE organization_id=$1::uuid AND employee_id=$2
			  AND NOT (schedule_id=$3::uuid AND effective_from=$4::date)
			  AND (effective_to IS NULL OR effective_to >= $4::date)
			  AND ($5::date IS NULL OR effective_from <= $5::date)
		)`, orgID, input.EmployeeID, scheduleID, from.Format("2006-01-02"), nullableDate(to)).Scan(&overlap); err != nil {
		return err
	}
	if overlap {
		return ErrConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO work_schedule_assignments (organization_id, employee_id, schedule_id, effective_from, effective_to)
		VALUES ($1::uuid, $2, $3::uuid, $4::date, $5::date)
		ON CONFLICT (organization_id, employee_id, schedule_id, effective_from)
		DO UPDATE SET effective_to=EXCLUDED.effective_to`,
		orgID, input.EmployeeID, scheduleID, from.Format("2006-01-02"), nullableDate(to)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CheckOutTodayV2 closes only the still-open attendance entry whose work_date
// is "today" in the schedule timezone snapshotted on that row. It never falls
// back to an older forgotten entry.
func (s *Store) CheckOutTodayV2(ctx context.Context, orgID, employeeID string) (model.AttendanceEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	defer tx.Rollback(ctx)

	entry, err := scanAttendanceEntryV2(tx.QueryRow(ctx, `
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date,'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status,
		       COALESCE(a.schedule_id::text,''), COALESCE(s.name,''), a.scheduled_start_at, a.scheduled_end_at,
		       a.break_minutes, a.late_minutes, a.early_leave_minutes, a.worked_minutes,
		       a.overtime_minutes, a.overtime_threshold_minutes, a.source
		FROM attendance_entries a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		LEFT JOIN work_schedules s ON s.id=a.schedule_id AND s.organization_id=a.organization_id
		WHERE a.organization_id=$1::uuid AND a.employee_id=$2
		  AND a.check_in IS NOT NULL AND a.check_out IS NULL
		  AND a.work_date=(now() AT TIME ZONE COALESCE(s.timezone,'UTC'))::date
		ORDER BY a.work_date DESC
		LIMIT 1 FOR UPDATE OF a`, orgID, employeeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AttendanceEntry{}, ErrNotFound
	}
	if err != nil {
		return model.AttendanceEntry{}, err
	}

	checkOut := time.Now().UTC()
	worked, early, overtime := calculateAttendanceMetrics(
		*entry.CheckIn, checkOut, entry.ScheduledStartAt, entry.ScheduledEndAt,
		entry.BreakMinutes, entry.OvertimeThresholdMinutes,
	)
	updated, err := scanAttendanceEntryV2(tx.QueryRow(ctx, `
		WITH changed AS (
			UPDATE attendance_entries
			SET check_out=$3, worked_minutes=$4, early_leave_minutes=$5,
			    overtime_minutes=$6, updated_at=now()
			WHERE organization_id=$1::uuid AND id=$2::uuid AND check_out IS NULL
			RETURNING *
		)
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date,'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status,
		       COALESCE(a.schedule_id::text,''), COALESCE(s.name,''), a.scheduled_start_at, a.scheduled_end_at,
		       a.break_minutes, a.late_minutes, a.early_leave_minutes, a.worked_minutes,
		       a.overtime_minutes, a.overtime_threshold_minutes, a.source
		FROM changed a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		LEFT JOIN work_schedules s ON s.id=a.schedule_id AND s.organization_id=a.organization_id`,
		orgID, entry.ID, checkOut, worked, early, overtime))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AttendanceEntry{}, ErrConflict
	}
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.AttendanceEntry{}, err
	}
	return updated, nil
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}
