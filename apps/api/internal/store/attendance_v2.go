package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/model"
)

type attendanceQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) EnsureDefaultWorkSchedule(ctx context.Context, orgID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var hasDefault bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM work_schedules WHERE organization_id=$1::uuid AND is_default=true AND active=true)`, orgID).Scan(&hasDefault); err != nil {
		return err
	}
	if !hasDefault {
		if _, err := tx.Exec(ctx, `
			INSERT INTO work_schedules (
				organization_id, code, name, timezone, start_time, end_time,
				break_minutes, grace_minutes, overtime_threshold_minutes, work_days, is_default, active
			) VALUES ($1::uuid, 'standard', 'Standard workday', 'UTC', '09:00', '17:00', 60, 10, 15, ARRAY[1,2,3,4,5]::smallint[], true, true)
			ON CONFLICT (organization_id, code) DO UPDATE SET is_default=true, active=true, updated_at=now()`, orgID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListWorkSchedules(ctx context.Context, orgID string) ([]model.WorkSchedule, error) {
	if err := s.EnsureDefaultWorkSchedule(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, code, name, timezone, to_char(start_time,'HH24:MI'), to_char(end_time,'HH24:MI'),
		       break_minutes, grace_minutes, overtime_threshold_minutes, work_days,
		       is_default, active, created_at, updated_at
		FROM work_schedules
		WHERE organization_id=$1::uuid
		ORDER BY is_default DESC, active DESC, name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var schedules []model.WorkSchedule
	for rows.Next() {
		schedule, err := scanWorkSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
}

func (s *Store) UpsertWorkSchedule(ctx context.Context, orgID string, input model.UpsertWorkSchedule) (model.WorkSchedule, error) {
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		// Go's zone database is not guaranteed in the minimal runtime, so PostgreSQL
		// remains authoritative. This catches common invalid zones when available.
		var ignored time.Time
		if dbErr := s.pool.QueryRow(ctx, `SELECT now() AT TIME ZONE $1`, input.Timezone).Scan(&ignored); dbErr != nil {
			return model.WorkSchedule{}, fmt.Errorf("invalid timezone")
		}
	}
	workDays := make([]int16, 0, len(input.WorkDays))
	for _, day := range input.WorkDays {
		workDays = append(workDays, int16(day))
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.WorkSchedule{}, err
	}
	defer tx.Rollback(ctx)
	if input.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE work_schedules SET is_default=false, updated_at=now() WHERE organization_id=$1::uuid AND is_default=true`, orgID); err != nil {
			return model.WorkSchedule{}, err
		}
	}
	schedule, err := scanWorkSchedule(tx.QueryRow(ctx, `
		INSERT INTO work_schedules (
			organization_id, code, name, timezone, start_time, end_time,
			break_minutes, grace_minutes, overtime_threshold_minutes, work_days, is_default, active
		) VALUES ($1::uuid, $2, $3, $4, $5::time, $6::time, $7, $8, $9, $10::smallint[], $11, true)
		ON CONFLICT (organization_id, code) DO UPDATE SET
			name=EXCLUDED.name, timezone=EXCLUDED.timezone, start_time=EXCLUDED.start_time,
			end_time=EXCLUDED.end_time, break_minutes=EXCLUDED.break_minutes,
			grace_minutes=EXCLUDED.grace_minutes,
			overtime_threshold_minutes=EXCLUDED.overtime_threshold_minutes,
			work_days=EXCLUDED.work_days, is_default=EXCLUDED.is_default,
			active=true, updated_at=now()
		RETURNING id::text, code, name, timezone, to_char(start_time,'HH24:MI'), to_char(end_time,'HH24:MI'),
		          break_minutes, grace_minutes, overtime_threshold_minutes, work_days,
		          is_default, active, created_at, updated_at`,
		orgID, input.Code, input.Name, input.Timezone, input.StartTime, input.EndTime,
		input.BreakMinutes, input.GraceMinutes, input.OvertimeThresholdMinutes, workDays, input.IsDefault))
	if err != nil {
		return model.WorkSchedule{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.WorkSchedule{}, err
	}
	return schedule, nil
}

func (s *Store) AssignWorkSchedule(ctx context.Context, orgID, scheduleID string, input model.WorkScheduleAssignment) error {
	var to any
	if input.EffectiveTo != "" {
		to = input.EffectiveTo
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO work_schedule_assignments (organization_id, employee_id, schedule_id, effective_from, effective_to)
		VALUES ($1::uuid, $2, $3::uuid, $4::date, $5::date)
		ON CONFLICT (organization_id, employee_id, schedule_id, effective_from)
		DO UPDATE SET effective_to=EXCLUDED.effective_to`,
		orgID, input.EmployeeID, scheduleID, input.EffectiveFrom, to)
	return err
}

func (s *Store) ListAttendanceV2(ctx context.Context, orgID string, workDate time.Time, scopeEmployeeID string, includeReports bool) ([]model.AttendanceEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date, 'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status,
		       COALESCE(a.schedule_id::text,''), COALESCE(s.name,''), a.scheduled_start_at, a.scheduled_end_at,
		       a.break_minutes, a.late_minutes, a.early_leave_minutes, a.worked_minutes,
		       a.overtime_minutes, a.overtime_threshold_minutes, a.source
		FROM attendance_entries a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		LEFT JOIN work_schedules s ON s.id=a.schedule_id AND s.organization_id=a.organization_id
		WHERE a.organization_id=$1::uuid AND a.work_date=$2::date
		  AND ($3='' OR e.id=$3 OR ($4::boolean AND e.manager_id=$3))
		ORDER BY e.first_name, e.last_name`, orgID, workDate.Format("2006-01-02"), scopeEmployeeID, includeReports)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []model.AttendanceEntry
	for rows.Next() {
		entry, err := scanAttendanceEntryV2(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) CheckInV2(ctx context.Context, orgID, employeeID, workMode, source string) (model.AttendanceEntry, error) {
	if err := s.EnsureDefaultWorkSchedule(ctx, orgID); err != nil {
		return model.AttendanceEntry{}, err
	}
	now := time.Now().UTC()
	provisionalDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	schedule, err := resolveWorkSchedule(ctx, s.pool, orgID, employeeID, provisionalDate)
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	localDate, err := localDateInZone(ctx, s.pool, schedule.Timezone)
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	if !sameCalendarDate(localDate, provisionalDate) {
		schedule, err = resolveWorkSchedule(ctx, s.pool, orgID, employeeID, localDate)
		if err != nil {
			return model.AttendanceEntry{}, err
		}
	}

	var scheduledStart, scheduledEnd *time.Time
	breakMinutes := 0
	overtimeThreshold := 0
	lateMinutes := 0
	if workScheduleIncludesDay(schedule, localDate) {
		start, end, err := workScheduleBounds(ctx, s.pool, schedule, localDate)
		if err != nil {
			return model.AttendanceEntry{}, err
		}
		scheduledStart, scheduledEnd = &start, &end
		breakMinutes = schedule.BreakMinutes
		overtimeThreshold = schedule.OvertimeThresholdMinutes
		lateMinutes = maxInt(0, minutesBetween(start, now)-schedule.GraceMinutes)
	}

	return scanAttendanceEntryV2(s.pool.QueryRow(ctx, `
		WITH changed AS (
			INSERT INTO attendance_entries (
				organization_id, employee_id, work_date, check_in, work_mode, status,
				schedule_id, scheduled_start_at, scheduled_end_at, break_minutes,
				overtime_threshold_minutes, late_minutes, source
			) VALUES ($1::uuid, $2, $3::date, $4, $5, 'present', $6::uuid, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (organization_id, employee_id, work_date) DO UPDATE SET
				check_in=COALESCE(attendance_entries.check_in, EXCLUDED.check_in),
				work_mode=EXCLUDED.work_mode,
				status='present',
				schedule_id=COALESCE(attendance_entries.schedule_id, EXCLUDED.schedule_id),
				scheduled_start_at=COALESCE(attendance_entries.scheduled_start_at, EXCLUDED.scheduled_start_at),
				scheduled_end_at=COALESCE(attendance_entries.scheduled_end_at, EXCLUDED.scheduled_end_at),
				break_minutes=CASE WHEN attendance_entries.check_in IS NULL THEN EXCLUDED.break_minutes ELSE attendance_entries.break_minutes END,
				overtime_threshold_minutes=CASE WHEN attendance_entries.check_in IS NULL THEN EXCLUDED.overtime_threshold_minutes ELSE attendance_entries.overtime_threshold_minutes END,
				late_minutes=CASE WHEN attendance_entries.check_in IS NULL THEN EXCLUDED.late_minutes ELSE attendance_entries.late_minutes END,
				source=CASE WHEN attendance_entries.check_in IS NULL THEN EXCLUDED.source ELSE attendance_entries.source END,
				updated_at=now()
			RETURNING *
		)
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date,'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status,
		       COALESCE(a.schedule_id::text,''), COALESCE(s.name,''), a.scheduled_start_at, a.scheduled_end_at,
		       a.break_minutes, a.late_minutes, a.early_leave_minutes, a.worked_minutes,
		       a.overtime_minutes, a.overtime_threshold_minutes, a.source
		FROM changed a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		LEFT JOIN work_schedules s ON s.id=a.schedule_id`,
		orgID, employeeID, localDate.Format("2006-01-02"), now, workMode, schedule.ID,
		scheduledStart, scheduledEnd, breakMinutes, overtimeThreshold, lateMinutes, source))
}

func (s *Store) CheckOutV2(ctx context.Context, orgID, employeeID string) (model.AttendanceEntry, error) {
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
		LEFT JOIN work_schedules s ON s.id=a.schedule_id
		WHERE a.organization_id=$1::uuid AND a.employee_id=$2 AND a.check_in IS NOT NULL
		ORDER BY a.work_date DESC LIMIT 1 FOR UPDATE OF a`, orgID, employeeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AttendanceEntry{}, ErrNotFound
	}
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	if entry.CheckIn == nil {
		return model.AttendanceEntry{}, ErrNotFound
	}
	checkOut := time.Now().UTC()
	if entry.CheckOut != nil {
		checkOut = *entry.CheckOut
	}
	worked, early, overtime := calculateAttendanceMetrics(*entry.CheckIn, checkOut, entry.ScheduledStartAt, entry.ScheduledEndAt, entry.BreakMinutes, entry.OvertimeThresholdMinutes)

	updated, err := scanAttendanceEntryV2(tx.QueryRow(ctx, `
		WITH changed AS (
			UPDATE attendance_entries
			SET check_out=COALESCE(check_out,$3), worked_minutes=$4, early_leave_minutes=$5,
			    overtime_minutes=$6, updated_at=now()
			WHERE organization_id=$1::uuid AND id=$2::uuid
			RETURNING *
		)
		SELECT a.id::text, a.employee_id, trim(e.first_name || ' ' || e.last_name),
		       to_char(a.work_date,'YYYY-MM-DD'), a.check_in, a.check_out, a.work_mode, a.status,
		       COALESCE(a.schedule_id::text,''), COALESCE(s.name,''), a.scheduled_start_at, a.scheduled_end_at,
		       a.break_minutes, a.late_minutes, a.early_leave_minutes, a.worked_minutes,
		       a.overtime_minutes, a.overtime_threshold_minutes, a.source
		FROM changed a
		JOIN employees e ON e.organization_id=a.organization_id AND e.id=a.employee_id
		LEFT JOIN work_schedules s ON s.id=a.schedule_id`,
		orgID, entry.ID, checkOut, worked, early, overtime))
	if err != nil {
		return model.AttendanceEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.AttendanceEntry{}, err
	}
	return updated, nil
}

func (s *Store) CreateAttendanceCorrection(ctx context.Context, orgID, employeeID, submittedBy string, input model.CreateAttendanceCorrection, checkIn, checkOut *time.Time) (model.AttendanceCorrectionRequest, error) {
	var request model.AttendanceCorrectionRequest
	err := scanAttendanceCorrection(s.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO attendance_correction_requests (
				organization_id, employee_id, attendance_entry_id, work_date,
				requested_check_in, requested_check_out, requested_work_mode,
				reason, submitted_by
			) VALUES (
				$1::uuid, $2,
				(SELECT id FROM attendance_entries WHERE organization_id=$1::uuid AND employee_id=$2 AND work_date=$3::date),
				$3::date, $4, $5, $6, $7, NULLIF($8,'')::uuid
			) RETURNING *
		)
		SELECT c.id::text, c.employee_id, trim(e.first_name || ' ' || e.last_name),
		       COALESCE(c.attendance_entry_id::text,''), to_char(c.work_date,'YYYY-MM-DD'),
		       c.requested_check_in, c.requested_check_out, c.requested_work_mode,
		       c.reason, c.status, COALESCE(c.reviewer_user_id::text,''),
		       COALESCE(c.review_note,''), c.reviewed_at, c.created_at
		FROM created c JOIN employees e ON e.organization_id=c.organization_id AND e.id=c.employee_id`,
		orgID, employeeID, input.WorkDate, checkIn, checkOut, input.WorkMode, input.Reason, submittedBy), &request)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.AttendanceCorrectionRequest{}, ErrConflict
		}
		return model.AttendanceCorrectionRequest{}, err
	}
	return request, nil
}

func (s *Store) ListAttendanceCorrections(ctx context.Context, orgID, scopeEmployeeID string, includeReports bool) ([]model.AttendanceCorrectionRequest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text, c.employee_id, trim(e.first_name || ' ' || e.last_name),
		       COALESCE(c.attendance_entry_id::text,''), to_char(c.work_date,'YYYY-MM-DD'),
		       c.requested_check_in, c.requested_check_out, c.requested_work_mode,
		       c.reason, c.status, COALESCE(c.reviewer_user_id::text,''),
		       COALESCE(c.review_note,''), c.reviewed_at, c.created_at
		FROM attendance_correction_requests c
		JOIN employees e ON e.organization_id=c.organization_id AND e.id=c.employee_id
		WHERE c.organization_id=$1::uuid
		  AND ($2='' OR e.id=$2 OR ($3::boolean AND e.manager_id=$2))
		ORDER BY CASE WHEN c.status='pending' THEN 0 ELSE 1 END, c.created_at DESC`, orgID, scopeEmployeeID, includeReports)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []model.AttendanceCorrectionRequest
	for rows.Next() {
		var request model.AttendanceCorrectionRequest
		if err := scanAttendanceCorrection(rows, &request); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *Store) AttendanceCorrectionOwner(ctx context.Context, orgID, correctionID string) (string, error) {
	var employeeID string
	err := s.pool.QueryRow(ctx, `SELECT employee_id FROM attendance_correction_requests WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, correctionID).Scan(&employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return employeeID, err
}

func (s *Store) DecideAttendanceCorrection(ctx context.Context, orgID, correctionID, reviewerUserID, decision, note string) (model.AttendanceCorrectionRequest, error) {
	if err := s.EnsureDefaultWorkSchedule(ctx, orgID); err != nil {
		return model.AttendanceCorrectionRequest{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.AttendanceCorrectionRequest{}, err
	}
	defer tx.Rollback(ctx)

	var employeeID, workDate, requestedMode, currentStatus string
	var requestedIn, requestedOut *time.Time
	var existingEntryID string
	if err := tx.QueryRow(ctx, `
		SELECT employee_id, to_char(work_date,'YYYY-MM-DD'), requested_check_in, requested_check_out,
		       requested_work_mode, status, COALESCE(attendance_entry_id::text,'')
		FROM attendance_correction_requests
		WHERE organization_id=$1::uuid AND id=$2::uuid FOR UPDATE`, orgID, correctionID).Scan(
		&employeeID, &workDate, &requestedIn, &requestedOut, &requestedMode, &currentStatus, &existingEntryID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.AttendanceCorrectionRequest{}, ErrNotFound
		}
		return model.AttendanceCorrectionRequest{}, err
	}
	if currentStatus != "pending" {
		return model.AttendanceCorrectionRequest{}, ErrConflict
	}

	if decision == "approved" {
		var existingIn, existingOut *time.Time
		var existingMode string
		if existingEntryID != "" {
			_ = tx.QueryRow(ctx, `SELECT check_in, check_out, work_mode FROM attendance_entries WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, existingEntryID).Scan(&existingIn, &existingOut, &existingMode)
		}
		if requestedIn == nil {
			requestedIn = existingIn
		}
		if requestedOut == nil {
			requestedOut = existingOut
		}
		if requestedMode == "" {
			requestedMode = existingMode
		}
		if requestedMode == "" {
			requestedMode = "office"
		}
		date, err := time.Parse("2006-01-02", workDate)
		if err != nil {
			return model.AttendanceCorrectionRequest{}, err
		}
		schedule, err := resolveWorkSchedule(ctx, tx, orgID, employeeID, date)
		if err != nil {
			return model.AttendanceCorrectionRequest{}, err
		}
		var scheduledStart, scheduledEnd *time.Time
		breakMinutes, overtimeThreshold, lateMinutes := 0, 0, 0
		if workScheduleIncludesDay(schedule, date) {
			start, end, err := workScheduleBounds(ctx, tx, schedule, date)
			if err != nil {
				return model.AttendanceCorrectionRequest{}, err
			}
			scheduledStart, scheduledEnd = &start, &end
			breakMinutes = schedule.BreakMinutes
			overtimeThreshold = schedule.OvertimeThresholdMinutes
			if requestedIn != nil {
				lateMinutes = maxInt(0, minutesBetween(start, *requestedIn)-schedule.GraceMinutes)
			}
		}
		worked, early, overtime := 0, 0, 0
		if requestedIn != nil && requestedOut != nil {
			worked, early, overtime = calculateAttendanceMetrics(*requestedIn, *requestedOut, scheduledStart, scheduledEnd, breakMinutes, overtimeThreshold)
		}
		var entryID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO attendance_entries (
				organization_id, employee_id, work_date, check_in, check_out, work_mode, status,
				schedule_id, scheduled_start_at, scheduled_end_at, break_minutes,
				overtime_threshold_minutes, late_minutes, early_leave_minutes,
				worked_minutes, overtime_minutes, source
			) VALUES ($1::uuid,$2,$3::date,$4,$5,$6,'present',$7::uuid,$8,$9,$10,$11,$12,$13,$14,$15,'correction')
			ON CONFLICT (organization_id, employee_id, work_date) DO UPDATE SET
				check_in=EXCLUDED.check_in, check_out=EXCLUDED.check_out, work_mode=EXCLUDED.work_mode,
				status='present', schedule_id=EXCLUDED.schedule_id,
				scheduled_start_at=EXCLUDED.scheduled_start_at, scheduled_end_at=EXCLUDED.scheduled_end_at,
				break_minutes=EXCLUDED.break_minutes, overtime_threshold_minutes=EXCLUDED.overtime_threshold_minutes,
				late_minutes=EXCLUDED.late_minutes, early_leave_minutes=EXCLUDED.early_leave_minutes,
				worked_minutes=EXCLUDED.worked_minutes, overtime_minutes=EXCLUDED.overtime_minutes,
				source='correction', updated_at=now()
			RETURNING id::text`, orgID, employeeID, workDate, requestedIn, requestedOut, requestedMode,
			schedule.ID, scheduledStart, scheduledEnd, breakMinutes, overtimeThreshold,
			lateMinutes, early, worked, overtime).Scan(&entryID); err != nil {
			return model.AttendanceCorrectionRequest{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE attendance_correction_requests SET attendance_entry_id=$3::uuid WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, correctionID, entryID); err != nil {
			return model.AttendanceCorrectionRequest{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE attendance_correction_requests
		SET status=$3, reviewer_user_id=NULLIF($4,'')::uuid, review_note=NULLIF($5,''), reviewed_at=now(), updated_at=now()
		WHERE organization_id=$1::uuid AND id=$2::uuid`, orgID, correctionID, decision, reviewerUserID, note); err != nil {
		return model.AttendanceCorrectionRequest{}, err
	}
	var result model.AttendanceCorrectionRequest
	if err := scanAttendanceCorrection(tx.QueryRow(ctx, `
		SELECT c.id::text, c.employee_id, trim(e.first_name || ' ' || e.last_name),
		       COALESCE(c.attendance_entry_id::text,''), to_char(c.work_date,'YYYY-MM-DD'),
		       c.requested_check_in, c.requested_check_out, c.requested_work_mode,
		       c.reason, c.status, COALESCE(c.reviewer_user_id::text,''),
		       COALESCE(c.review_note,''), c.reviewed_at, c.created_at
		FROM attendance_correction_requests c
		JOIN employees e ON e.organization_id=c.organization_id AND e.id=c.employee_id
		WHERE c.organization_id=$1::uuid AND c.id=$2::uuid`, orgID, correctionID), &result); err != nil {
		return model.AttendanceCorrectionRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.AttendanceCorrectionRequest{}, err
	}
	return result, nil
}

func (s *Store) AttendanceTimesheet(ctx context.Context, orgID, employeeID string, from, to time.Time) (model.AttendanceTimesheetSummary, error) {
	if err := s.EnsureDefaultWorkSchedule(ctx, orgID); err != nil {
		return model.AttendanceTimesheetSummary{}, err
	}
	var summary model.AttendanceTimesheetSummary
	summary.EmployeeID = employeeID
	summary.From = from.Format("2006-01-02")
	summary.To = to.Format("2006-01-02")
	var location string
	if err := s.pool.QueryRow(ctx, `
		SELECT trim(first_name || ' ' || last_name), COALESCE(location,'')
		FROM employees WHERE organization_id=$1::uuid AND id=$2`, orgID, employeeID).Scan(&summary.Employee, &location); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.AttendanceTimesheetSummary{}, ErrNotFound
		}
		return model.AttendanceTimesheetSummary{}, err
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int,
		       COALESCE(sum(worked_minutes),0)::int,
		       COALESCE(sum(overtime_minutes),0)::int,
		       COALESCE(sum(late_minutes),0)::int,
		       COALESCE(sum(early_leave_minutes),0)::int
		FROM attendance_entries
		WHERE organization_id=$1::uuid AND employee_id=$2 AND work_date BETWEEN $3::date AND $4::date`,
		orgID, employeeID, summary.From, summary.To).Scan(
		&summary.RecordedDays, &summary.WorkedMinutes, &summary.OvertimeMinutes,
		&summary.LateMinutes, &summary.EarlyLeaveMinutes,
	); err != nil {
		return model.AttendanceTimesheetSummary{}, err
	}

	holidays := map[string]struct{}{}
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(holiday_date,'YYYY-MM-DD') FROM company_holidays
		WHERE organization_id=$1::uuid AND holiday_date BETWEEN $2::date AND $3::date
		  AND (location IS NULL OR location='' OR lower(location)=lower($4))`, orgID, summary.From, summary.To, location)
	if err != nil {
		return model.AttendanceTimesheetSummary{}, err
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return model.AttendanceTimesheetSummary{}, err
		}
		holidays[value] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return model.AttendanceTimesheetSummary{}, err
	}

	for day := dateAtUTC(from); !day.After(dateAtUTC(to)); day = day.AddDate(0, 0, 1) {
		if _, holiday := holidays[day.Format("2006-01-02")]; holiday {
			continue
		}
		schedule, err := resolveWorkSchedule(ctx, s.pool, orgID, employeeID, day)
		if err != nil {
			return model.AttendanceTimesheetSummary{}, err
		}
		if workScheduleIncludesDay(schedule, day) {
			summary.ScheduledDays++
		}
	}
	summary.WorkedHours = math.Round((float64(summary.WorkedMinutes)/60)*100) / 100
	summary.OvertimeHours = math.Round((float64(summary.OvertimeMinutes)/60)*100) / 100
	return summary, nil
}

func resolveWorkSchedule(ctx context.Context, q attendanceQueryer, orgID, employeeID string, date time.Time) (model.WorkSchedule, error) {
	return scanWorkSchedule(q.QueryRow(ctx, `
		SELECT s.id::text, s.code, s.name, s.timezone, to_char(s.start_time,'HH24:MI'), to_char(s.end_time,'HH24:MI'),
		       s.break_minutes, s.grace_minutes, s.overtime_threshold_minutes, s.work_days,
		       s.is_default, s.active, s.created_at, s.updated_at
		FROM work_schedules s
		LEFT JOIN work_schedule_assignments a
		  ON a.organization_id=s.organization_id AND a.schedule_id=s.id AND a.employee_id=$2
		 AND a.effective_from <= $3::date AND (a.effective_to IS NULL OR a.effective_to >= $3::date)
		WHERE s.organization_id=$1::uuid AND s.active=true AND (a.id IS NOT NULL OR s.is_default=true)
		ORDER BY CASE WHEN a.id IS NULL THEN 1 ELSE 0 END, a.effective_from DESC NULLS LAST
		LIMIT 1`, orgID, employeeID, date.Format("2006-01-02")))
}

func localDateInZone(ctx context.Context, q attendanceQueryer, timezone string) (time.Time, error) {
	var value time.Time
	err := q.QueryRow(ctx, `SELECT (now() AT TIME ZONE $1)::date`, timezone).Scan(&value)
	return value, err
}

func workScheduleBounds(ctx context.Context, q attendanceQueryer, schedule model.WorkSchedule, date time.Time) (time.Time, time.Time, error) {
	var start, end time.Time
	err := q.QueryRow(ctx, `
		SELECT (($1::date + $2::time) AT TIME ZONE $4), (($1::date + $3::time) AT TIME ZONE $4)`,
		date.Format("2006-01-02"), schedule.StartTime, schedule.EndTime, schedule.Timezone).Scan(&start, &end)
	return start, end, err
}

func workScheduleIncludesDay(schedule model.WorkSchedule, date time.Time) bool {
	day := int(date.Weekday())
	for _, allowed := range schedule.WorkDays {
		if allowed == day {
			return true
		}
	}
	return false
}

func calculateAttendanceMetrics(checkIn, checkOut time.Time, scheduledStart, scheduledEnd *time.Time, breakMinutes, overtimeThreshold int) (worked, early, overtime int) {
	if checkOut.Before(checkIn) {
		return 0, 0, 0
	}
	worked = maxInt(0, minutesBetween(checkIn, checkOut)-breakMinutes)
	if scheduledStart == nil || scheduledEnd == nil {
		return worked, 0, worked
	}
	early = maxInt(0, minutesBetween(checkOut, *scheduledEnd))
	expected := maxInt(0, minutesBetween(*scheduledStart, *scheduledEnd)-breakMinutes)
	overtime = maxInt(0, worked-expected-overtimeThreshold)
	return worked, early, overtime
}

func scanWorkSchedule(row rowScanner) (model.WorkSchedule, error) {
	var schedule model.WorkSchedule
	var workDays []int16
	err := row.Scan(&schedule.ID, &schedule.Code, &schedule.Name, &schedule.Timezone,
		&schedule.StartTime, &schedule.EndTime, &schedule.BreakMinutes, &schedule.GraceMinutes,
		&schedule.OvertimeThresholdMinutes, &workDays, &schedule.IsDefault, &schedule.Active,
		&schedule.CreatedAt, &schedule.UpdatedAt)
	if err == nil {
		schedule.WorkDays = make([]int, 0, len(workDays))
		for _, day := range workDays {
			schedule.WorkDays = append(schedule.WorkDays, int(day))
		}
	}
	return schedule, err
}

func scanAttendanceEntryV2(row rowScanner) (model.AttendanceEntry, error) {
	var entry model.AttendanceEntry
	err := row.Scan(&entry.ID, &entry.EmployeeID, &entry.Employee, &entry.WorkDate,
		&entry.CheckIn, &entry.CheckOut, &entry.WorkMode, &entry.Status,
		&entry.ScheduleID, &entry.ScheduleName, &entry.ScheduledStartAt, &entry.ScheduledEndAt,
		&entry.BreakMinutes, &entry.LateMinutes, &entry.EarlyLeaveMinutes, &entry.WorkedMinutes,
		&entry.OvertimeMinutes, &entry.OvertimeThresholdMinutes, &entry.Source)
	return entry, err
}

func scanAttendanceCorrection(row rowScanner, request *model.AttendanceCorrectionRequest) error {
	return row.Scan(&request.ID, &request.EmployeeID, &request.Employee, &request.AttendanceEntryID,
		&request.WorkDate, &request.RequestedCheckIn, &request.RequestedCheckOut, &request.RequestedWorkMode,
		&request.Reason, &request.Status, &request.ReviewerUserID, &request.ReviewNote,
		&request.ReviewedAt, &request.CreatedAt)
}

func minutesBetween(from, to time.Time) int {
	return int(math.Floor(to.Sub(from).Minutes()))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sameCalendarDate(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func dateAtUTC(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
